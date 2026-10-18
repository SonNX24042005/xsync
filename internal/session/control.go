package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const WorkerCommand = "_worker"

type workerControl struct {
	Nonce string `json:"nonce"`
}

func controlWorker(id string, cancel context.CancelFunc) (func(), error) {
	dir, err := Dir(id)
	if err != nil {
		return nil, err
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return nil, err
	}
	control := workerControl{Nonce: hex.EncodeToString(random[:])}
	if err = writeJSON(filepath.Join(dir, "control.json"), control); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				data, err := os.ReadFile(filepath.Join(dir, "stop.json"))
				if err != nil {
					continue
				}
				var request workerControl
				if json.Unmarshal(data, &request) == nil && request.Nonce == control.Nonce {
					cancel()
					return
				}
			}
		}
	}()
	return func() {
		close(done)
		<-finished
		os.Remove(filepath.Join(dir, "control.json"))
		os.Remove(filepath.Join(dir, "stop.json"))
	}, nil
}

// RequestStop addresses the worker holding this session's lock, without using a
// potentially stale process ID or exposing a network control port.
func RequestStop(id string) error {
	if _, err := Load(id); err != nil {
		return err
	}
	if !Running(id) {
		return nil
	}
	dir, _ := Dir(id)
	data, err := os.ReadFile(filepath.Join(dir, "control.json"))
	if err != nil {
		return fmt.Errorf("worker cannot be controlled by this version; stop its terminal or service: %w", err)
	}
	var control workerControl
	if err = json.Unmarshal(data, &control); err != nil {
		return err
	}
	if control.Nonce == "" {
		return fmt.Errorf("invalid worker control record")
	}
	if err = writeJSON(filepath.Join(dir, "stop.json"), control); err != nil {
		return err
	}
	deadline := time.Now().Add(20 * time.Second)
	for Running(id) {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out stopping session %s", id)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func StopAny(id string) error {
	if err := Stop(id); err == nil {
		return nil
	} else if !Running(id) {
		return err
	}
	return RequestStop(id)
}

func StartDetached(c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if _, err := Load(c.ID); err != nil {
		return err
	}
	if Running(c.ID) {
		return fmt.Errorf("session is already running")
	}
	if c.Auth == "password" && c.Password == "" {
		return fmt.Errorf("a password is required")
	}
	executable, err := stableExecutable()
	if err != nil {
		return err
	}
	dir, _ := Dir(c.ID)
	log, err := os.OpenFile(filepath.Join(dir, "launcher.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command(executable, WorkerCommand, c.ID)
	cmd.Env = os.Environ()
	if c.Auth == "password" {
		cmd.Env = append(cmd.Env, "XSYNC_SESSION_PASSWORD="+c.Password)
	}
	cmd.Stdout, cmd.Stderr = log, log
	detachCommand(cmd)
	if err = cmd.Start(); err != nil {
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-exited:
			if err == nil {
				err = fmt.Errorf("worker exited before starting")
			}
			return err
		case <-ticker.C:
			state := ReadStatus(c.ID)
			if Running(c.ID) && state.PID == cmd.Process.Pid && state.Phase != "stopped" && state.Phase != "" {
				return nil
			}
		case <-deadline.C:
			_ = cmd.Process.Kill()
			return fmt.Errorf("worker did not start")
		}
	}
}
