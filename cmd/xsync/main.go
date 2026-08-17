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
	// 1. Kiem tra phu thuoc he thong (Fatal Error neu thieu)
	if err := sync.CheckDependencies(); err != nil {
		os.Exit(1)
	}

	configDir, err := os.Getwd()
	if err != nil {
		tui.PrintErr(fmt.Sprintf("Khong the lay thu muc hien tai: %v", err))
		os.Exit(1)
	}

	migrateOldConfigs(configDir)

	envFilePath := filepath.Join(configDir, "xsync.ini")
	if !fileExists(envFilePath) {
		nearest := config.FindConfigNearest("xsync.ini", 8)
		if nearest != "" {
			envFilePath = filepath.Join(nearest, "xsync.ini")
		}
	}

	// 2. Tu dong tao xsync.ini neu chua ton tai va mo editor cho nguoi dung nhap
	if !fileExists(envFilePath) {
		tui.PrintWarn("Chua tim thay file xsync.ini tai thu muc hien tai.")
		_ = config.EnsureConfigFiles(configDir)
		envFilePath = filepath.Join(configDir, "xsync.ini")
		tui.PrintOk("Da tu dong tao file mau xsync.ini tai thu muc hien tai.")
		tui.PrintInfo("Chuan bi mo xsync.ini trong trinh soan thao de ban khai bao SSH Host va REMOTE_DIR...")
		fmt.Print("  An Enter de mo file cau hinh...")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		_ = tui.OpenFileInEditor(envFilePath)
	}

	var selectedProfile string
	var envVars config.Profile
	envFromTUI := false

	// 3. Chon SSH Profile tu ~/.ssh/config (co xu ly Fatal Error neu chua co host)
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
# Host my-server
#     HostName 192.168.1.100
#     Port 22
#     User root
`
				_ = os.WriteFile(sshConfigPath, []byte(template), 0o600)
			}
			_ = tui.OpenFileInEditor(sshConfigPath)

			// Kiem tra lai sau khi sua
			if len(config.ParseSSHHosts()) == 0 {
				retryOpts := []string{
					"Mo lai file ~/.ssh/config de them Host",
					"Dung lai",
				}
				cIdx := tui.PromptMenu("CHUA CO SSH HOST", retryOpts, 0)
				if cIdx == 1 {
					os.Exit(0)
				}
			}
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

	// 4. Kiem tra va bat buoc nhap REMOTE_DIR trong xsync.ini neu chua co
	for {
		profiles, _ := config.LoadProfiles(envFilePath)
		if p, exists := profiles[selectedProfile]; exists && strings.TrimSpace(p.RemoteDir) != "" && p.RemoteDir != "/path/to/remote/dir" {
			envVars = p
			break
		}

		tui.PrintWarn(fmt.Sprintf("Host '%s' chua duoc khai bao thu muc REMOTE_DIR hop le trong xsync.ini.", selectedProfile))
		fixOpts := []string{
			"Mo file xsync.ini trong trinh soan thao de dien REMOTE_DIR",
			"Chon lai SSH Host khac",
			"Dung lai",
		}
		fIdx := tui.PromptMenu("CAU HINH THIEU REMOTE_DIR", fixOpts, 0)
		if fIdx == 0 {
			if _, exists := profiles[selectedProfile]; !exists {
				_ = config.SaveProfile(envFilePath, selectedProfile, config.Profile{
					SSHPassword: "",
					RemoteDir:   "/path/to/remote/dir",
				})
			}
			_ = tui.OpenFileInEditor(envFilePath)
		} else if fIdx == 1 {
			// Restart profile choice
			return
		} else {
			os.Exit(0)
		}
	}

	// 5. Thiet lap va kiem tra ket noi SSH Master Socket (Co vong lap thu lai neu sai password)
	controlPath := sync.GetSSHControlPath(selectedProfile)
	cleanup := func() {
		if selectedProfile != "" {
			sync.CleanupSSHMaster(selectedProfile, controlPath)
		}
	}
	defer cleanup()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cleanup()
		os.Exit(130)
	}()

	for {
		tui.PrintInfo(fmt.Sprintf("Dang thiet lap ket noi SSH Master cho host '%s'...", selectedProfile))
		ok, errSSH := sync.SetupSSHMaster(selectedProfile, envVars.SSHPassword, controlPath)
		if ok {
			tui.PrintOk("Da thiet lap SSH Master thanh cong!")
			break
		}

		tui.PrintErr(fmt.Sprintf("Khong the ket noi SSH toi host '%s': %s", selectedProfile, errSSH))
		authOpts := []string{
			"Nhap mat khau SSH bang tay de thu lai",
			"Mo file xsync.ini de kiem tra va sua lai cau hinh",
			"Dung lai",
		}
		aIdx := tui.PromptMenu("LOI XAC THUC / KET NOI SSH", authOpts, 0)
		if aIdx == 0 {
			fmt.Printf("  Nhap mat khau SSH cho '%s': ", selectedProfile)
			pwdBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err == nil {
				envVars.SSHPassword = string(pwdBytes)
				envFromTUI = true
			}
		} else if aIdx == 1 {
			_ = tui.OpenFileInEditor(envFilePath)
			profiles, _ := config.LoadProfiles(envFilePath)
			if p, exists := profiles[selectedProfile]; exists {
				envVars = p
			}
		} else {
			os.Exit(1)
		}
	}

	// Tu dong xoa file trung ten tren remote neu co
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

	// 6. Menu lua chon che do dong bo
	modes := []string{
		"Day du lieu tu may len server  (PUSH)",
		"Tai du lieu tu server ve may   (PULL)",
		"Dung lai",
	}
	modeIdx := tui.PromptMenu("CHON CHE DO DONG BO", modes, 0)
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

	// 7. Quan ly Whitelist File (Tu dong tao va mo editor neu chua co)
	whitelistFileName := "xsync.push.ini"
	if syncMode == "pull" {
		whitelistFileName = "xsync.pull.ini"
	}
	whitelistFilePath := filepath.Join(configDir, whitelistFileName)

	if !fileExists(whitelistFilePath) {
		tui.PrintWarn(fmt.Sprintf("Chua tim thay %s tai thu muc hien tai.", whitelistFileName))
		_ = config.EnsureConfigFiles(configDir)
		tui.PrintOk(fmt.Sprintf("Da tu dong tao file mau %s.", whitelistFileName))
		tui.PrintInfo(fmt.Sprintf("Chuan bi mo %s trong trinh soan thao de ban nhap danh sach...", whitelistFileName))
		fmt.Print("  An Enter de mo file...")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		_ = tui.OpenFileInEditor(whitelistFilePath)
	}

	var generatedFilterFile string
	defer func() {
		if generatedFilterFile != "" {
			_ = os.Remove(generatedFilterFile)
		}
	}()

	for {
		data, err := os.ReadFile(whitelistFilePath)
		if err != nil {
			tui.PrintWarn(fmt.Sprintf("Khong the doc file %s. Se dong bo toan bo thu muc.", whitelistFileName))
			break
		}

		lines := strings.Split(string(data), "\n")
		// Xử lý và kiểm tra tính hợp lệ của đường dẫn (Cảnh báo non-fatal nếu đường dẫn local không tồn tại)
		processRes := pathutils.ProcessIncludePathsWithValidation(lines, syncMode, configDir, envVars.RemoteDir)

		for _, w := range processRes.Warnings {
			tui.PrintWarn(w)
		}

		rules := pathutils.BuildIncludeFilter(processRes.ValidPaths)
		if len(rules) > 0 {
			tmpF, err := os.CreateTemp("", "*_rsync_filter")
			if err == nil {
				_, _ = tmpF.WriteString(strings.Join(rules, "\n") + "\n")
				tmpF.Close()
				generatedFilterFile = tmpF.Name()
				tui.PrintOk(fmt.Sprintf("Da ap dung whitelist %s (%d quy tac)", syncMode, len(processRes.ValidPaths)))
			}
			break
		} else {
			tui.PrintWarn(fmt.Sprintf("Whitelist %s khong co muc hop le nao.", syncMode))
			emptyOpts := []string{
				"Tiep tuc dong bo TOAN BO thu muc",
				fmt.Sprintf("Mo lai file %s de nhap danh sach", whitelistFileName),
				"Dung lai",
			}
			eIdx := tui.PromptMenu("WHITELIST TRONG", emptyOpts, 0)
			if eIdx == 0 {
				tui.PrintInfo(fmt.Sprintf("Che do: %s TOAN BO thu muc.", strings.ToUpper(syncMode)))
				break
			} else if eIdx == 1 {
				_ = tui.OpenFileInEditor(whitelistFilePath)
				continue
			} else {
				os.Exit(0)
			}
		}
	}

	tui.PrintHeader(actionTitle)
	tui.PrintKV("Nguon", srcPath)
	tui.PrintKV("Dich", destPath)
	tui.PrintKV("SSH Host", selectedProfile)
	tui.PrintKV("Config dir", configDir)

	// 8. Chay Thu (Dry-Run)
	tui.PrintHeader("DANG CHAY THU (DRY-RUN)")
	dryOk := sync.RunDryRun(syncMode, configDir, selectedProfile, envVars.RemoteDir, generatedFilterFile, deleteEnabled)
	if !dryOk {
		tui.PrintErr("Dry-run that bai. Vui long kiem tra SSH, quyen truy cap va duong dan remote.")
		os.Exit(1)
	}

	// 9. Xac nhan thuc thi
	confirmOpts := []string{
		"Tiep tuc (chay THAT)",
		"Dung lai",
	}
	confIdx := tui.PromptMenu("XAC NHAN CHAY THAT", confirmOpts, 0)
	if confIdx == 1 {
		os.Exit(0)
	}

	// 10. Chay truyen tai song song
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

		if envFromTUI {
			saveItems := []tui.ChecklistItem{
				{Key: "env", Label: "Luu mat khau SSH vua nhap vao xsync.ini", Checked: true},
			}
			selectedSaves := tui.PromptChecklist("LUU MAT KHAU", saveItems)
			for _, s := range selectedSaves {
				if s == "env" {
					_ = config.SaveProfile(envFilePath, selectedProfile, envVars)
					tui.PrintOk(fmt.Sprintf("Da luu mat khau cho profile '%s' vao xsync.ini", selectedProfile))
				}
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
