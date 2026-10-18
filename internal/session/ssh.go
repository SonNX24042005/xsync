package session

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"xsync/internal/fsbrowse"
)

type Transport struct {
	Host, Auth, Password string
	socketDir            string
	configFile           string // Optional explicit OpenSSH config, used by isolated integration tests.
	checked              bool
	localChecked         bool
}

func NewTransport(host, auth, password string) (*Transport, error) {
	if err := checkLocalRsync(); err != nil {
		return nil, err
	}
	t, err := NewSSHTransport(host, auth, password)
	if err == nil {
		t.localChecked = true
	}
	return t, err
}

func checkLocalRsync() error {
	if _, err := exec.LookPath("rsync"); err != nil {
		return fmt.Errorf("missing dependency: rsync")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	version, err := exec.CommandContext(ctx, "rsync", "--version").Output()
	cancel()
	if err != nil || !supportedRsync(string(version)) {
		return fmt.Errorf("local rsync 3.1.3 or newer is required for precise change detection")
	}
	return nil
}

// NewSSHTransport permits authentication checks without requiring local rsync.
func NewSSHTransport(host, auth, password string) (*Transport, error) {
	if _, err := exec.LookPath("ssh"); err != nil {
		return nil, fmt.Errorf("missing dependency: ssh")
	}
	if auth == "password" {
		if password == "" {
			return nil, fmt.Errorf("a password is required for unattended password authentication")
		}
		if _, err := exec.LookPath("sshpass"); err != nil {
			return nil, fmt.Errorf("password authentication requires sshpass; SSH key authentication does not")
		}
	}
	t := &Transport{Host: host, Auth: auth, Password: password}
	// Native Windows OpenSSH does not support Unix control sockets.
	if runtime.GOOS != "windows" {
		dir, err := os.MkdirTemp("", "xsync-ssh-")
		if err != nil {
			return nil, err
		}
		t.socketDir = dir
	}
	return t, nil
}

func ShellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

func (t *Transport) args() []string {
	args := []string{"-T", "-o", "ConnectTimeout=10", "-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=2", "-o", "StrictHostKeyChecking=accept-new"}
	if t.configFile != "" {
		args = append(args, "-F", t.configFile)
	}
	if t.socketDir != "" {
		args = append(args, "-o", "ControlMaster=auto", "-o", "ControlPersist=60", "-o", "ControlPath="+filepath.Join(t.socketDir, "ctl"))
	}
	if t.Auth == "password" {
		args = append(args, "-o", "BatchMode=no", "-o", "NumberOfPasswordPrompts=1",
			"-o", "PubkeyAuthentication=no", "-o", "PreferredAuthentications=password,keyboard-interactive")
	} else {
		args = append(args, "-o", "BatchMode=yes", "-o", "PasswordAuthentication=no", "-o", "KbdInteractiveAuthentication=no", "-o", "PreferredAuthentications=publickey")
	}
	return args
}

func (t *Transport) command(ctx context.Context, args ...string) *exec.Cmd {
	var cmd *exec.Cmd
	if t.Auth == "password" {
		cmd = exec.CommandContext(ctx, "sshpass", append([]string{"-e", "ssh"}, args...)...)
		cmd.Env = append(os.Environ(), "SSHPASS="+t.Password)
	} else {
		cmd = exec.CommandContext(ctx, "ssh", args...)
	}
	cmd.WaitDelay = 2 * time.Second
	prepareCommand(cmd)
	return cmd
}

func (t *Transport) Run(ctx context.Context, script string) ([]byte, error) {
	return t.runWithTimeout(ctx, script, 20*time.Second)
}

func (t *Transport) runWithTimeout(ctx context.Context, script string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := t.command(ctx, append(t.args(), "--", t.Host, script)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if t.Password != "" {
			message = strings.ReplaceAll(message, t.Password, "[redacted]")
		}
		return nil, fmt.Errorf("SSH: %w: %s", err, message)
	}
	return out, nil
}

func (t *Transport) Check(ctx context.Context) error {
	if !t.localChecked {
		if err := checkLocalRsync(); err != nil {
			return err
		}
		t.localChecked = true
	}
	out, err := t.Run(ctx, "rsync --version")
	if err != nil {
		return err
	}
	if !supportedRsync(string(out)) {
		return fmt.Errorf("remote rsync 3.1.3 or newer is required for precise change detection")
	}
	t.checked = true
	return nil
}

var rsyncVersion = regexp.MustCompile(`\brsync\s+version\s+(\d+)\.(\d+)\.(\d+)`)

func supportedRsync(text string) bool {
	match := rsyncVersion.FindStringSubmatch(text)
	if len(match) != 4 {
		return false
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])
	return major > 3 || (major == 3 && (minor > 1 || (minor == 1 && patch >= 3)))
}

