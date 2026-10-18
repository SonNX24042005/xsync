package serverstore

import (
	"context"
	"strings"
	"time"
	"xsync/internal/session"
)

// CheckKey records successful SSH authentication separately from rsync availability.
func CheckKey(ctx context.Context, host string) error {
	t, err := session.NewSSHTransport(host, "key", "")
	if err != nil {
		return err
	}
	defer t.Close()
	if _, err = t.Run(ctx, "true"); err != nil {
		return err
	}
	return Update(host, func(s *Setting) { s.KeyVerifiedAt = time.Now().UTC() })
}

type ConnectionCheck struct {
	SSHOK     bool   `json:"ssh_ok"`
	SyncReady bool   `json:"sync_ready"`
	Auth      string `json:"auth,omitempty"`
	Error     string `json:"error,omitempty"`
	SyncError string `json:"sync_error,omitempty"`
	Warning   string `json:"warning,omitempty"`
}

func CheckConnection(parent context.Context, host string) (ConnectionCheck, error) {
	settings, err := Read()
	if err != nil {
		return ConnectionCheck{}, err
	}
	password := EffectiveCredentials(settings)[host].Password
	methods := []string{"key"}
	if password != "" {
		methods = append(methods, "password")
	}
	ctx, cancel := context.WithTimeout(parent, 35*time.Second)
	defer cancel()
	var failures []string
	for _, auth := range methods {
		secret := ""
		if auth == "password" {
			secret = password
		}
		t, err := session.NewSSHTransport(host, auth, secret)
		if err == nil {
			_, err = t.Run(ctx, "true")
		}
		if err != nil {
			if t != nil {
				t.Close()
			}
			label := "SSH key"
			if auth == "password" {
				label = "Mật khẩu đã lưu"
			}
			failures = append(failures, label+": "+err.Error())
			if ctx.Err() != nil {
				break
			}
			continue
		}
		defer t.Close()
		result := ConnectionCheck{SSHOK: true, Auth: auth}
		if auth == "key" {
			if err = Update(host, func(s *Setting) { s.KeyVerifiedAt = time.Now().UTC() }); err != nil {
				result.Warning = "Đã kết nối SSH nhưng chưa lưu được dấu key: " + err.Error()
			}
		}
		if err = t.Check(ctx); err != nil {
			result.SyncError = err.Error()
		} else {
			result.SyncReady = true
		}
		return result, nil
	}
	message := strings.Join(failures, "\n")
	if password != "" {
		message = strings.ReplaceAll(message, password, "[redacted]")
	}
	return ConnectionCheck{Error: message}, nil
}
