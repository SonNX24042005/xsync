// Package sessioncli connects the session engine to the interactive CLI.
package sessioncli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode"

	"golang.org/x/term"
	"xsync/internal/config"
	"xsync/internal/fsbrowse"
	"xsync/internal/i18n"
	"xsync/internal/session"
	"xsync/internal/tui"
)

var tr = tui.Text

// Dispatch handles session subcommands before the legacy command performs any side effects.
func Dispatch(args []string) (bool, int) {
	var remaining []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--lang" && i+1 < len(args) {
			i++
			i18n.SetLanguage(args[i])
			continue
		}
		if strings.HasPrefix(args[i], "--lang=") {
			i18n.SetLanguage(strings.TrimPrefix(args[i], "--lang="))
			continue
		}
		remaining = append(remaining, args[i])
	}
	if len(remaining) == 0 || remaining[0] != "session" {
		return false, 0
	}
	if err := command(remaining[1:]); err != nil {
		if errors.Is(err, tui.ErrCancelled) {
			return true, 0
		}
		fmt.Fprintln(os.Stderr, err)
		return true, 1
	}
	return true, 0
}

func Help() string {
	return tr(`Phiên đồng bộ thư mục (local A → remote B/A)

  xsync session new           Chọn server, xác thực, thư mục và dữ liệu
  xsync session list          Xem các phiên đã lưu
  xsync session status <id>   Xem trạng thái và lỗi gần nhất
  xsync session start <id>    Chạy nền và bật tự khởi động
  xsync session stop <id>     Dừng dịch vụ và tắt tự khởi động
  xsync session run <id>      Chạy trong terminal, Ctrl+C để dừng
  xsync session preview <id>  Xem trước thay đổi, không truyền/xóa
  xsync session select <id>   Chọn lại dữ liệu khi phiên đã dừng
  xsync session logs <id>     Xem log gần nhất

Phiên phản chiếu cả thao tác xóa trong phần dữ liệu đã chọn.
Linux: tự chạy khi boot qua systemd user + linger.
macOS/Windows: tự chạy khi đăng nhập tài khoản.`, `Folder mirror sessions (local A → remote B/A)

  xsync session new           Choose server, authentication, folders and data
  xsync session list          List saved sessions
  xsync session status <id>   Show status and the latest error
  xsync session start <id>    Run in background and enable startup
  xsync session stop <id>     Stop the service and disable startup
  xsync session run <id>      Run in this terminal, Ctrl+C to stop
  xsync session preview <id>  Preview changes without transferring/deleting
  xsync session select <id>   Change selection while the session is stopped
  xsync session logs <id>     Show recent logs

Sessions also mirror deletions within the selected scope.
Linux: boot startup via systemd user + linger.
macOS/Windows: startup when the user logs in.`)
}

