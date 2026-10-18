package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xsync/internal/config"
	"xsync/internal/session"
)

// A non-nil, empty password explicitly disables fallback to legacy profiles.
type serverSetting struct {
	Password      *string   `json:"password,omitempty"`
	KeyVerifiedAt time.Time `json:"key_verified_at,omitzero"`
}

type serverInfo struct {
	Host           string    `json:"host"`
	PasswordSaved  bool      `json:"password_saved"`
	PasswordSource string    `json:"password_source,omitempty"`
	KeyVerifiedAt  time.Time `json:"key_verified_at,omitzero"`
}

var settingsMu sync.Mutex

func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	return filepath.Join(home, ".config", "xsync", "servers.json"), err
}

func readSettings() (map[string]serverSetting, error) {
	name, err := settingsPath()
	if err != nil {
		return nil, err
	}
	settings := map[string]serverSetting{}
	data, err := os.ReadFile(name)
	if os.IsNotExist(err) {
		return settings, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("Không đọc được cấu hình máy chủ: %w", err)
	}
	if settings == nil {
		settings = map[string]serverSetting{}
	}
	return settings, nil
}

func updateSetting(host string, update func(*serverSetting)) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	settings, err := readSettings()
	if err != nil {
		return err
	}
	value := settings[host]
	update(&value)
	settings[host] = value
	name, err := settingsPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(name)
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err = os.Chmod(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".servers-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = json.NewEncoder(f).Encode(settings); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), name)
}

func effectiveCredentials(settings map[string]serverSetting) map[string]savedCredential {
	credentials := savedCredentials()
	for host, value := range settings {
		if value.Password != nil {
			delete(credentials, host)
			if *value.Password != "" {
				credentials[host] = savedCredential{*value.Password, "Cấu hình máy chủ"}
			}
		}
	}
	return credentials
}

func serverInfos(hosts []string, settings map[string]serverSetting, credentials map[string]savedCredential) []serverInfo {
	result := make([]serverInfo, 0, len(hosts))
	for _, host := range hosts {
		credential := credentials[host]
		result = append(result, serverInfo{host, credential.password != "", credential.source, settings[host].KeyVerifiedAt})
	}
	return result
}

func configuredHost(host string) bool {
	for _, h := range config.ParseSSHHosts() {
		if h == host {
			return true
		}
	}
	return false
}

func (a *App) saveServer(w http.ResponseWriter, r *http.Request) {
	host := r.PathValue("host")
	if !configuredHost(host) {
		jsonError(w, 400, fmt.Errorf("Chọn một máy chủ trong SSH config."))
		return
	}
	var req struct {
		Password *string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, 400, err)
		return
	}
	if req.Password == nil {
		jsonError(w, 400, fmt.Errorf("Nhập mật khẩu mới hoặc chuỗi rỗng để ngừng dùng mật khẩu đã lưu."))
		return
	}
	if err := updateSetting(host, func(s *serverSetting) { s.Password = req.Password }); err != nil {
		jsonError(w, 500, err)
		return
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}

// Check keys separately from rsync availability. Remember only proven SSH access.
func (a *App) checkServerKey(w http.ResponseWriter, r *http.Request) {
	host := r.PathValue("host")
	if !configuredHost(host) {
		jsonError(w, 400, fmt.Errorf("Chọn một máy chủ trong SSH config."))
		return
	}
	t, err := session.NewSSHTransport(host, "key", "")
	if err == nil {
		defer t.Close()
		_, err = t.Run(r.Context(), "true")
	}
	if err != nil {
		jsonError(w, 400, err)
		return
	}
	if err = updateSetting(host, func(s *serverSetting) { s.KeyVerifiedAt = time.Now().UTC() }); err != nil {
		jsonError(w, 500, err)
		return
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}

type connectionCheck struct {
	SSHOK     bool   `json:"ssh_ok"`
	SyncReady bool   `json:"sync_ready"`
	Auth      string `json:"auth,omitempty"`
	Error     string `json:"error,omitempty"`
	SyncError string `json:"sync_error,omitempty"`
	Warning   string `json:"warning,omitempty"`
}

func (a *App) checkServerConnection(w http.ResponseWriter, r *http.Request) {
	host := r.PathValue("host")
	if !configuredHost(host) {
		jsonError(w, 400, fmt.Errorf("Chọn một máy chủ trong SSH config."))
		return
	}
	settings, err := readSettings()
	if err != nil {
		jsonError(w, 500, err)
		return
	}
	password := effectiveCredentials(settings)[host].password
	methods := []string{"key"}
	if password != "" {
		methods = append(methods, "password")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
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
		result := connectionCheck{SSHOK: true, Auth: auth}
		if auth == "key" {
			if err = updateSetting(host, func(s *serverSetting) { s.KeyVerifiedAt = time.Now().UTC() }); err != nil {
				result.Warning = "Đã kết nối SSH nhưng chưa lưu được dấu key: " + err.Error()
			}
		}
		if err = t.Check(ctx); err != nil {
			result.SyncError = err.Error()
		} else {
			result.SyncReady = true
		}
		jsonResponse(w, 200, result)
		return
	}
	message := strings.Join(failures, "\n")
	if password != "" {
		message = strings.ReplaceAll(message, password, "[redacted]")
	}
	jsonResponse(w, 200, connectionCheck{Error: message})
}
