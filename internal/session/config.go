// Package session implements persistent local-to-remote mirror sessions.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Selection struct {
	Path string `json:"path"`
	Dir  bool   `json:"directory"`
}

type Config struct {
	ID           string      `json:"id"`
	Host         string      `json:"host"`
	Auth         string      `json:"auth"`
	Password     string      `json:"password,omitempty"`
	LocalDir     string      `json:"local_dir"`
	RemoteParent string      `json:"remote_parent"`
	Selections   []Selection `json:"selections"`
	CreatedAt    time.Time   `json:"created_at"`
}

var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("sync-%d", time.Now().UnixNano())
	}
	return "sync-" + hex.EncodeToString(b[:])
}

func Root() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "xsync", "sessions"), nil
}

func Dir(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", fmt.Errorf("invalid session id: %q", id)
	}
	root, err := Root()
	return filepath.Join(root, id), err
}

func (c Config) Target() string {
	return path.Join(c.RemoteParent, filepath.Base(c.LocalDir))
}

func unsafeText(s string) bool { return strings.ContainsAny(s, "\x00\r\n") }

func (c Config) Validate() error {
	if !validID.MatchString(c.ID) {
		return fmt.Errorf("invalid session id")
	}
	if c.Host == "" || strings.HasPrefix(c.Host, "-") || strings.ContainsAny(c.Host, " \t\r\n\x00:*?!") {
		return fmt.Errorf("invalid SSH host alias")
	}
	if c.Auth != "key" && c.Auth != "password" {
		return fmt.Errorf("authentication must be key or password")
	}
	if !filepath.IsAbs(c.LocalDir) || filepath.Clean(c.LocalDir) != c.LocalDir || unsafeText(c.LocalDir) || filepath.Dir(c.LocalDir) == c.LocalDir {
		return fmt.Errorf("local source must be an absolute folder below the filesystem root")
	}
	if root, err := Root(); err == nil && (c.LocalDir == filepath.Dir(root) || strings.HasPrefix(c.LocalDir, filepath.Dir(root)+string(os.PathSeparator))) {
		return fmt.Errorf("private xsync configuration cannot be used as a sync source")
	}
	if !path.IsAbs(c.RemoteParent) || path.Clean(c.RemoteParent) != c.RemoteParent || unsafeText(c.RemoteParent) {
		return fmt.Errorf("remote parent must be a clean absolute Unix path")
	}
	if len(c.Selections) == 0 {
		return fmt.Errorf("select at least one file or folder")
	}
	for _, s := range c.Selections {
		if s.Path == "." && s.Dir {
			continue
		}
		if s.Path == "" || s.Path == "." || path.IsAbs(s.Path) || path.Clean(s.Path) != s.Path ||
			s.Path == ".." || strings.HasPrefix(s.Path, "../") || strings.Contains(s.Path, "\\") || unsafeText(s.Path) {
			return fmt.Errorf("invalid selected relative path: %q", s.Path)
		}
	}
	return nil
}

func Save(c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	all, err := List()
	if err != nil {
		return err
	}
	for _, other := range all {
		if other.ID != c.ID && other.Host == c.Host && (other.Target() == c.Target() ||
			strings.HasPrefix(other.Target(), c.Target()+"/") || strings.HasPrefix(c.Target(), other.Target()+"/")) {
			return fmt.Errorf("destination is already managed by session %s", other.ID)
		}
	}
	dir, err := Dir(c.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	return writeJSON(filepath.Join(dir, "session.json"), c)
}

func Load(id string) (Config, error) {
	var c Config
	dir, err := Dir(id)
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "session.json"))
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if c.ID != id {
		return c, fmt.Errorf("session id does not match its directory")
	}
	return c, c.Validate()
}

func List() ([]Config, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []Config
	for _, e := range entries {
		if !e.IsDir() || !validID.MatchString(e.Name()) {
			continue
		}
		c, err := Load(e.Name())
		if err != nil {
			return nil, fmt.Errorf("session %s: %w", e.Name(), err)
		}
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func writeJSON(name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(name), ".write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), name)
}

// Relevant includes both selected descendants and the ancestors needed to reach them.
func (c Config) Relevant(rel string) bool {
	rel = filepath.ToSlash(rel)
	if internal := c.storageRelative(); internal != "" && (rel == internal || strings.HasPrefix(rel, internal+"/")) {
		return false
	}
	for _, s := range c.Selections {
		if rel == "." || s.Path == "." || rel == s.Path || strings.HasPrefix(s.Path, rel+"/") ||
			(s.Dir && strings.HasPrefix(rel, s.Path+"/")) {
			return true
		}
	}
	return false
}

func (c Config) storageRelative() string {
	root, err := Root()
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(c.LocalDir, filepath.Dir(root))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return ""
	}
	return filepath.ToSlash(rel)
}

func escapeFilter(s string) string {
	return strings.NewReplacer("\\", "\\\\", "*", "\\*", "?", "\\?", "[", "\\[").Replace(s)
}

// Filters are anchored and literal. Unselected remote files remain protected from deletion.
func (c Config) Filters() []string {
	var rules []string
	if internal := c.storageRelative(); internal != "" {
		rules = append(rules, "- /"+escapeFilter(internal)+"/***")
	}
	seen := make(map[string]bool)
	add := func(rule string) {
		if !seen[rule] {
			rules = append(rules, rule)
			seen[rule] = true
		}
	}
	for _, s := range c.Selections {
		if s.Path == "." {
			return rules
		}
		parts := strings.Split(s.Path, "/")
		for i := 1; i < len(parts); i++ {
			add("+ /" + escapeFilter(strings.Join(parts[:i], "/")) + "/")
		}
		rule := "+ /" + escapeFilter(s.Path)
		if s.Dir {
			rule += "/***"
		}
		add(rule)
	}
	return append(rules, "- *")
}
