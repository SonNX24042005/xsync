package session

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// Only hashes and paths enter the private Git object database, never user file contents.
type gitEntry struct {
	Hash       string `json:"hash,omitempty"`
	Kind       string `json:"kind"`
	Executable bool   `json:"executable,omitempty"`
	Tree       string `json:"tree,omitempty"`
	Stat       string `json:"stat,omitempty"`
	Metadata   string `json:"metadata,omitempty"`
}
type gitSnapshot struct {
	Identity     string              `json:"identity"`
	Tree         string              `json:"tree"`
	Entries      map[string]gitEntry `json:"entries"`
	KnownPaths   map[string]bool     `json:"known_paths"`
	RootMetadata string              `json:"root_metadata,omitempty"`
}
type gitMove struct{ From, To, Hash string }
type gitPlan struct {
	repo      string
	stateFile string
	before    *gitSnapshot
	after     gitSnapshot
	original  Config
	effective Config
	cleanup   []Selection
	moves     []gitMove
	warning   string
	release   func()
}

func selectionIdentity(c Config) string {
	mode := c.Mode()
	if mode == SelectionInclude {
		mode = ""
	}
	data, _ := json.Marshal(struct {
		Host, Local, Remote string
		Selected            []Selection
		Mode                string `json:",omitempty"`
	}{c.Host, c.LocalDir, c.RemoteParent, c.Selections, mode})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func gitCommand(ctx context.Context, repo string, input []byte, args ...string) ([]byte, error) {
	argv := []string{"--git-dir=" + repo, "-c", "core.hooksPath=", "-c", "core.autocrlf=false", "-c", "gc.auto=0"}
	cmd := exec.CommandContext(ctx, "git", append(argv, args...)...)
	// Ignore repository, index, object-store and config overrides inherited from the project.
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "GIT_") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_NO_REPLACE_OBJECTS=1", "GIT_TERMINAL_PROMPT=0")
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Git change tracking: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func initChangesRepo(ctx context.Context, repo string) error {
	if err := os.MkdirAll(repo, 0o700); err != nil {
		return err
	}
	if info, err := os.Lstat(repo); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("invalid private Git directory")
	}
	if err := os.Chmod(repo, 0o700); err != nil {
		return err
	}
	_, err := gitCommand(ctx, repo, nil, "init", "--bare", "--quiet", "--template=", repo)
	return err
}

func scanGitEntries(ctx context.Context, c Config, all bool, previous map[string]gitEntry) (map[string]gitEntry, error) {
	entries := make(map[string]gitEntry)
	err := filepath.WalkDir(c.LocalDir, func(name string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(c.LocalDir, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		// Keep xsync's credentials and private state outside the source snapshot.
		if c.storageRelative() != "" && (rel == c.storageRelative() || strings.HasPrefix(rel, c.storageRelative()+"/")) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if unsafeText(rel) {
			return fmt.Errorf("path cannot be tracked safely: %q", rel)
		}
		if !all && !c.Relevant(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			entries[rel] = gitEntry{Kind: "directory", Metadata: gitMetadata(info)}
			return nil
		}
		// Ancestor paths may be relevant for traversal, but only selected files enter the snapshot.
		if !all && !selectedPath(c, rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		entry := gitEntry{Kind: "file", Executable: info.Mode()&0o111 != 0, Metadata: gitMetadata(info)}
		// Include filesystem identity/change time where available, like Git's stat cache.
		entry.Stat = fmt.Sprintf("%d:%d:%d:%v", info.Size(), info.ModTime().UnixNano(), info.Mode(), info.Sys())
		if old, ok := previous[rel]; ok && old.Stat == entry.Stat && old.Kind == "file" && info.Mode().IsRegular() {
			old.Metadata = entry.Metadata
			entries[rel] = old
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			value, err := os.Readlink(name)
			if err != nil {
				return err
			}
			sum := sha256.Sum256([]byte(value))
			entry.Kind = "symlink"
			entry.Hash = hex.EncodeToString(sum[:])
		} else if info.Mode().IsRegular() {
			file, err := os.Open(name)
			if err != nil {
				return err
			}
			opened, err := file.Stat()
			if err != nil {
				file.Close()
				return err
			}
			if !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
				file.Close()
				return fmt.Errorf("source changed while scanning: %s", rel)
			}
			hash := sha256.New()
			_, copyErr := io.Copy(hash, gitContextReader{ctx: ctx, reader: file})
			after, statErr := file.Stat()
			file.Close()
			if copyErr != nil {
				return copyErr
			}
			if statErr != nil {
				return statErr
			}
			if opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
				return fmt.Errorf("source changed while scanning: %s", rel)
			}
			entry.Hash = hex.EncodeToString(hash.Sum(nil))
		} else {
			// Fingerprint special entries without opening pipes or devices.
			entry.Kind = "special"
			sum := sha256.Sum256([]byte(entry.Metadata))
			entry.Hash = hex.EncodeToString(sum[:])
		}
		entries[rel] = entry
		return nil
	})
	return entries, err
}

