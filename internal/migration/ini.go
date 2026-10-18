package migration

import (
	"bufio"
	"os"
	"strings"
)

type legacyProfile struct{ SSHPassword string }

func legacyINI(name string) (map[string]map[string]string, error) {
	result := map[string]map[string]string{}
	f, err := os.Open(name)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	section := ""
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if result[section] == nil {
				result[section] = map[string]string{}
			}
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if section != "" && len(parts) == 2 {
			result[section][strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
		}
	}
	return result, scanner.Err()
}
func legacyProfiles(name string) (map[string]legacyProfile, error) {
	data, err := legacyINI(name)
	result := map[string]legacyProfile{}
	for host, values := range data {
		if host != "settings" {
			result[host] = legacyProfile{values["ssh_password"]}
		}
	}
	return result, err
}
func legacyLanguage(name string) string {
	data, err := legacyINI(name)
	if err != nil {
		return ""
	}
	return data["settings"]["language"]
}