func (t *Transport) List(ctx context.Context, dir string) (fsbrowse.Listing, error) {
	cd := "cd"
	if dir != "" {
		if !path.IsAbs(dir) || unsafeText(dir) {
			return fsbrowse.Listing{}, fmt.Errorf("enter an absolute remote folder path")
		}
		cd = "cd " + ShellQuote(dir)
	}
	// NUL delimiters preserve spaces, quotes and terminal control characters in names.
	script := cd + ` || exit; printf '%s\000' "$(pwd -P)"; for entry in .[!.]* ..?* *; do [ -d "$entry" ] || continue; printf '%s\000' "$entry"; done`
	// Expand unmatched globs in POSIX sh instead of the remote login shell (e.g. zsh).
	out, err := t.Run(ctx, "sh -c "+ShellQuote(script))
	if err != nil {
		return fsbrowse.Listing{}, err
	}
	parts := bytes.Split(out, []byte{0})
	if len(parts) < 2 || !path.IsAbs(string(parts[0])) {
		return fsbrowse.Listing{}, fmt.Errorf("invalid remote directory listing; check SSH startup output")
	}
	listing := fsbrowse.Listing{Path: string(parts[0])}
	for _, name := range parts[1:] {
		if len(name) != 0 {
			listing.Entries = append(listing.Entries, fsbrowse.Entry{Name: string(name), Dir: true})
		}
	}
	fsbrowse.Sort(listing.Entries)
	return listing, nil
}

func (t *Transport) Mkdir(ctx context.Context, dir string) error {
	if !path.IsAbs(dir) || unsafeText(dir) {
		return fmt.Errorf("invalid remote folder path")
	}
	_, err := t.Run(ctx, "mkdir -p -- "+ShellQuote(dir))
	return err
}

func (t *Transport) Close() {
	if t.socketDir == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = t.command(ctx, append(t.args(), "-O", "exit", "--", t.Host)...).Run()
	_ = os.RemoveAll(t.socketDir)
}

func (t *Transport) RsyncShell() string {
	args := append([]string{"ssh"}, t.args()...)
	if t.Auth == "password" {
		args = append([]string{"sshpass", "-e"}, args...)
	}
	for i := range args {
		// rsync parses -e itself; single-quote arguments and double embedded quotes.
		args[i] = "'" + strings.ReplaceAll(args[i], "'", "''") + "'"
	}
	return strings.Join(args, " ")
}