func command(args []string) error {
	if len(args) == 0 {
		return Menu()
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(Help())
		return nil
	}
	if args[0] == "new" && len(args) == 1 {
		return wizard()
	}
	if args[0] == "list" && len(args) == 1 {
		return list()
	}
	if len(args) != 2 {
		return fmt.Errorf("%s", Help())
	}
	c, err := session.Load(args[1])
	if err != nil {
		return err
	}
	switch args[0] {
	case "status":
		return status(c)
	case "start":
		if err = session.Start(c.ID); err != nil {
			return err
		}
		fmt.Println(tr("Đã bật phiên nền và tự khởi động.", "Background session and startup enabled."))
		return nil
	case "stop":
		if err = session.Stop(c.ID); err != nil {
			return err
		}
		fmt.Println(tr("Đã dừng và tắt tự khởi động.", "Stopped and startup disabled."))
		return nil
	case "run":
		if c.Auth == "password" && c.Password == "" {
			c.Password, err = readPassword()
			if err != nil {
				return err
			}
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		fmt.Printf("%s → %s:%s\n", c.LocalDir, c.Host, c.Target())
		return session.Run(ctx, c)
	case "preview":
		if c.Auth == "password" && c.Password == "" {
			c.Password, err = readPassword()
			if err != nil {
				return err
			}
		}
		t, err := session.NewTransport(c.Host, c.Auth, c.Password)
		if err != nil {
			return err
		}
		defer t.Close()
		var out string
		err = interruptible(func(ctx context.Context) error { var err error; out, err = t.Sync(ctx, c, true); return err })
		fmt.Print(safeOutput(out))
		return err
	case "select":
		if session.Running(c.ID) {
			return fmt.Errorf("%s", tr("Dừng phiên trước khi đổi dữ liệu được chọn.", "Stop the session before changing its selection."))
		}
		selections, err := selectData(c.LocalDir)
		if err != nil {
			return err
		}
		c.Selections = selections
		return session.Save(c)
	case "logs":
		dir, _ := session.Dir(c.ID)
		data, err := os.ReadFile(filepath.Join(dir, "session.log"))
		if err != nil {
			return err
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) > 60 {
			lines = lines[len(lines)-60:]
		}
		fmt.Println(safeOutput(strings.Join(lines, "\n")))
		return nil
	}
	return fmt.Errorf("%s", Help())
}

func Menu() error {
	for {
		sessions, err := session.List()
		if err != nil {
			return err
		}
		options := []string{tr("Tạo phiên đồng bộ thư mục", "Create a folder mirror session")}
		for _, c := range sessions {
			options = append(options, fmt.Sprintf("%s · %s · %s → %s", c.ID, c.Host, c.LocalDir, c.Target()))
		}
		options = append(options, tr("Quay lại / thoát", "Back / exit"))
		idx := tui.PromptMenu(tr("Phiên đồng bộ nền", "Background sync sessions"), options, 0)
		if idx == len(options)-1 {
			return nil
		}
		if idx == 0 {
			if err := wizard(); err != nil && !errors.Is(err, tui.ErrCancelled) {
				tui.PrintErr(err.Error())
			}
			continue
		}
		c := sessions[idx-1]
		actions := []string{tr("Xem trạng thái", "Show status"), tr("Chạy nền và tự khởi động", "Start in background and enable startup"), tr("Dừng và tắt tự khởi động", "Stop and disable startup"), tr("Xem log", "Show logs"), tr("Chọn lại dữ liệu", "Change selection"), tr("Quay lại", "Back")}
		a := tui.PromptMenu(c.ID, actions, 0)
		if a == 5 {
			continue
		}
		verbs := []string{"status", "start", "stop", "logs", "select"}
		if err := command([]string{verbs[a], c.ID}); err != nil {
			tui.PrintErr(err.Error())
		}
	}
}

func readPassword() (string, error) {
	fmt.Print(tr("Mật khẩu SSH (ẩn khi nhập): ", "SSH password (hidden): "))
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", err
	}
	if len(password) == 0 {
		return "", fmt.Errorf("%s", tr("Mật khẩu trống.", "Empty password."))
	}
	return string(password), nil
}

func selectData(root string) ([]session.Selection, error) {
	_, items, err := tui.Browse(context.Background(), tui.BrowserOptions{
		Title: tr("Chọn file/thư mục con cần phản chiếu; a để chọn tất cả", "Select files/subfolders to mirror; a selects everything"),
		Start: filepath.ToSlash(root), Root: filepath.ToSlash(root), List: fsbrowse.Local, Multi: true,
	})
	if err != nil {
		return nil, err
	}
	var result []session.Selection
	for _, s := range items {
		result = append(result, session.Selection{Path: s.Path, Dir: s.Dir})
	}
	return result, nil
}

