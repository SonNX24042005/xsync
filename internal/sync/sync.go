package sync

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"xsync/internal/tui"
)

func CheckDependencies() error {
	if _, err := exec.LookPath("sshpass"); err != nil {
		tui.PrintErr("Khong tim thay sshpass. Cai dat: sudo apt install sshpass")
		return fmt.Errorf("missing dependency: sshpass")
	}
	if _, err := exec.LookPath("rsync"); err != nil {
		tui.PrintErr("Khong tim thay rsync. Cai dat: sudo apt install rsync")
		return fmt.Errorf("missing dependency: rsync")
	}
	return nil
}

func GetSSHControlPath(host string) string {
	return fmt.Sprintf("/tmp/rsync-ctrl-%s", host)
}

func SetupSSHMaster(host, password, controlPath string) (bool, string) {
	if _, err := os.Stat(controlPath); err == nil {
		_ = os.Remove(controlPath)
	}

	sshArgs := []string{
		"-o", "ControlMaster=yes",
		"-o", "ControlPersist=10m",
		"-o", "ControlPath=" + controlPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "BatchMode=no",
		"-c", "aes128-gcm@openssh.com",
		"-o", "Compression=no",
		"-o", "IPQoS=throughput",
		"-Nf", host,
	}

	var cmd *exec.Cmd
	if password != "" {
		cmd = exec.Command("sshpass", append([]string{"-e", "ssh"}, sshArgs...)...)
		cmd.Env = append(os.Environ(), "SSHPASS="+password)
	} else {
		cmd = exec.Command("ssh", sshArgs...)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return false, strings.TrimSpace(stderr.String())
	}

	// Cho toi da 5 giay de file socket duoc tao thanh cong boi daemon SSH chay ngam
	socketReady := false
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(controlPath); err == nil {
			socketReady = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !socketReady {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = "Khong the tao SSH Master socket. Vui long kiem tra lai mat khau SSH hoac ket noi toi host."
		}
		return false, errMsg
	}

	return true, ""
}

func CleanupSSHMaster(host, controlPath string) {
	if _, err := os.Stat(controlPath); err == nil {
		cmd := exec.Command("ssh", "-o", "ControlPath="+controlPath, "-O", "exit", host)
		_ = cmd.Run()
		_ = os.Remove(controlPath)
	}
}

func RunDryRun(mode, localDir, host, remoteDir, filterFile string, delete bool) bool {
	controlPath := GetSSHControlPath(host)
	sshCmd := fmt.Sprintf("ssh -o ControlMaster=auto -o ControlPersist=10m -o ControlPath=%s -o BatchMode=yes -c aes128-gcm@openssh.com -o Compression=no -o IPQoS=throughput", controlPath)

	args := []string{
		"-avW",
		"--partial",
		"--info=progress2",
		"-e", sshCmd,
		"--dry-run",
	}

	if filterFile != "" {
		args = append(args, fmt.Sprintf("--filter=merge %s", filterFile))
	}
	if delete && mode == "push" {
		args = append(args, "--delete")
	}

	cleanLocal := strings.TrimRight(localDir, "/") + "/"
	cleanRemote := strings.TrimRight(remoteDir, "/") + "/"

	var src, dest string
	if mode == "push" {
		src = cleanLocal
		dest = fmt.Sprintf("%s:%s", host, cleanRemote)
	} else {
		src = fmt.Sprintf("%s:%s", host, cleanRemote)
		dest = cleanLocal
		_ = os.MkdirAll(localDir, 0o755)
	}

	args = append(args, src, dest)
	cmd := exec.Command("rsync", args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		tui.PrintErr(fmt.Sprintf("Khong the mo pipe rsync: %v", err))
		return false
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		tui.PrintErr(fmt.Sprintf("Khong the khoi chay rsync: %v", err))
		return false
	}

	scanner := bufio.NewScanner(stdoutPipe)
	for scanner.Scan() {
		fmt.Printf("  %s\n", scanner.Text())
	}

	if err := cmd.Wait(); err != nil {
		return false
	}
	return true
}