// Permission and owner changes still need rsync even if Git's content tree is equal.
// Timestamps are intentionally omitted: touching a file or adding an excluded
// child must not start another transfer when the managed content is unchanged.
func gitMetadata(info fs.FileInfo) string {
	value := fmt.Sprintf("%d", info.Mode())
	system := reflect.ValueOf(info.Sys())
	if system.Kind() == reflect.Pointer && !system.IsNil() {
		system = system.Elem()
	}
	if system.IsValid() && system.Kind() == reflect.Struct {
		for _, name := range []string{"Uid", "Gid", "Rdev"} {
			field := system.FieldByName(name)
			if field.IsValid() && field.CanInterface() {
				value += fmt.Sprintf(":%v", field.Interface())
			}
		}
	}
	return value
}

func selectedPath(c Config, rel string) bool {
	if c.Mode() == SelectionExclude {
		return c.Relevant(rel)
	}
	for _, s := range c.Selections {
		if s.Path == "." || rel == s.Path || (s.Dir && strings.HasPrefix(rel, s.Path+"/")) {
			return true
		}
	}
	return false
}

func projectGitPath(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if part == ".git" {
			return true
		}
	}
	return false
}

// Git tree objects include empty directories; blobs contain only fixed-size SHA-256 hashes.
func writeGitTree(ctx context.Context, repo string, entries map[string]gitEntry) (string, error) {
	children := make(map[string][]string)
	for rel := range entries {
		// Git trees reject .git entries. Their fingerprints remain in the private
		// snapshot so a selected project's own Git data still triggers mirroring.
		if projectGitPath(rel) {
			continue
		}
		children[path.Dir(rel)] = append(children[path.Dir(rel)], rel)
	}
	var tree func(string) (string, error)
	tree = func(parent string) (string, error) {
		names := children[parent]
		sort.Slice(names, func(i, j int) bool {
			a, b := path.Base(names[i]), path.Base(names[j])
			if entries[names[i]].Kind == "directory" {
				a += "/"
			}
			if entries[names[j]].Kind == "directory" {
				b += "/"
			}
			return a < b
		})
		var object bytes.Buffer
		for _, rel := range names {
			entry := entries[rel]
			mode := "100644"
			var id string
			if entry.Kind == "directory" {
				var err error
				id, err = tree(rel)
				if err != nil {
					return "", err
				}
				entry.Tree = id
				entries[rel] = entry
				mode = "40000"
			} else {
				if entry.Executable {
					mode = "100755"
				}
				if entry.Kind == "symlink" {
					mode = "120000"
				}
				var err error
				id, err = gitObject(ctx, repo, "blob", []byte(entry.Hash))
				if err != nil {
					return "", err
				}
			}
			raw, err := hex.DecodeString(id)
			if err != nil || len(raw) != 20 {
				return "", fmt.Errorf("invalid Git object id")
			}
			fmt.Fprintf(&object, "%s %s%c", mode, path.Base(rel), 0)
			object.Write(raw)
		}
		return gitObject(ctx, repo, "tree", object.Bytes())
	}
	return tree(".")
}

