package webui

import (
	"os"
	"path/filepath"
	"sort"

	"xsync/internal/config"
	"xsync/internal/session"
)

type savedCredential struct {
	password string
	source   string
}

// Keep credentials on the server. Only their availability is sent to the UI.
func savedCredentials() map[string]savedCredential {
	result := map[string]savedCredential{}
	// Prefer the nearest project profile, without scanning unrelated descendants.
	dir, _ := os.Getwd()
	for dir != "" {
		profiles, err := config.LoadProfiles(filepath.Join(dir, "xsync.ini"))
		if err == nil {
			for host, profile := range profiles {
				if profile.SSHPassword != "" && result[host].password == "" {
					result[host] = savedCredential{profile.SSHPassword, "xsync.ini"}
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	sessions, _ := session.List()
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].CreatedAt.After(sessions[j].CreatedAt) })
	for _, s := range sessions {
		if s.Auth == "password" && s.Password != "" && result[s.Host].password == "" {
			result[s.Host] = savedCredential{s.Password, "Phiên " + s.ID}
		}
	}
	return result
}
