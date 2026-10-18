package serverstore

import (
	"sort"

	"xsync/internal/session"
)

type Credential struct {
	Password string
	Source   string
}

// Keep credentials on the server. Only their availability is sent to the UI.
func SavedCredentials() map[string]Credential {
	result := map[string]Credential{}

	sessions, _ := session.List()
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].CreatedAt.After(sessions[j].CreatedAt) })
	for _, s := range sessions {
		if s.Auth == "password" && s.Password != "" && result[s.Host].Password == "" {
			result[s.Host] = Credential{s.Password, "Phiên " + s.ID}
		}
	}
	return result
}
