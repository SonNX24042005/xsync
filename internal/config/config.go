package config

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Profile struct {
	SSHPassword string
	RemoteDir   string
}

// FindConfigNearest walks down directory structure from PWD to find the file.
// Returns the directory containing the file with shallowest depth, or "" if not found.
func FindConfigNearest(filename string, maxDepth int) string {
	pwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	type match struct {
		depth int
		dir   string
	}
	var matches []match

	_ = filepath.WalkDir(pwd, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(pwd, path)
		if err != nil {
			return nil
		}
		if rel == "." {
			return nil
		}

		depth := strings.Count(rel, string(os.PathSeparator))
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !d.IsDir() && d.Name() == filename {
			matches = append(matches, match{depth: depth, dir: filepath.Dir(path)})
		}
		return nil
	})

	if len(matches) > 0 {
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].depth < matches[j].depth
		})
		return matches[0].dir
	}

	// Also check directly in PWD
	if _, err := os.Stat(filepath.Join(pwd, filename)); err == nil {
		return pwd
	}

	return ""
}

// ParseSSHHosts parses ~/.ssh/config and returns a list of configured Host names.
func ParseSSHHosts() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	hostSet := make(map[string]bool)
	seen := make(map[string]bool)
	var read func(string, int)
	read = func(name string, depth int) {
		name = filepath.Clean(name)
		if seen[name] || depth > 16 {
			return
		}
		seen[name] = true
		file, err := os.Open(name)
		if err != nil {
			return
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			parts := sshFields(scanner.Text())
			if len(parts) < 2 {
				continue
			}
			switch strings.ToLower(parts[0]) {
			case "host":
				for _, h := range parts[1:] {
					if strings.ContainsAny(h, "*?!") || strings.HasPrefix(h, "-") {
						continue
					}
					hostSet[h] = true
				}
			case "include":
				for _, pattern := range parts[1:] {
					if strings.HasPrefix(pattern, "~/") {
						pattern = filepath.Join(home, pattern[2:])
					}
					if !filepath.IsAbs(pattern) {
						pattern = filepath.Join(home, ".ssh", pattern)
					}
					matches, _ := filepath.Glob(pattern)
					for _, match := range matches {
						read(match, depth+1)
					}
				}
			}
		}
	}
	read(filepath.Join(home, ".ssh", "config"), 0)

	var hosts []string
	for h := range hostSet {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	return hosts
}

// sshFields handles quoted Include paths, key=value syntax and trailing comments.
func sshFields(line string) []string {
	var result []string
	var word strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if word.Len() > 0 {
			result = append(result, word.String())
			word.Reset()
		}
	}
	for _, r := range line {
		if escaped {
			word.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case '#':
			flush()
			return result
		case ' ', '\t', '=':
			flush()
		default:
			word.WriteRune(r)
		}
	}
	flush()
	return result
}

type IniData struct {
	Sections map[string]map[string]string
	Order    []string
}

func parseIniFile(iniPath string) (*IniData, error) {
	data := &IniData{
		Sections: make(map[string]map[string]string),
		Order:    []string{},
	}

	if _, err := os.Stat(iniPath); os.IsNotExist(err) {
		return data, nil
	}

	file, err := os.Open(iniPath)
	if err != nil {
		return data, err
	}
	defer file.Close()

	currentSection := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.TrimSpace(line[1 : len(line)-1])
			if _, exists := data.Sections[currentSection]; !exists {
				data.Sections[currentSection] = make(map[string]string)
				data.Order = append(data.Order, currentSection)
			}
			continue
		}

		if currentSection == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			data.Sections[currentSection][strings.ToLower(k)] = v
		}
	}
	return data, nil
}

func writeIniFile(iniPath string, data *IniData) error {
	var sb strings.Builder
	for _, sec := range data.Order {
		sb.WriteString(fmt.Sprintf("[%s]\n", sec))
		secMap := data.Sections[sec]
		// Sort keys for deterministic output
		var keys []string
		for k := range secMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("%s = %s\n", k, secMap[k]))
		}
		sb.WriteString("\n")
	}

	return os.WriteFile(iniPath, []byte(sb.String()), 0o644)
}