// Sync mirrors only the configured selection inside B/A, never B or its siblings.
func (t *Transport) Sync(ctx context.Context, c Config, dry bool) (string, error) {
	if dry {
		ctx = context.WithValue(ctx, progressKey{}, nil)
	}
	reportProgress(ctx, Progress{Stage: "connecting"})
	if err := c.Validate(); err != nil {
		return "", err
	}
	if !t.checked {
		if err := t.Check(ctx); err != nil {
			return "", err
		}
	}
	info, err := os.Lstat(c.LocalDir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("source folder is unavailable; remote deletion is suspended: %s", c.LocalDir)
	}
	// Refuse remote symlinks or a conflicting file instead of deleting them.
	target := ShellQuote(c.Target())
	if _, err := t.Run(ctx, "test -d "+ShellQuote(c.RemoteParent)+" && test ! -L "+target+" && { test ! -e "+target+" || test -d "+target+"; }"); err != nil {
		return "", fmt.Errorf("remote parent is unavailable or target is a file/symlink: %w", err)
	}
	reportProgress(ctx, Progress{Stage: "scanning"})
	plan, err := prepareGitPlan(ctx, c, dry)
	if err != nil {
		return "", err
	}
	effective := c
	var prefix string
	if plan != nil {
		defer plan.release()
		if plan.repo != "" {
			effective = plan.effective
			// The previous selected root remains managed until a successful relocation finishes.
			effective.Selections = append(append([]Selection(nil), effective.Selections...), plan.cleanup...)
		}
		prefix, err = plan.moveRemote(ctx, t, dry)
		if err != nil {
			return prefix, err
		}
	}
	args := []string{"-az", "--modify-window=-1", "--protect-args", "--delete-delay", "--partial", "--timeout=60",
		"--itemize-changes", "--out-format=%i %n%L", "-e", t.RsyncShell()}
	if dry {
		args = append(args, "--dry-run")
	} else {
		args = append(args, "--info=progress2", "--outbuf=L", "--no-human-readable")
	}
	if rules := effective.Filters(); len(rules) > 0 {
		f, err := os.CreateTemp("", "xsync-filter-")
		if err != nil {
			return "", err
		}
		defer os.Remove(f.Name())
		_, err = f.WriteString(strings.Join(rules, "\n") + "\n")
		closeErr := f.Close()
		if err != nil {
			return "", err
		}
		if closeErr != nil {
			return "", closeErr
		}
		args = append(args, "--filter=merge "+rsyncLocalPath(f.Name()))
	}
	source := rsyncLocalPath(c.LocalDir) + "/"
	args = append(args, "--", source, c.Host+":"+c.Target()+"/")
	cmd := exec.CommandContext(ctx, "rsync", args...)
	cmd.WaitDelay = 2 * time.Second
	prepareCommand(cmd)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if t.Auth == "password" {
		cmd.Env = append(cmd.Env, "SSHPASS="+t.Password)
	}
	// Keep memory bounded for large initial uploads. Full rsync stdout is not retained.
	out := &tailBuffer{limit: 64 * 1024}
	stream := &progressOutput{ctx: ctx, log: out}
	cmd.Stdout, cmd.Stderr = stream, stream
	reportProgress(ctx, Progress{Stage: "transferring"})
	err = cmd.Run()
	stream.flush()
	text := out.String()
	if t.Password != "" {
		text = strings.ReplaceAll(text, t.Password, "[redacted]")
	}
	if err != nil {
		return prefix + text, fmt.Errorf("rsync: %w: %s", err, text)
	}
	if !dry {
		finalizing := stream.last
		finalizing.Stage, finalizing.Rate, finalizing.ETA = "finishing", "", ""
		reportProgress(ctx, finalizing)
		if err := plan.finish(ctx); err != nil {
			return prefix + text, err
		}
	}
	return prefix + text, nil
}

func rsyncLocalPath(name string) string {
	source := filepath.ToSlash(name)
	if runtime.GOOS == "windows" && len(source) > 2 && source[1] == ':' {
		// Scoop/Cygwin rsync uses POSIX drive paths.
		source = "/cygdrive/" + strings.ToLower(source[:1]) + source[2:]
	}
	return source
}

type tailBuffer struct {
	data  []byte
	limit int
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	n := len(p)
	b.data = append(b.data, p...)
	if len(b.data) > b.limit {
		b.data = b.data[len(b.data)-b.limit:]
	}
	return n, nil
}
func (b *tailBuffer) String() string { return string(b.data) }
