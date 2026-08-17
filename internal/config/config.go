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

	sshConfigPath := filepath.Join(home, ".ssh", "config")
	file, err := os.Open(sshConfigPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	hostSet := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 && strings.EqualFold(parts[0], "host") {
			for _, h := range parts[1:] {
				if strings.Contains(h, "*") || strings.Contains(h, "?") {
					continue
				}
				if strings.Contains(strings.ToLower(h), "github") {
					continue
				}
				hostSet[h] = true
			}
		}
	}

	var hosts []string
	for h := range hostSet {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	return hosts
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