// LoadProfiles loads all profiles from the INI file.
func LoadProfiles(iniPath string) (map[string]Profile, error) {
	profiles := make(map[string]Profile)
	data, err := parseIniFile(iniPath)
	if err != nil {
		return profiles, err
	}

	for sec, vals := range data.Sections {
		if sec == "settings" {
			continue
		}
		profiles[sec] = Profile{
			SSHPassword: vals["ssh_password"],
			RemoteDir:   vals["remote_dir"],
		}
	}
	return profiles, nil
}

// SaveProfile updates or adds a profile in the INI file.
func SaveProfile(iniPath string, profileName string, p Profile) error {
	data, _ := parseIniFile(iniPath)

	if _, exists := data.Sections[profileName]; !exists {
		data.Sections[profileName] = make(map[string]string)
		data.Order = append(data.Order, profileName)
	}

	data.Sections[profileName]["ssh_password"] = p.SSHPassword
	data.Sections[profileName]["remote_dir"] = p.RemoteDir

	return writeIniFile(iniPath, data)
}

// GetDefaultProfile retrieves default_profile from [settings].
func GetDefaultProfile(iniPath string) string {
	data, err := parseIniFile(iniPath)
	if err != nil {
		return ""
	}
	if settings, exists := data.Sections["settings"]; exists {
		return settings["default_profile"]
	}
	return ""
}

// SetDefaultProfile writes default_profile under [settings].
func SetDefaultProfile(iniPath string, profileName string) error {
	data, _ := parseIniFile(iniPath)

	if _, exists := data.Sections["settings"]; !exists {
		data.Sections["settings"] = make(map[string]string)
		data.Order = append(data.Order, "settings")
	}

	data.Sections["settings"]["default_profile"] = profileName
	return writeIniFile(iniPath, data)
}

// GetLanguage retrieves language from [settings].
func GetLanguage(iniPath string) string {
	data, err := parseIniFile(iniPath)
	if err != nil {
		return ""
	}
	if settings, exists := data.Sections["settings"]; exists {
		return settings["language"]
	}
	return ""
}

// SetLanguage writes language under [settings].
func SetLanguage(iniPath string, lang string) error {
	data, _ := parseIniFile(iniPath)

	if _, exists := data.Sections["settings"]; !exists {
		data.Sections["settings"] = make(map[string]string)
		data.Order = append(data.Order, "settings")
	}

	data.Sections["settings"]["language"] = lang
	return writeIniFile(iniPath, data)
}

// EnsureConfigFiles checks and creates xsync.ini, xsync.push.ini, and xsync.pull.ini in configDir if they don't exist.
func EnsureConfigFiles(configDir string) []string {
	var created []string

	iniPath := filepath.Join(configDir, "xsync.ini")
	if _, err := os.Stat(iniPath); os.IsNotExist(err) {
		template := `# Cau hinh dong bo SSH cho xsync
# Khai bao cac Host tuong ung voi Host trong ~/.ssh/config

# [my-server]
# ssh_password = YOUR_SSH_PASSWORD
# remote_dir = /path/to/remote/directory

# [settings]
# default_profile = my-server
`
		if err := os.WriteFile(iniPath, []byte(template), 0o644); err == nil {
			created = append(created, "xsync.ini")
		}
	}

	pushPath := filepath.Join(configDir, "xsync.push.ini")
	if _, err := os.Stat(pushPath); os.IsNotExist(err) {
		template := `# Danh sach thu muc hoac file muon day len server (Push)
# Thu muc ket thuc bang dau / (vi du: data/datasets/)
# File khong co dau / o cuoi (vi du: scripts/train.py)

`
		if err := os.WriteFile(pushPath, []byte(template), 0o644); err == nil {
			created = append(created, "xsync.push.ini")
		}
	}

	pullPath := filepath.Join(configDir, "xsync.pull.ini")
	if _, err := os.Stat(pullPath); os.IsNotExist(err) {
		template := `# Danh sach thu muc hoac file muon tai ve tu server (Pull)
# Thu muc ket thuc bang dau / (vi du: outputs/runs/)
# File khong co dau / o cuoi (vi du: weights/best.pt)

`
		if err := os.WriteFile(pullPath, []byte(template), 0o644); err == nil {
			created = append(created, "xsync.pull.ini")
		}
	}

	return created
}