func wizard() error {
	hosts := config.ParseSSHHosts()
	if len(hosts) == 0 {
		return fmt.Errorf("%s", tr("Thêm Host vào ~/.ssh/config trước khi tạo phiên.", "Add a Host to ~/.ssh/config before creating a session."))
	}
	host := hosts[tui.PromptMenu(tr("Chọn server trong SSH config", "Choose a server from SSH config"), hosts, 0)]
	c := session.Config{ID: session.NewID(), Host: host, CreatedAt: time.Now()}
	var t *session.Transport
	savePassword := true
	for {
		a := tui.PromptMenu(tr("Cách xác thực SSH", "SSH authentication"), []string{tr("SSH key / SSH agent (không cần mật khẩu)", "SSH key / SSH agent (no password)"), tr("Mật khẩu", "Password"), tr("Hủy", "Cancel")}, 0)
		if a == 2 {
			return tui.ErrCancelled
		}
		c.Auth, c.Password = "key", ""
		if a == 1 {
			c.Auth = "password"
			var err error
			c.Password, err = readPassword()
			if err != nil {
				return err
			}
		}
		var err error
		t, err = session.NewTransport(c.Host, c.Auth, c.Password)
		if err == nil {
			tui.PrintInfo(tr("Đang kiểm tra kết nối SSH…", "Checking SSH connection…"))
			err = interruptible(t.Check)
		}
		if err == nil {
			break
		}
		if t != nil {
			t.Close()
		}
		if errors.Is(err, tui.ErrCancelled) {
			return err
		}
		tui.PrintErr(err.Error())
	}
	defer t.Close()
	if c.Auth == "password" {
		choice, err := tui.ChooseMenu(tr("Lưu mật khẩu", "Save password"), []string{
			tr("Lưu trong cấu hình riêng của tài khoản để chạy nền và tự khởi động", "Save in the account's private config for background/startup use"),
			tr("Không lưu; chỉ chạy phiên trong terminal này", "Do not save; run only in this terminal"),
		}, 0)
		if err != nil {
			return err
		}
		savePassword = choice == 0
	}
	local, _, err := tui.Browse(context.Background(), tui.BrowserOptions{Title: tr("Chọn thư mục nguồn A; Space để dùng thư mục đang mở", "Choose source folder A; Space uses the current folder"), List: fsbrowse.Local})
	if err != nil {
		return err
	}
	c.LocalDir = filepath.FromSlash(local)
	parent, _, err := tui.Browse(context.Background(), tui.BrowserOptions{Title: tr("Chọn thư mục chứa B trên server; dữ liệu sẽ nằm trong B/A", "Choose remote parent B; data will be placed inside B/A"), List: t.List, Mkdir: t.Mkdir})
	if err != nil {
		return err
	}
	c.RemoteParent = parent
	c.Selections, err = selectData(c.LocalDir)
	if err != nil {
		return err
	}
	if err = c.Validate(); err != nil {
		return err
	}
	fmt.Printf("\n%s\n%s → %s:%s\n", tr("Phiên phản chiếu, bao gồm thao tác xóa trong phạm vi đã chọn:", "Mirror session, including deletions inside the selected scope:"), c.LocalDir, c.Host, c.Target())
	for _, s := range c.Selections {
		fmt.Printf("  • %s\n", s.Path)
	}
	tui.PrintInfo(tr("Đang xem trước thay đổi…", "Previewing changes…"))
	var out string
	err = interruptible(func(ctx context.Context) error { var err error; out, err = t.Sync(ctx, c, true); return err })
	if err != nil {
		return err
	}
	fmt.Print(safeOutput(out))
	startLabel := tr("Lưu, chạy nền và bật tự khởi động", "Save, start in background and enable startup")
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		startLabel = tr("Lưu, chạy nền và tự chạy khi đăng nhập", "Save, start in background and run at login")
	}
	if !savePassword {
		startLabel = tr("Lưu cấu hình không có mật khẩu và chạy trong terminal", "Save config without the password and run in this terminal")
	}
	idx, err := tui.ChooseMenu(tr("Xác nhận phiên đồng bộ", "Confirm sync session"), []string{startLabel, tr("Chỉ lưu cấu hình", "Save config only"), tr("Hủy", "Cancel")}, 0)
	if err != nil {
		return err
	}
	if idx == 2 {
		return tui.ErrCancelled
	}
	saved := c
	if !savePassword {
		saved.Password = ""
	}
	if err = session.Save(saved); err != nil {
		return err
	}
	fmt.Printf("%s: %s\n", tr("Đã lưu phiên", "Saved session"), c.ID)
	if idx == 1 {
		return nil
	}
	if !savePassword {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		return session.Run(ctx, c)
	}
	if err = session.Start(c.ID); err != nil {
		return fmt.Errorf("%s %s: %w", tr("Cấu hình đã được lưu; chưa bật được dịch vụ. Thử lại bằng: xsync session start", "Config saved; service could not be enabled. Retry: xsync session start"), c.ID, err)
	}
	fmt.Println(tr("Phiên đang chạy nền. Dùng session list/status/logs để theo dõi.", "Session is running in the background. Use session list/status/logs to monitor it."))
	return nil
}

func list() error {
	all, err := session.List()
	if err != nil {
		return err
	}
	if len(all) == 0 {
		fmt.Println(tr("Chưa có phiên. Chạy: xsync session new", "No sessions yet. Run: xsync session new"))
	}
	for _, c := range all {
		phase := tr("đã dừng", "stopped")
		if session.Running(c.ID) {
			phase = session.ReadStatus(c.ID).Phase
		}
		fmt.Printf("%s [%s] %s → %s:%s\n", c.ID, phase, c.LocalDir, c.Host, c.Target())
	}
	return nil
}

func status(c session.Config) error {
	s := session.ReadStatus(c.ID)
	phase := tr("đã dừng", "stopped")
	if session.Running(c.ID) {
		phase = s.Phase
		if phase == "" {
			phase = "starting"
		}
	}
	fmt.Printf("%s [%s]\n%s → %s:%s\n", c.ID, phase, c.LocalDir, c.Host, c.Target())
	fmt.Printf("%s: %s\n", tr("Xác thực", "Authentication"), c.Auth)
	for _, item := range c.Selections {
		fmt.Printf("  • %s\n", item.Path)
	}
	if !s.LastSuccess.IsZero() {
		fmt.Printf("%s: %s\n", tr("Đồng bộ thành công gần nhất", "Last successful sync"), s.LastSuccess.Local().Format(time.RFC3339))
	}
	if s.LastError != "" {
		fmt.Printf("%s: %s\n", tr("Lỗi gần nhất", "Latest error"), safeOutput(s.LastError))
	}
	return nil
}

func safeOutput(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return '�'
		}
		return r
	}, s)
}

func interruptible(fn func(context.Context) error) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	err := fn(ctx)
	if ctx.Err() != nil {
		return tui.ErrCancelled
	}
	return err
}
