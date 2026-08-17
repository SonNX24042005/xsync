package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"

	"xsync/internal/config"
	"xsync/internal/parallel"
	"xsync/internal/pathutils"
	"xsync/internal/sync"
	"xsync/internal/tui"
)

func migrateOldConfigs(configDir string) {
	migrations := [][2]string{
		{".env_rsync", "xsync.ini"},
		{".rsyncinclude.push", "xsync.push.ini"},
		{"xsync.push", "xsync.push.ini"},
		{".rsyncinclude.pull", "xsync.pull.ini"},
		{"xsync.pull", "xsync.pull.ini"},
	}

	for _, m := range migrations {
		oldPath := filepath.Join(configDir, m[0])
		newPath := filepath.Join(configDir, m[1])
		if _, err := os.Stat(oldPath); err == nil {
			if _, errNew := os.Stat(newPath); os.IsNotExist(errNew) {
				if err := os.Rename(oldPath, newPath); err == nil {
					tui.PrintInfo(fmt.Sprintf("Da tu dong di tru %s sang %s", m[0], m[1]))
				}
			}
		}
	}
}

func main() {
	if err := sync.CheckDependencies(); err != nil {
		os.Exit(1)
	}

	configDir, err := os.Getwd()
	if err != nil {
		tui.PrintErr(fmt.Sprintf("Khong the lay thu muc hien tai: %v", err))
		os.Exit(1)
	}

	migrateOldConfigs(configDir)

	createdConfigs := config.EnsureConfigFiles(configDir)
	for _, f := range createdConfigs {
		tui.PrintOk(fmt.Sprintf("Da tu dong tao file cau hinh: %s", f))
	}

	envDir := config.FindConfigNearest("xsync.ini", 8)
	pushDir := config.FindConfigNearest("xsync.push.ini", 8)
	pullDir := config.FindConfigNearest("xsync.pull.ini", 8)

	envFilePath := filepath.Join(configDir, "xsync.ini")
	if envDir != "" {
		envFilePath = filepath.Join(envDir, "xsync.ini")
	}

	var selectedProfile string
	var envVars config.Profile
	envFromTUI := false

	for {
		hosts := config.ParseSSHHosts()
		if len(hosts) == 0 {
			tui.PrintWarn("Khong tim thay SSH Host nao duoc cau hinh trong ~/.ssh/config.")
			tui.PrintInfo("Chuan bi mo ~/.ssh/config de ban tu them Host...")
			fmt.Print("  An Enter de mo file cau hinh SSH...")
			_, _ = bufio.NewReader(os.Stdin).ReadString('\n')

			home, _ := os.UserHomeDir()
			sshConfigPath := filepath.Join(home, ".ssh", "config")
			_ = os.MkdirAll(filepath.Dir(sshConfigPath), 0o700)
			if _, err := os.Stat(sshConfigPath); os.IsNotExist(err) {
				template := `# Vi du cau hinh SSH Host trong ~/.ssh/config
# Host server1
#     HostName 192.168.1.100
#     Port 22
#     User root
`
				_ = os.WriteFile(sshConfigPath, []byte(template), 0o600)
			}
			_ = tui.OpenFileInEditor(sshConfigPath)
			continue
		}

		var options []string
		for _, h := range hosts {
			options = append(options, fmt.Sprintf("SSH Host: %s", h))
		}
		options = append(options, "Mo file ~/.ssh/config de tu them/sua SSH Host...")
		options = append(options, "Mo file xsync.ini de tu sua thu muc REMOTE_DIR/REMOTE_PULL_DIR...")
		options = append(options, "Thoat")

		defaultProfile := config.GetDefaultProfile(envFilePath)
		defaultIdx := -1
		if defaultProfile != "" {
			for i, h := range hosts {
				if h == defaultProfile {
					defaultIdx = i
					break
				}
			}
		}

		idx := tui.PromptMenu("CHON SSH PROFILE (tu ~/.ssh/config)", options, defaultIdx)
		if idx < len(hosts) {
			selectedProfile = hosts[idx]
			_ = config.SetDefaultProfile(envFilePath, selectedProfile)
			break
		} else if idx == len(hosts) {
			home, _ := os.UserHomeDir()
			tui.PrintInfo("Dang mo file ~/.ssh/config trong trinh soan thao...")
			_ = tui.OpenFileInEditor(filepath.Join(home, ".ssh", "config"))
			continue
		} else if idx == len(hosts)+1 {
			tui.PrintInfo("Dang mo file xsync.ini trong trinh soan thao...")
			_ = tui.OpenFileInEditor(envFilePath)
			continue
		} else {
			os.Exit(0)
		}
	}

	for {
		profiles, _ := config.LoadProfiles(envFilePath)
		if p, exists := profiles[selectedProfile]; exists && p.RemoteDir != "" {
			envVars = p
			break
		}

		tui.PrintWarn(fmt.Sprintf("Host '%s' chua duoc cau hinh thu muc REMOTE_DIR trong xsync.ini.", selectedProfile))
		tui.PrintInfo("Chuan bi mo file xsync.ini de ban tu khai bao...")
		fmt.Print("  An Enter de mo file cau hinh...")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')

		if _, exists := profiles[selectedProfile]; !exists {
			_ = config.SaveProfile(envFilePath, selectedProfile, config.Profile{
				SSHPassword: "",
				RemoteDir:   "/path/to/remote/dir",
			})
		}
		_ = tui.OpenFileInEditor(envFilePath)
	}

	if envVars.SSHPassword == "" {
		fmt.Printf("  Nhap mat khau SSH cho '%s' (de trong neu dung SSH key): ", selectedProfile)
		pwdBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err == nil && len(pwdBytes) > 0 {
			envVars.SSHPassword = string(pwdBytes)
			envFromTUI = true
		}
	}

	pushFromTUI := false
	pullFromTUI := false
	var pushIncludeTmp string
	var pullIncludeTmp string
	var generatedFilterFile string

	cleanup := func() {
		if generatedFilterFile != "" {
			_ = os.Remove(generatedFilterFile)
		}
		if pushIncludeTmp != "" {
			_ = os.Remove(pushIncludeTmp)
		}
		if pullIncludeTmp != "" {
			_ = os.Remove(pullIncludeTmp)
		}
		if selectedProfile != "" {
			controlPath := sync.GetSSHControlPath(selectedProfile)
			sync.CleanupSSHMaster(selectedProfile, controlPath)
		}
	}
	defer cleanup()

	// Handle interrupt signal for safe cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cleanup()
		os.Exit(130)
	}()

	controlPath := sync.GetSSHControlPath(selectedProfile)
	tui.PrintInfo(fmt.Sprintf("Dang thiet lap ket noi SSH Master cho host '%s'...", selectedProfile))
	ok, errSSH := sync.SetupSSHMaster(selectedProfile, envVars.SSHPassword, controlPath)
	if !ok {
		tui.PrintErr(fmt.Sprintf("Khong the xac thuc SSH: %s", errSSH))
		os.Exit(1)
	}
	tui.PrintOk("Da thiet lap SSH Master thanh cong!")

	// Delete file with identical name on remote to prevent collision
	if rpath := strings.TrimRight(envVars.RemoteDir, "/"); rpath != "" {
		checkArgs := []string{
			"-o", "ControlMaster=auto",
			"-o", fmt.Sprintf("ControlPath=%s", controlPath),
			"-o", "BatchMode=yes",
			selectedProfile,
			fmt.Sprintf("if [ -f '%s' ]; then rm -f '%s'; fi", rpath, rpath),
		}
		var checkCmd *exec.Cmd
		if envVars.SSHPassword != "" {
			checkCmd = exec.Command("sshpass", append([]string{"-e", "ssh"}, checkArgs...)...)
			checkCmd.Env = append(os.Environ(), "SSHPASS="+envVars.SSHPassword)
		} else {
			checkCmd = exec.Command("ssh", checkArgs...)
		}
		_ = checkCmd.Run()
	}

	modes := []string{
		"Day du lieu tu may len server  (PUSH)",
		"Tai du lieu tu server ve may   (PULL)",
		"Dung lai",
	}
	modeIdx := tui.PromptMenu("CHON CHE DO DONG BO", modes, -1)
	if modeIdx == 2 {
		os.Exit(0)
	}

	syncMode := "push"
	if modeIdx == 1 {
		syncMode = "pull"
	}

	deleteEnabled := false
	var actionTitle, actionStart, actionDone, actionFail string
	var srcPath, destPath string

	if syncMode == "push" {
		actionTitle = "CHUAN BI DAY DU LIEU LEN SERVER"
		actionStart = "BAT DAU DAY DU LIEU"
		actionDone = "HOAN TAT DAY DU LIEU!"
		actionFail = "Day du lieu that bai. Kiem tra ket noi SSH va duong dan."
		srcPath = configDir + "/"
		destPath = fmt.Sprintf("%s:%s/", selectedProfile, strings.TrimRight(envVars.RemoteDir, "/"))

		deleteOpts := []string{
			"Co  (Xoa file tren Server neu khong co trong danh sach push)",
			"Khong (Giu lai file tren Server)",
		}
		delIdx := tui.PromptMenu("CO DUNG FLAG --delete?", deleteOpts, 1)
		deleteEnabled = (delIdx == 0)
	} else {
		actionTitle = "CHUAN BI TAI DU LIEU TU SERVER VE MAY"
		actionStart = "BAT DAU TAI DU LIEU"
		actionDone = "HOAN TAT TAI DU LIEU!"
		actionFail = "Tai du lieu that bai. Kiem tra ket noi SSH va duong dan."
		srcPath = fmt.Sprintf("%s:%s/", selectedProfile, strings.TrimRight(envVars.RemoteDir, "/"))
		destPath = configDir
	}

	includeFilePush := ""
	if pushDir != "" {
		includeFilePush = filepath.Join(pushDir, "xsync.push.ini")
	}
	includeFilePull := ""
	if pullDir != "" {
		includeFilePull = filepath.Join(pullDir, "xsync.pull.ini")
	}

	createTemplate := func(mode, localVal, remoteVal string) string {
		baseHint := localVal
		if mode == "pull" {
			baseHint = remoteVal
		}
		return fmt.Sprintf(`# Duong dan tuong doi so voi: %s
# Thu muc ket bang /    Vi du:  mlops/datasets/
# File khong co /       Vi du:  upload_run.py
# Keo tha file/folder vao day: path tuyet doi se tu dong chuyen thanh tuong doi

`, baseHint)
	}

	if syncMode == "push" {
		if includeFilePush == "" || !fileExists(includeFilePush) {
			tui.PrintWarn("Khong tim thay xsync.push.ini.")
			tui.PrintInfo("Hien giao dien nhap danh sach push...")
			tpl := createTemplate("push", configDir, envVars.RemoteDir)
			rawText, _ := tui.EditTextInEditor(tpl, "_push_whitelist")

			lines := strings.Split(rawText, "\n")
			processed := pathutils.ProcessIncludePaths(lines, "push", configDir, envVars.RemoteDir)

			tmpF, err := os.CreateTemp("", "*_push_inc")
			if err == nil {
				_, _ = tmpF.WriteString(strings.Join(processed, "\n") + "\n")
				tmpF.Close()
				pushIncludeTmp = tmpF.Name()
				includeFilePush = pushIncludeTmp
				pushFromTUI = true
			}
		} else {
			tui.PrintInfo(fmt.Sprintf("Dung xsync.push.ini tai: %s", pushDir))
		}
	} else {
		if includeFilePull == "" || !fileExists(includeFilePull) {
			tui.PrintWarn("Khong tim thay xsync.pull.ini.")
			tui.PrintInfo("Hien giao dien nhap danh sach pull...")
			tpl := createTemplate("pull", configDir, envVars.RemoteDir)
			rawText, _ := tui.EditTextInEditor(tpl, "_pull_whitelist")

			lines := strings.Split(rawText, "\n")
			processed := pathutils.ProcessIncludePaths(lines, "pull", configDir, envVars.RemoteDir)

			tmpF, err := os.CreateTemp("", "*_pull_inc")
			if err == nil {
				_, _ = tmpF.WriteString(strings.Join(processed, "\n") + "\n")
				tmpF.Close()
				pullIncludeTmp = tmpF.Name()
				includeFilePull = pullIncludeTmp
				pullFromTUI = true
			}
		} else {
			tui.PrintInfo(fmt.Sprintf("Dung xsync.pull.ini tai: %s", pullDir))
		}
	}

	tui.PrintHeader(actionTitle)
	tui.PrintKV("Nguon", srcPath)
	tui.PrintKV("Dich", destPath)
	tui.PrintKV("SSH Host", selectedProfile)
	tui.PrintKV("Config dir", configDir)

	activeWhitelistFile := includeFilePush
	if syncMode == "pull" {
		activeWhitelistFile = includeFilePull
	}

	if activeWhitelistFile != "" && fileExists(activeWhitelistFile) {
		data, err := os.ReadFile(activeWhitelistFile)
		if err == nil {
			lines := strings.Split(string(data), "\n")
			cleanedLines := pathutils.ProcessIncludePaths(lines, syncMode, configDir, envVars.RemoteDir)
			rules := pathutils.BuildIncludeFilter(cleanedLines)
			if len(rules) > 0 {
				tmpF, err := os.CreateTemp("", "*_rsync_filter")
				if err == nil {
					_, _ = tmpF.WriteString(strings.Join(rules, "\n") + "\n")
					tmpF.Close()
					generatedFilterFile = tmpF.Name()
					tui.PrintOk(fmt.Sprintf("Da ap dung whitelist %s", syncMode))
				}
			} else {
				tui.PrintWarn(fmt.Sprintf("Whitelist %s khong co entry hop le — %s TOAN BO thu muc.", syncMode, syncMode))
			}
		}
	} else {
		tui.PrintWarn(fmt.Sprintf("Khong co whitelist %s — %s TOAN BO thu muc.", syncMode, syncMode))
	}

	// 5. Run Dry-run
	tui.PrintHeader("DANG CHAY THU (DRY-RUN)")
	dryOk := sync.RunDryRun(syncMode, configDir, selectedProfile, envVars.RemoteDir, generatedFilterFile, deleteEnabled)
	if !dryOk {
		tui.PrintErr("Dry-run that bai. Kiem tra SSH, quyen truy cap, duong dan remote va port.")
		os.Exit(1)
	}

	// 6. Confirm execution
	confirmOpts := []string{
		"Tiep tuc (chay THAT)",
		"Dung lai",
	}
	confIdx := tui.PromptMenu("XAC NHAN CHAY THAT", confirmOpts, 0)
	if confIdx == 1 {
		os.Exit(0)
	}

	// 7. Execute parallel sync
	tui.PrintHeader(actionStart)
	syncOk := parallel.RunParallelSync(
		syncMode,
		configDir,
		selectedProfile,
		envVars.RemoteDir,
		generatedFilterFile,
		deleteEnabled,
		4,
	)

	if syncOk {
		tui.PrintOk(actionDone)
		fmt.Println()

		if envFromTUI || pushFromTUI || pullFromTUI {
			var saveItems []tui.ChecklistItem
			if envFromTUI {
				saveItems = append(saveItems, tui.ChecklistItem{Key: "env", Label: "Luu xsync.ini (SSH credentials + duong dan)", Checked: true})
			}
			if pushFromTUI {
				saveItems = append(saveItems, tui.ChecklistItem{Key: "push", Label: "Luu xsync.push.ini (danh sach files push)", Checked: true})
			}
			if pullFromTUI {
				saveItems = append(saveItems, tui.ChecklistItem{Key: "pull", Label: "Luu xsync.pull.ini (danh sach files pull)", Checked: true})
			}

			selectedSaves := tui.PromptChecklist("LUU CAU HINH", saveItems)
			saveSet := make(map[string]bool)
			for _, s := range selectedSaves {
				saveSet[s] = true
			}

			if saveSet["env"] {
				_ = config.SaveProfile(envFilePath, selectedProfile, envVars)
				tui.PrintOk(fmt.Sprintf("Da luu profile '%s' vao xsync.ini", selectedProfile))
			}
			if saveSet["push"] && pushIncludeTmp != "" && fileExists(pushIncludeTmp) {
				copyFile(pushIncludeTmp, filepath.Join(configDir, "xsync.push.ini"))
				tui.PrintOk("Da luu: xsync.push.ini")
			}
			if saveSet["pull"] && pullIncludeTmp != "" && fileExists(pullIncludeTmp) {
				copyFile(pullIncludeTmp, filepath.Join(configDir, "xsync.pull.ini"))
				tui.PrintOk("Da luu: xsync.pull.ini")
			}
		}
		os.Exit(0)
	} else {
		tui.PrintErr(actionFail)
		os.Exit(1)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func copyFile(src, dst string) {
	data, err := os.ReadFile(src)
	if err == nil {
		_ = os.WriteFile(dst, data, 0o644)
	}
}
