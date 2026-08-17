package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIniSaveAndLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "xsync_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	iniPath := filepath.Join(tempDir, "test_xsync.ini")

	profile1 := Profile{
		SSHPassword: "pass123_secret",
		RemoteDir:   "/home/user/workspace",
	}

	if err := SaveProfile(iniPath, "server1", profile1); err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	if err := SetDefaultProfile(iniPath, "server1"); err != nil {
		t.Fatalf("SetDefaultProfile failed: %v", err)
	}

	profiles, err := LoadProfiles(iniPath)
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}

	if p, exists := profiles["server1"]; !exists {
		t.Errorf("server1 profile not found")
	} else if !reflect.DeepEqual(p, profile1) {
		t.Errorf("got profile %v, want %v", p, profile1)
	}

	defaultProfile := GetDefaultProfile(iniPath)
	if defaultProfile != "server1" {
		t.Errorf("got default profile '%s', want 'server1'", defaultProfile)
	}
}

func TestEnsureConfigFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "xsync_ensure_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	created := EnsureConfigFiles(tempDir)
	if len(created) != 3 {
		t.Errorf("expected 3 created files, got %d: %v", len(created), created)
	}

	for _, name := range []string{"xsync.ini", "xsync.push.ini", "xsync.pull.ini"} {
		path := filepath.Join(tempDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", name)
		}
	}

	// Second run should create nothing
	createdSecond := EnsureConfigFiles(tempDir)
	if len(createdSecond) != 0 {
		t.Errorf("expected 0 files on second run, got %d: %v", len(createdSecond), createdSecond)
	}
}

