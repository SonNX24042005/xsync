package sync

import (
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

	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		tui.PrintErr(fmt.Sprintf("Dry-run gap loi: %s", string(outBytes)))
		return false
	}

	lines := strings.Split(string(outBytes), "\n")
	var filesToTransfer []string
	var filesToDelete []string
	totalStats := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "deleting ") {
			filesToDelete = append(filesToDelete, strings.TrimPrefix(line, "deleting "))
		} else if strings.HasPrefix(line, "sending incremental") ||
			strings.HasPrefix(line, "receiving incremental") ||
			strings.HasPrefix(line, "sent ") ||
			strings.HasPrefix(line, "speedup is") ||
			strings.HasPrefix(line, "created directory ") ||
			strings.Contains(line, "bytes/sec") {
			continue
		} else if strings.HasPrefix(line, "total size") {
			totalStats = line
		} else if strings.HasSuffix(line, "/") {
			continue
		} else {
			filesToTransfer = append(filesToTransfer, line)
		}
	}

	// Ghi toan bo log chi tiet vao file log tam
	logFileName := fmt.Sprintf("/tmp/xsync_dryrun_%d.log", time.Now().Unix())
	_ = os.WriteFile(logFileName, outBytes, 0o644)

	// Hien thi danh sach file can xoa (neu co)
	if len(filesToDelete) > 0 {
		fmt.Printf("\n  %s[CAN XOA TREN DICH - %d tep]:%s\n", tui.CErr, len(filesToDelete), tui.CReset)
		if len(filesToDelete) <= 10 {
			for _, f := range filesToDelete {
				fmt.Printf("    %s- %s%s\n", tui.CErr, f, tui.CReset)
			}
		} else {
			for i := 0; i < 5; i++ {
				fmt.Printf("    %s- %s%s\n", tui.CErr, filesToDelete[i], tui.CReset)
			}
			fmt.Printf("    %s... (va %d tep xoa khac) ...%s\n", tui.CWarn, len(filesToDelete)-8, tui.CReset)
			for i := len(filesToDelete) - 3; i < len(filesToDelete); i++ {
				fmt.Printf("    %s- %s%s\n", tui.CErr, filesToDelete[i], tui.CReset)
			}
		}
	}

	// Hien thi danh sach file can truyen tai
	fmt.Printf("\n  %s[DANH SACH TEP CAN TRUYEN TAI - %d tep]:%s\n", tui.CInfo, len(filesToTransfer), tui.CReset)
	if len(filesToTransfer) == 0 {
		tui.PrintOk("Khong co tep nao thay doi can truyen tai.")
	} else if len(filesToTransfer) <= 20 {
		for _, f := range filesToTransfer {
			fmt.Printf("    %s+ %s%s\n", tui.COk, f, tui.CReset)
		}
	} else {
		// Hien thi 10 file dau
		for i := 0; i < 10; i++ {
			fmt.Printf("    %s+ %s%s\n", tui.COk, filesToTransfer[i], tui.CReset)
		}
		// Thong bao an bot
		fmt.Printf("    %s... (an %d tep khac de tranh tran man hinh) ...%s\n", tui.CWarn, len(filesToTransfer)-15, tui.CReset)
		// Hien thi 5 file cuoi
		for i := len(filesToTransfer) - 5; i < len(filesToTransfer); i++ {
			fmt.Printf("    %s+ %s%s\n", tui.COk, filesToTransfer[i], tui.CReset)
		}
	}

	// Bang tong ket
	fmt.Println("\n════════════════════════════════════════════════════════════════")
	tui.PrintKV("Tong so tep can truyen", fmt.Sprintf("%d tep", len(filesToTransfer)))
	if len(filesToDelete) > 0 {
		tui.PrintKV("Tong so tep can xoa", fmt.Sprintf("%d tep", len(filesToDelete)))
	}
	if totalStats != "" {
		tui.PrintKV("Thong tin dung luong", totalStats)
	}
	tui.PrintKV("File log chi tiet", logFileName)
	fmt.Println("════════════════════════════════════════════════════════════════")

	return true
}
