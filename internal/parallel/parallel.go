package parallel

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"xsync/internal/tui"
)

var progressPattern = regexp.MustCompile(
	`^\s*([\d,]+)\s+(\d+)%\s+([\d\.]+\w+/s)\s+[\d:]+\s+\(xfr#(\d+),\s+(?:to|ir|re)-chk=(\d+)/(\d+)\)`,
)

type JobProgress struct {
	Percent     int
	Speed       string
	Xfer        int
	Total       int
	CurrentFile string
	Status      string
}

type ProgressTracker struct {
	mu   sync.RWMutex
	jobs []JobProgress
}

func (pt *ProgressTracker) Update(id int, fn func(*JobProgress)) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if id >= 0 && id < len(pt.jobs) {
		fn(&pt.jobs[id])
	}
}

func (pt *ProgressTracker) Get(id int) JobProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	if id >= 0 && id < len(pt.jobs) {
		return pt.jobs[id]
	}
	return JobProgress{}
}

func parseLine(line string, jobID int, tracker *ProgressTracker) {
	match := progressPattern.FindStringSubmatch(line)
	if len(match) >= 7 {
		pct, _ := strconv.Atoi(match[2])
		spd := match[3]
		xfr, _ := strconv.Atoi(match[4])
		tot, _ := strconv.Atoi(match[6])

		tracker.Update(jobID, func(j *JobProgress) {
			j.Percent = pct
			j.Speed = spd
			j.Xfer = xfr
			j.Total = tot
			j.Status = "Syncing"
		})
	} else {
		if !strings.HasPrefix(line, "sending ") &&
			!strings.HasPrefix(line, "receiving ") &&
			!strings.HasPrefix(line, "sent ") &&
			!strings.HasPrefix(line, "total size") &&
			!strings.HasPrefix(line, "speedup is") &&
			!strings.HasPrefix(line, "building file list") &&
			!strings.Contains(line, "bytes/sec") &&
			!strings.Contains(line, "ir-chk") &&
			!strings.Contains(line, "to-chk") {
			tracker.Update(jobID, func(j *JobProgress) {
				j.CurrentFile = line
			})
		}
	}
}

func stdoutReader(r io.Reader, jobID int, tracker *ProgressTracker) {
	buf := make([]byte, 1)
	var lineBuf bytes.Buffer

	for {
		n, err := r.Read(buf)
		if n > 0 {
			b := buf[0]
			if b == '\r' || b == '\n' {
				line := strings.TrimSpace(lineBuf.String())
				lineBuf.Reset()
				if line != "" {
					parseLine(line, jobID, tracker)
				}
			} else {
				lineBuf.WriteByte(b)
			}
		}
		if err != nil {
			break
		}
	}

	tracker.Update(jobID, func(j *JobProgress) {
		j.Status = "Finished"
		j.Percent = 100
		j.Speed = "0 B/s"
	})
}

func stderrReader(r io.Reader, jobID int, errorsList *[]string, mu *sync.Mutex) {
	data, _ := io.ReadAll(r)
	if len(data) > 0 {
		mu.Lock()
		*errorsList = append(*errorsList, fmt.Sprintf("Luong %02d:\n%s", jobID+1, strings.TrimSpace(string(data))))
		mu.Unlock()
	}
}

