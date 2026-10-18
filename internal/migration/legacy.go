// Package migration archives legacy configuration after importing credentials.
package migration

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"xsync/internal/i18n"
	"xsync/internal/serverstore"
)

// Legacy only visits the three known files in dir and the old global language
// file. It never creates sessions or starts transfers from legacy whitelists.
func Legacy(dir string) (string, error) {
	var names []string
	for _, name := range []string{"xsync.ini", "xsync.push.ini", "xsync.pull.ini"} {
		names = append(names, filepath.Join(dir, name))
	}
	prefs := i18n.GetGlobalConfigPath()
	if prefs != "" {
		names = append(names, filepath.Join(filepath.Dir(prefs), "config.ini"))
	}
	files := map[string][]byte{}
	for _, name := range names {
		info, err := os.Lstat(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("legacy configuration must be a regular file: %s", name)
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return "", err
		}
		files[name] = data
	}
	if len(files) == 0 {
		return "", nil
	}
	settings, err := serverstore.Read()
	if err != nil {
		return "", err
	}
	profiles := filepath.Join(dir, "xsync.ini")
	if _, ok := files[profiles]; ok {
		values, err := legacyProfiles(profiles)
		if err != nil {
			return "", err
		}
		for host, value := range values {
			if value.SSHPassword == "" || settings[host].Password != nil {
				continue
			}
			secret := value.SSHPassword
			if err = serverstore.Update(host, func(s *serverstore.Setting) {
				if s.Password == nil {
					s.Password = &secret
				}
			}); err != nil {
				return "", err
			}
		}
	}
	// Preserve an explicitly saved JSON language; import INI only when absent.
	if prefs != "" {
		data, err := os.ReadFile(prefs)
		if err == nil {
			var value map[string]any
			if err = json.Unmarshal(data, &value); err != nil {
				return "", err
			}
		} else if os.IsNotExist(err) {
			lang := legacyLanguage(profiles)
			if lang == "" {
				lang = legacyLanguage(filepath.Join(filepath.Dir(prefs), "config.ini"))
			}
			if lang == "" {
				lang = i18n.GetLanguage()
			}
			if err = i18n.SaveGlobalLanguage(lang); err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(home, ".config", "xsync", "migrations")
	if err = os.MkdirAll(base, 0o700); err != nil {
		return "", err
	}
	backup, err := os.MkdirTemp(base, time.Now().UTC().Format("20060102T150405Z")+"-")
	if err != nil {
		return "", err
	}
	manifest := map[string]string{}
	for name, data := range files {
		target := filepath.Join(backup, filepath.Base(name)+".bak")
		if err = os.WriteFile(target, data, 0o600); err != nil {
			return "", err
		}
		copy, err := os.ReadFile(target)
		if err != nil || !bytes.Equal(data, copy) {
			return "", fmt.Errorf("legacy backup verification failed: %s", name)
		}
		manifest[name] = fmt.Sprintf("%x", sha256.Sum256(data))
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(backup, "manifest.json"), data, 0o600); err != nil {
		return "", err
	}
	for name, original := range files {
		current, err := os.ReadFile(name)
		if err != nil {
			return "", err
		}
		if !bytes.Equal(original, current) {
			return "", fmt.Errorf("legacy configuration changed during migration: %s", name)
		}
		if err = os.Remove(name); err != nil {
			return "", err
		}
	}
	return backup, nil
}
