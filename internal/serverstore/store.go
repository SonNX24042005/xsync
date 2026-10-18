// Package serverstore shares server settings and authentication between terminal and web interfaces.
package serverstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"xsync/internal/config"
)

// A non-nil, empty password explicitly disables fallback to passwords from previous sessions.
type Setting struct {
	Password      *string   `json:"password,omitempty"`
	KeyVerifiedAt time.Time `json:"key_verified_at,omitzero"`
}

type Info struct {
	Host           string    `json:"host"`
	PasswordSaved  bool      `json:"password_saved"`
	PasswordSource string    `json:"password_source,omitempty"`
	KeyVerifiedAt  time.Time `json:"key_verified_at,omitzero"`
}

var settingsMu sync.Mutex

func Path() (string, error) {
	home, err := os.UserHomeDir()
	return filepath.Join(home, ".config", "xsync", "servers.json"), err
}

func Read() (map[string]Setting, error) {
	name, err := Path()
	if err != nil {
		return nil, err
	}
	settings := map[string]Setting{}
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
		settings = map[string]Setting{}
	}
	return settings, nil
}

func Update(host string, update func(*Setting)) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	name, err := Path()
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
	lock, err := os.OpenFile(filepath.Join(dir, ".servers.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err = lockSettings(lock); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("server settings are busy: %w", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer unlockSettings(lock)
	settings, err := Read()
	if err != nil {
		return err
	}
	value := settings[host]
	update(&value)
	settings[host] = value
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

func EffectiveCredentials(settings map[string]Setting) map[string]Credential {
	credentials := SavedCredentials()
	for host, value := range settings {
		if value.Password != nil {
			delete(credentials, host)
			if *value.Password != "" {
				credentials[host] = Credential{*value.Password, "Cấu hình máy chủ"}
			}
		}
	}
	return credentials
}

func Infos(hosts []string, settings map[string]Setting, credentials map[string]Credential) []Info {
	result := make([]Info, 0, len(hosts))
	for _, host := range hosts {
		credential := credentials[host]
		result = append(result, Info{host, credential.Password != "", credential.Source, settings[host].KeyVerifiedAt})
	}
	return result
}

func ConfiguredHost(host string) bool {
	for _, h := range config.ParseSSHHosts() {
		if h == host {
			return true
		}
	}
	return false
}