// RunParallelSync manages parallel rsync transfers.
func RunParallelSync(mode, localDir, host, remoteDir, filterFile string, deleteFiles bool, threads int) bool {
	sshControlPath := fmt.Sprintf("/tmp/rsync-ctrl-%s", host)
	sshCmd := fmt.Sprintf("ssh -o ControlMaster=auto -o ControlPersist=10m -o ControlPath=%s -c aes128-gcm@openssh.com -o Compression=no -o IPQoS=throughput", sshControlPath)

	rsyncBase := []string{
		"-aW",
		"--partial",
		"-e", sshCmd,
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
	}

	tui.PrintInfo("Dang quet danh sach tep thay doi (Dry-run)...")
	dryRunArgs := append([]string{}, rsyncBase...)
	dryRunArgs = append(dryRunArgs, "-v", "--dry-run")
	if filterFile != "" {
		dryRunArgs = append(dryRunArgs, fmt.Sprintf("--filter=merge %s", filterFile))
	}
	if deleteFiles && mode == "push" {
		dryRunArgs = append(dryRunArgs, "--delete")
	}
	dryRunArgs = append(dryRunArgs, src, dest)

	cmdDry := exec.Command("rsync", dryRunArgs...)
	outBytes, err := cmdDry.CombinedOutput()
	if err != nil {
		tui.PrintErr(fmt.Sprintf("Dry-run that bai: %s", string(outBytes)))
		return false
	}

	lines := strings.Split(string(outBytes), "\n")
	var filesToTransfer []string
	var filesToDelete []string

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
			strings.HasPrefix(line, "total size") ||
			strings.HasPrefix(line, "speedup is") ||
			strings.HasPrefix(line, "created directory ") ||
			strings.Contains(line, "bytes/sec") {
			continue
		} else if strings.HasSuffix(line, "/") {
			continue
		} else {
			filesToTransfer = append(filesToTransfer, line)
		}
	}

	tui.PrintInfo(fmt.Sprintf("Tim thay %d tep can truyen tai, %d tep can xoa.", len(filesToTransfer), len(filesToDelete)))

	// 2. Perform deletions first if enabled
	if deleteFiles && len(filesToDelete) > 0 {
		tui.PrintInfo("Dang xoa cac tep thua tren dich...")
		if mode == "pull" {
			for _, f := range filesToDelete {
				target := filepath.Join(localDir, f)
				_ = os.RemoveAll(target)
			}
		} else {
			chunkSize := 300
			for i := 0; i < len(filesToDelete); i += chunkSize {
				end := i + chunkSize
				if end > len(filesToDelete) {
					end = len(filesToDelete)
				}
				chunk := filesToDelete[i:end]
				var quoted []string
				for _, f := range chunk {
					quoted = append(quoted, fmt.Sprintf("'%s'", filepath.Join(remoteDir, f)))
				}
				delCmdStr := fmt.Sprintf("rm -rf %s", strings.Join(quoted, " "))
				delCmd := exec.Command("ssh", "-o", fmt.Sprintf("ControlPath=%s", sshControlPath), host, delCmdStr)
				_ = delCmd.Run()
			}
		}
		tui.PrintOk("Da hoan tat xoa tep thua.")
	}

	if len(filesToTransfer) == 0 {
		tui.PrintOk("Khong co tep nao can truyen tai. Dong bo hoan tat!")
		return true
	}

	// 3. Split files into chunks
	numThreads := threads
	if len(filesToTransfer) < numThreads {
		numThreads = len(filesToTransfer)
	}

	chunks := make([][]string, numThreads)
	for i, f := range filesToTransfer {
		chunks[i%numThreads] = append(chunks[i%numThreads], f)
	}

	var chunkFiles []string
	defer func() {
		for _, cf := range chunkFiles {
			_ = os.Remove(cf)
		}
	}()

	for i := 0; i < numThreads; i++ {
		tmpF, err := os.CreateTemp("", fmt.Sprintf("*_chunk_%d.txt", i))
		if err != nil {
			tui.PrintErr(fmt.Sprintf("Khong the tao file chunk: %v", err))
			return false
		}
		for _, p := range chunks[i] {
			_, _ = tmpF.WriteString(p + "\n")
		}
		tmpF.Close()
		chunkFiles = append(chunkFiles, tmpF.Name())
	}

	// 4. Launch parallel processes
	tui.PrintInfo(fmt.Sprintf("Dang khoi chay %d luong truyen tai song song...", numThreads))

	tracker := &ProgressTracker{
		jobs: make([]JobProgress, numThreads),
	}
	for i := 0; i < numThreads; i++ {
		tracker.jobs[i] = JobProgress{
			Percent:     0,
			Speed:       "0 B/s",
			Xfer:        0,
			Total:       len(chunks[i]),
			CurrentFile: "Dang ket noi...",
			Status:      "Waiting",
		}
	}

	var procs []*exec.Cmd
	var errList []string
	var errMu sync.Mutex

	// Handle Ctrl+C to kill running rsync subprocesses
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	interrupted := false

	go func() {
		<-sigChan
		interrupted = true
		for _, p := range procs {
			if p != nil && p.Process != nil {
				_ = p.Process.Kill()
			}
		}
	}()

	for i := 0; i < numThreads; i++ {
		args := append([]string{}, rsyncBase...)
		args = append(args, "-v", "--info=progress2", fmt.Sprintf("--files-from=%s", chunkFiles[i]), src, dest)

		cmd := exec.Command("rsync", args...)
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			tui.PrintErr(fmt.Sprintf("Loi stdout pipe luong %d: %v", i+1, err))
			return false
		}
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			tui.PrintErr(fmt.Sprintf("Loi stderr pipe luong %d: %v", i+1, err))
			return false
		}

		if err := cmd.Start(); err != nil {
			tui.PrintErr(fmt.Sprintf("Loi khoi chay luong %d: %v", i+1, err))
			return false
		}
		procs = append(procs, cmd)

		go stdoutReader(stdoutPipe, i, tracker)
		go stderrReader(stderrPipe, i, &errList, &errMu)
	}

	// 5. Live progress dashboard
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println(" TIEN TRINH DONG BO PHAN LUONG (PARALLEL SYNC)")
	fmt.Println("════════════════════════════════════════════════════════════════")
	for i := 0; i < numThreads; i++ {
		fmt.Println()
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		if interrupted {
			fmt.Println("\n  [WARN] Da dung truyen tai boi nguoi dung.")
			return false
		}

		allFinished := true
		for i := 0; i < numThreads; i++ {
			if tracker.Get(i).Status != "Finished" {
				allFinished = false
				break
			}
		}

		fmt.Printf("\033[%dF", numThreads)
		for i := 0; i < numThreads; i++ {
			job := tracker.Get(i)
			barWidth := 15
			filled := int(float64(job.Percent) / 100.0 * float64(barWidth))
			if filled > barWidth {
				filled = barWidth
			}
			bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

			fileInfo := job.CurrentFile
			if len(fileInfo) > 30 {
				fileInfo = "..." + fileInfo[len(fileInfo)-27:]
			} else if fileInfo == "" {
				fileInfo = "-"
			}

			if job.Status == "Finished" {
				fmt.Printf("\033[KLuong %02d: [███████████████] 100%% (Hoan thanh) - %d/%d tep\n", i+1, job.Total, job.Total)
			} else {
				fmt.Printf("\033[KLuong %02d: [%s] %3d%% (%s) - %s\n", i+1, bar, job.Percent, job.Speed, fileInfo)
			}
		}

		if allFinished {
			break
		}
	}

	// Wait for all subprocesses
	hasErrors := false
	for _, cmd := range procs {
		if err := cmd.Wait(); err != nil {
			hasErrors = true
		}
	}

	if hasErrors || len(errList) > 0 {
		fmt.Println("════════════════════════════════════════════════════════════════")
		tui.PrintWarn("Co luong truyen tai gap loi. Chi tiet:")
		for _, msg := range errList {
			fmt.Println(msg)
		}
		return false
	}

	fmt.Println("════════════════════════════════════════════════════════════════")
	tui.PrintOk("Dong bo tat ca cac luong hoan tat thanh cong!")
	return true
}