func prepareGitPlan(ctx context.Context, c Config, dry bool) (*gitPlan, error) {
	// Unsaved previews and callers without a session directory remain read-only rsync operations.
	saved, err := Load(c.ID)
	if err != nil || selectionIdentity(saved) != selectionIdentity(c) {
		return nil, nil
	}
	if _, err := exec.LookPath("git"); err != nil {
		return &gitPlan{warning: "Git unavailable; using rsync for change checks and transfers without rename optimization.\n", release: func() {}}, nil
	}
	dir, _ := Dir(c.ID)
	p := &gitPlan{repo: filepath.Join(dir, "changes.git"), stateFile: filepath.Join(dir, "changes.json"), original: c, effective: c, release: func() {}}
	if !dry {
		lock, err := os.OpenFile(filepath.Join(dir, "changes.lock"), os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return nil, err
		}
		if err = tryLock(lock); err != nil {
			lock.Close()
			return nil, fmt.Errorf("change tracking is busy: %w", err)
		}
		p.release = func() { unlock(lock); lock.Close() }
	}
	data, err := os.ReadFile(p.stateFile)
	if err == nil {
		var baseline gitSnapshot
		if err = json.Unmarshal(data, &baseline); err != nil {
			p.release()
			return nil, fmt.Errorf("invalid Git tracking state: %w", err)
		}
		if baseline.Identity == selectionIdentity(c) {
			p.before = &baseline
		}
	} else if !os.IsNotExist(err) {
		p.release()
		return nil, err
	}
	if dry {
		if p.before == nil {
			return nil, nil
		}
		temp, err := os.MkdirTemp("", "xsync-git-preview-")
		if err != nil {
			return nil, err
		}
		p.repo = filepath.Join(temp, "changes.git")
		p.release = func() { _ = os.RemoveAll(temp) }
	}
	fail := func(err error) (*gitPlan, error) { p.release(); return nil, err }
	if err = initChangesRepo(ctx, p.repo); err != nil {
		return fail(err)
	}
	missing := false
	if c.Mode() == SelectionInclude {
		for _, s := range c.Selections {
			if _, err := os.Lstat(filepath.Join(c.LocalDir, filepath.FromSlash(s.Path))); os.IsNotExist(err) {
				missing = true
			}
		}
	}
	var previous map[string]gitEntry
	if p.before != nil {
		previous = p.before.Entries
	}
	entries, err := scanGitEntries(ctx, c, missing && p.before != nil, previous)
	if err != nil {
		return fail(err)
	}
	if _, err = writeGitTree(ctx, p.repo, entries); err != nil {
		return fail(err)
	}
	if missing && p.before != nil {
		effective := append([]Selection(nil), c.Selections...)
		for i, s := range c.Selections {
			if _, exists := entries[s.Path]; exists || s.Path == "." {
				continue
			}
			old, exists := p.before.Entries[s.Path]
			if !exists {
				continue
			}
			var candidates []string
			// Follow a selected root only when its complete content/tree has one exact match.
			oldCount := 0
			for _, entry := range p.before.Entries {
				if equivalentGitEntry(old, entry) {
					oldCount++
				}
			}
			for rel, entry := range entries {
				if projectGitPath(rel) {
					continue
				}
				if equivalentGitEntry(old, entry) && !p.before.KnownPaths[rel] {
					candidates = append(candidates, rel)
				}
			}
			if len(candidates) == 1 && oldCount == 1 {
				effective[i].Path = candidates[0]
				p.cleanup = append(p.cleanup, s)
			} else if len(candidates) > 0 {
				return fail(fmt.Errorf("selected path %q disappeared and its new location is ambiguous; stop the session and choose its data again", s.Path))
			} else {
				for rel, entry := range entries {
					if entry.Kind == old.Kind && !p.before.KnownPaths[rel] {
						return fail(fmt.Errorf("selected path %q disappeared while new paths appeared; cannot safely distinguish a move with edits from deletion; stop the session and choose its data again", s.Path))
					}
				}
			}
		}
		p.effective.Selections = effective
		if err := p.effective.Validate(); err != nil {
			return fail(err)
		}
		for rel := range entries {
			if !p.effective.Relevant(rel) {
				delete(entries, rel)
			}
		}
	}
	known, err := knownGitPaths(ctx, c)
	if err != nil {
		return fail(err)
	}
	rootInfo, err := os.Lstat(c.LocalDir)
	if err != nil {
		return fail(err)
	}
	p.after = gitSnapshot{Identity: selectionIdentity(p.effective), Entries: entries, KnownPaths: known, RootMetadata: gitMetadata(rootInfo)}
	p.after.Tree, err = writeGitTree(ctx, p.repo, entries)
	if err != nil {
		return fail(err)
	}
	if p.before != nil {
		// Rebuild from checksums if the private object cache was cleared; previews use a temporary repo.
		oldTree, err := writeGitTree(ctx, p.repo, p.before.Entries)
		if err != nil {
			return fail(err)
		}
		if oldTree != p.before.Tree {
			return fail(fmt.Errorf("Git tracking baseline tree does not match its metadata"))
		}
		out, err := gitCommand(ctx, p.repo, nil, "diff-tree", "--no-commit-id", "--name-status", "-r", "-z", "-M100%", p.before.Tree, p.after.Tree)
		if err != nil {
			return fail(err)
		}
		tokens := bytes.Split(out, []byte{0})
		for i := 0; i < len(tokens) && len(tokens[i]) > 0; {
			status := string(tokens[i])
			i++
			if strings.HasPrefix(status, "R") && i+1 < len(tokens) {
				from, to := string(tokens[i]), string(tokens[i+1])
				i += 2
				old, oldOK := p.before.Entries[from]
				next, newOK := entries[to]
				if oldOK && newOK && old.Kind == "file" && next.Kind == "file" && old.Hash == next.Hash && selectedPath(c, from) && selectedPath(p.effective, to) {
					p.moves = append(p.moves, gitMove{from, to, old.Hash})
				}
			} else {
				i++
			}
		}
	}
	return p, nil
}

