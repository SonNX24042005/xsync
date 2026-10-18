package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// stableExecutable also makes sessions created via `go run` survive its temporary binary.
func stableExecutable() (string, error) {
	source, err := os.Executable()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	root, err := Root()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, ".bin")
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	name := fmt.Sprintf("xsync-%x", sum[:8])
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(dir, name)
	if _, err = os.Stat(target); err == nil {
		return target, nil
	}
	f, err := os.CreateTemp(dir, ".binary-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Chmod(0o700); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(f.Name(), target); err != nil {
		return "", err
	}
	return target, nil
}

func serviceCommand(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func systemdQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%", "\n", `\n`, "\r", `\r`).Replace(s) + `"`
}

func systemdUnit(executable, id, envPath string) string {
	return "[Unit]\nDescription=xsync folder mirror " + id + "\n\n[Service]\nType=simple\n" +
		"ExecStart=" + systemdQuote(strings.ReplaceAll(executable, "$", "$$")) + " " + WorkerCommand + " " + id + "\n" +
		"Environment=" + systemdQuote("PATH="+envPath) + "\nRestart=on-failure\nRestartSec=5\nTimeoutStopSec=20\nKillMode=control-group\nUMask=0077\n\n[Install]\nWantedBy=default.target\n"
}

func xmlText(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
func launchAgent(executable, id, envPath, logPath string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>Label</key><string>io.xsync.` + id + `</string>
<key>ProgramArguments</key><array><string>` + xmlText(executable) + `</string><string>` + WorkerCommand + `</string><string>` + id + `</string></array>
<key>EnvironmentVariables</key><dict><key>PATH</key><string>` + xmlText(envPath) + `</string></dict>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/><key>ThrottleInterval</key><integer>5</integer>
<key>StandardErrorPath</key><string>` + xmlText(logPath) + `</string></dict></plist>`
}

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func Start(id string) error {
	c, err := Load(id)
	if err != nil {
		return err
	}
	if c.Auth == "password" && c.Password == "" {
		return fmt.Errorf("save a password or choose SSH key authentication before enabling startup")
	}
	if Running(id) {
		return fmt.Errorf("session is already running; stop it before enabling/restarting its service")
	}
	executable, err := stableExecutable()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "linux":
		if err = serviceCommand("systemctl", "--user", "show-environment"); err != nil {
			return fmt.Errorf("a systemd user manager is required; choose 'Run in this terminal' in the session menu: %w", err)
		}
		// Lingering starts the user manager at boot even before an interactive login.
		uid := fmt.Sprint(os.Getuid())
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		out, _ := exec.CommandContext(ctx, "loginctl", "show-user", uid, "-p", "Linger", "--value").Output()
		cancel()
		if strings.TrimSpace(string(out)) != "yes" {
			if err = serviceCommand("loginctl", "enable-linger", uid); err != nil {
				return fmt.Errorf("boot startup requires user lingering; enable it with 'loginctl enable-linger %s' then retry: %w", uid, err)
			}
		}
		dir := filepath.Join(home, ".config", "systemd", "user")
		if err = os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		unit := "xsync-" + id + ".service"
		if err = os.WriteFile(filepath.Join(dir, unit), []byte(systemdUnit(executable, id, os.Getenv("PATH"))), 0o600); err != nil {
			return err
		}
		if err = serviceCommand("systemctl", "--user", "daemon-reload"); err != nil {
			return err
		}
		return serviceCommand("systemctl", "--user", "enable", "--now", unit)
	case "darwin":
		dir := filepath.Join(home, "Library", "LaunchAgents")
		if err = os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		file := filepath.Join(dir, "io.xsync."+id+".plist")
		stateDir, _ := Dir(id)
		if err = os.WriteFile(file, []byte(launchAgent(executable, id, os.Getenv("PATH"), filepath.Join(stateDir, "service.log"))), 0o600); err != nil {
			return err
		}
		return serviceCommand("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), file)
	case "windows":
		script := `$ErrorActionPreference='Stop'; $user=[System.Security.Principal.WindowsIdentity]::GetCurrent().Name; ` +
			`$a=New-ScheduledTaskAction -Execute ` + psQuote(executable) + ` -Argument ` + psQuote(WorkerCommand+" "+id) + `; ` +
			`$t=New-ScheduledTaskTrigger -AtLogOn -User $user; ` +
			`$s=New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -StartWhenAvailable -MultipleInstances IgnoreNew; ` +
			`Register-ScheduledTask -TaskName ` + psQuote("xsync-"+id) + ` -Action $a -Trigger $t -Settings $s -User $user -RunLevel Limited -Force | Out-Null; Start-ScheduledTask -TaskName ` + psQuote("xsync-"+id)
		return serviceCommand("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	}
	return fmt.Errorf("startup is unsupported on %s", runtime.GOOS)
}

// Stop disables startup as well as stopping the managed service. It preserves configuration/data.
func Stop(id string) error {
	if _, err := Load(id); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "linux":
		return serviceCommand("systemctl", "--user", "disable", "--now", "xsync-"+id+".service")
	case "darwin":
		file := filepath.Join(home, "Library", "LaunchAgents", "io.xsync."+id+".plist")
		if err = serviceCommand("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), file); err != nil {
			return err
		}
		return os.Remove(file)
	case "windows":
		name := psQuote("xsync-" + id)
		return serviceCommand("powershell", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; Stop-ScheduledTask -TaskName `+name+`; Unregister-ScheduledTask -TaskName `+name+` -Confirm:$false`)
	}
	return fmt.Errorf("startup is unsupported on %s", runtime.GOOS)
}
