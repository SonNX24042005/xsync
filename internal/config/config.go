package config

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