func (p *gitPlan) transferPendingPath() string {
	return filepath.Join(filepath.Dir(p.stateFile), "sync-pending.json")
}

func (p *gitPlan) unchanged() (bool, error) {
	if p == nil || p.repo == "" || p.before == nil || p.before.Identity != p.after.Identity ||
		p.before.Tree != p.after.Tree || p.before.RootMetadata != p.after.RootMetadata || len(p.before.Entries) != len(p.after.Entries) {
		return false, nil
	}
	for rel, next := range p.after.Entries {
		if old, ok := p.before.Entries[rel]; !ok || old.Metadata != next.Metadata || old.Hash != next.Hash || old.Kind != next.Kind || old.Executable != next.Executable {
			return false, nil
		}
	}
	for _, name := range []string{p.pendingPath(), p.transferPendingPath()} {
		if _, err := os.Lstat(name); err == nil {
			return false, nil
		} else if !os.IsNotExist(err) {
			return false, err
		}
	}
	return true, nil
}

func (p *gitPlan) beginTransfer() error {
	if p == nil || p.repo == "" {
		return nil
	}
	// Keep a durable retry marker before any remote mutation. Local changes may
	// revert to the previous baseline after a failed or interrupted transfer.
	return writeJSON(p.transferPendingPath(), map[string]string{"identity": p.after.Identity})
}

func (p *gitPlan) completeTransfer() error {
	if p == nil || p.repo == "" {
		return nil
	}
	if err := os.Remove(p.transferPendingPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (p *gitPlan) needsChecksum() bool {
	if p == nil || p.repo == "" {
		return false
	}
	if _, err := os.Lstat(p.transferPendingPath()); err == nil {
		return true
	}
	if p.before == nil {
		return false
	}
	for rel, next := range p.after.Entries {
		old, ok := p.before.Entries[rel]
		if ok && old.Kind == "file" && next.Kind == "file" && old.Hash != next.Hash {
			previous, current := strings.SplitN(old.Stat, ":", 3), strings.SplitN(next.Stat, ":", 3)
			if len(previous) < 3 || len(current) < 3 || (previous[0] == current[0] && previous[1] == current[1]) {
				return true
			}
		}
	}
	return false
}

func equivalentGitEntry(a, b gitEntry) bool {
	if a.Kind != b.Kind || a.Executable != b.Executable {
		return false
	}
	if a.Kind == "directory" {
		return a.Tree != "" && a.Tree == b.Tree
	}
	return a.Hash != "" && a.Hash == b.Hash
}

func (p *gitPlan) finish(ctx context.Context) error {
	if p == nil || p.repo == "" {
		return nil
	}
	if !reflect.DeepEqual(p.original.Selections, p.effective.Selections) {
		saved, err := Load(p.original.ID)
		if err != nil {
			return err
		}
		if selectionIdentity(saved) != selectionIdentity(p.original) {
			return fmt.Errorf("selection changed during synchronization; retry")
		}
		saved.Selections = p.effective.Selections
		if err = Save(saved); err != nil {
			return err
		}
	}
	if _, err := gitCommand(ctx, p.repo, nil, "update-ref", "refs/xsync/synced", p.after.Tree); err != nil {
		return err
	}
	if err := writeJSON(p.stateFile, p.after); err != nil {
		return err
	}
	if err := os.Remove(p.pendingPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func gitObject(ctx context.Context, repo, kind string, payload []byte) (string, error) {
	hash := sha1.New()
	fmt.Fprintf(hash, "%s %d%c", kind, len(payload), 0)
	hash.Write(payload)
	id := hex.EncodeToString(hash.Sum(nil))
	if _, err := os.Stat(filepath.Join(repo, "objects", id[:2], id[2:])); err == nil {
		return id, nil
	}
	out, err := gitCommand(ctx, repo, payload, "hash-object", "-w", "-t", kind, "--stdin")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(out)) != id {
		return "", fmt.Errorf("Git object id mismatch")
	}
	return id, nil
}

func knownGitPaths(ctx context.Context, c Config) (map[string]bool, error) {
	known := make(map[string]bool)
	err := filepath.WalkDir(c.LocalDir, func(name string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(c.LocalDir, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if c.Mode() == SelectionExclude && !c.Relevant(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if c.storageRelative() != "" && (rel == c.storageRelative() || strings.HasPrefix(rel, c.storageRelative()+"/")) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		known[rel] = true
		return nil
	})
	return known, err
}

type gitContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r gitContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
