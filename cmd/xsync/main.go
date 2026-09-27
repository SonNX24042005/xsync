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
	"xsync/internal/i18n"
	"xsync/internal/parallel"
	"xsync/internal/pathutils"
	"xsync/internal/sync"
	"xsync/internal/tui"
)

const Version = "1.0.1"

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
					tui.PrintInfo(i18n.TF("migrated_config", m[0], m[1]))
				}
			}
		}
	}
}

func main() {
	configDir, err := os.Getwd()
	if err != nil {
		tui.PrintErr(fmt.Sprintf("Cannot get current directory: %v", err))
		os.Exit(1)
	}

	envFilePath := filepath.Join(configDir, "xsync.ini")
	if !fileExists(envFilePath) {
		nearest := config.FindConfigNearest("xsync.ini", 8)
		if nearest != "" {
			envFilePath = filepath.Join(nearest, "xsync.ini")
		}
	}

	// Load saved language from environment, local xsync.ini, or global config
	i18n.LoadSavedLanguage(envFilePath)

	// Parse command line arguments and flags
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "-h", "--help", "help":
			fmt.Println(i18n.T("help_usage"))
			os.Exit(0)
		case "-v", "--version", "version":
			fmt.Printf(i18n.T("version_info"), Version)
			os.Exit(0)
		case "--lang":
			if i+1 < len(os.Args) {
				i18n.SetLanguage(os.Args[i+1])
				i++
			}
		case "--set-lang":
			if i+1 < len(os.Args) {
				targetLang := os.Args[i+1]
				if err := i18n.SaveGlobalLanguage(targetLang); err != nil {
					tui.PrintErr(fmt.Sprintf("Failed to save language: %v", err))
					os.Exit(1)
				}
				if i18n.GetLanguage() == i18n.LangEN {
					tui.PrintOk(fmt.Sprintf("Default language saved: %s", targetLang))
				} else {
					tui.PrintOk(fmt.Sprintf("Đã lưu ngôn ngữ mặc định: %s", targetLang))
				}
				os.Exit(0)
			}
		default:
			if strings.HasPrefix(arg, "--lang=") {
				i18n.SetLanguage(strings.TrimPrefix(arg, "--lang="))
			} else if strings.HasPrefix(arg, "--set-lang=") {
				targetLang := strings.TrimPrefix(arg, "--set-lang=")
				_ = i18n.SaveGlobalLanguage(targetLang)
				os.Exit(0)
			}
		}
	}

	// 1. Kiem tra phu thuoc he thong (Fatal Error neu thieu)
	if err := sync.CheckDependencies(); err != nil {
		os.Exit(1)
	}

	migrateOldConfigs(configDir)

	// 2. Tu dong tao xsync.ini neu chua ton tai va mo editor cho nguoi dung nhap
	if !fileExists(envFilePath) {
		tui.PrintWarn(i18n.T("not_found_xsync_ini"))
		_ = config.EnsureConfigFiles(configDir)
		envFilePath = filepath.Join(configDir, "xsync.ini")
		tui.PrintOk(i18n.T("created_xsync_ini"))
		tui.PrintInfo(i18n.T("prompt_open_xsync_ini"))
		fmt.Print(i18n.T("press_enter_to_open_config"))
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
			tui.PrintWarn(i18n.T("no_ssh_host_found"))
			tui.PrintInfo(i18n.T("prompt_open_ssh_config"))
			fmt.Print(i18n.T("press_enter_to_open_ssh"))
			_, _ = bufio.NewReader(os.Stdin).ReadString('\n')

			home, _ := os.UserHomeDir()
			sshConfigPath := filepath.Join(home, ".ssh", "config")
			_ = os.MkdirAll(filepath.Dir(sshConfigPath), 0o700)
			if _, err := os.Stat(sshConfigPath); os.IsNotExist(err) {
				template := `# Example SSH Host in ~/.ssh/config
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
					i18n.T("menu_reopen_ssh_config"),
					i18n.T("menu_stop"),
				}
				cIdx := tui.PromptMenu(i18n.T("menu_no_ssh_host_title"), retryOpts, 0)
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
		options = append(options, i18n.T("menu_open_ssh_config_opt"))
		options = append(options, i18n.T("menu_open_xsync_ini_opt"))
		options = append(options, i18n.T("menu_exit"))

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

		idx := tui.PromptMenu(i18n.T("menu_ssh_profile_title"), options, defaultIdx)
		if idx < len(hosts) {
			selectedProfile = hosts[idx]
			_ = config.SetDefaultProfile(envFilePath, selectedProfile)
			break
		} else if idx == len(hosts) {
			home, _ := os.UserHomeDir()
			tui.PrintInfo(i18n.T("opening_ssh_config"))
			_ = tui.OpenFileInEditor(filepath.Join(home, ".ssh", "config"))
			continue
		} else if idx == len(hosts)+1 {
			tui.PrintInfo(i18n.T("opening_xsync_ini"))
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

		tui.PrintWarn(i18n.TF("warn_host_no_remote_dir", selectedProfile))
		fixOpts := []string{
			i18n.T("menu_fill_remote_dir"),
			i18n.T("menu_reselect_host"),
			i18n.T("menu_stop"),
		}
		fIdx := tui.PromptMenu(i18n.T("menu_missing_remote_dir_title"), fixOpts, 0)
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

	// 5. Thiet lap va kiem tra ket noi SSH Master Socket
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
		tui.PrintInfo(i18n.TF("ssh_connecting", selectedProfile))
		ok, errSSH := sync.SetupSSHMaster(selectedProfile, envVars.SSHPassword, controlPath)
		if ok {
			tui.PrintOk(i18n.T("ssh_connected"))
			break
		}

		tui.PrintErr(i18n.TF("ssh_connect_failed", selectedProfile, errSSH))
		authOpts := []string{
			i18n.T("menu_enter_pwd_manually"),
			i18n.T("menu_fix_xsync_ini"),
			i18n.T("menu_stop"),
		}
		aIdx := tui.PromptMenu(i18n.T("menu_ssh_auth_error_title"), authOpts, 0)
		if aIdx == 0 {
			fmt.Printf(i18n.T("prompt_enter_pwd"), selectedProfile)
			pwdBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
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
		i18n.T("menu_mode_push"),
		i18n.T("menu_mode_pull"),
		i18n.T("menu_stop"),
	}
	modeIdx := tui.PromptMenu(i18n.T("menu_sync_mode_title"), modes, 0)
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
		actionTitle = i18n.T("title_push_prepare")
		actionStart = i18n.T("title_push_start")
		actionDone = i18n.T("title_push_done")
		actionFail = i18n.T("title_push_fail")
		srcPath = configDir + "/"
		destPath = fmt.Sprintf("%s:%s/", selectedProfile, strings.TrimRight(envVars.RemoteDir, "/"))

		deleteOpts := []string{
			i18n.T("menu_delete_yes"),
			i18n.T("menu_delete_no"),
		}
		delIdx := tui.PromptMenu(i18n.T("menu_delete_flag_title"), deleteOpts, 1)
		deleteEnabled = (delIdx == 0)
	} else {
		actionTitle = i18n.T("title_pull_prepare")
		actionStart = i18n.T("title_pull_start")
		actionDone = i18n.T("title_pull_done")
		actionFail = i18n.T("title_pull_fail")
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
		tui.PrintWarn(i18n.TF("warn_whitelist_not_found", whitelistFileName))
		_ = config.EnsureConfigFiles(configDir)
		tui.PrintOk(i18n.TF("ok_created_whitelist", whitelistFileName))
		tui.PrintInfo(i18n.TF("info_open_whitelist", whitelistFileName))
		fmt.Print(i18n.T("press_enter_to_open_file"))
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
			tui.PrintWarn(i18n.TF("cannot_read_whitelist", whitelistFileName))
			break
		}

		lines := strings.Split(string(data), "\n")
		// Xử lý và kiểm tra tính hợp lệ của đường dẫn
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
				tui.PrintOk(i18n.TF("applied_whitelist", syncMode, len(processRes.ValidPaths)))
			}
			break
		} else {
			tui.PrintWarn(i18n.TF("warn_empty_whitelist", syncMode))
			emptyOpts := []string{
				i18n.T("menu_sync_all_folder"),
				i18n.TF("menu_reopen_whitelist", whitelistFileName),
				i18n.T("menu_stop"),
			}
			eIdx := tui.PromptMenu(i18n.T("menu_empty_whitelist_title"), emptyOpts, 0)
			if eIdx == 0 {
				tui.PrintInfo(i18n.TF("sync_all_notice", strings.ToUpper(syncMode)))
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
	tui.PrintKV(i18n.T("kv_source"), srcPath)
	tui.PrintKV(i18n.T("kv_dest"), destPath)
	tui.PrintKV(i18n.T("kv_ssh_host"), selectedProfile)
	tui.PrintKV(i18n.T("kv_config_dir"), configDir)

	// 8. Chay Thu (Dry-Run)
	tui.PrintHeader(i18n.T("header_dry_run"))
	dryOk := sync.RunDryRun(syncMode, configDir, selectedProfile, envVars.RemoteDir, generatedFilterFile, deleteEnabled)
	if !dryOk {
		tui.PrintErr(i18n.T("dry_run_failed"))
		os.Exit(1)
	}

	// 9. Xac nhan thuc thi
	confirmOpts := []string{
		i18n.T("menu_confirm_continue"),
		i18n.T("menu_stop"),
	}
	confIdx := tui.PromptMenu(i18n.T("menu_confirm_run_title"), confirmOpts, 0)
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
				{Key: "env", Label: i18n.T("save_password_label"), Checked: true},
			}
			selectedSaves := tui.PromptChecklist(i18n.T("menu_save_password_title"), saveItems)
			for _, s := range selectedSaves {
				if s == "env" {
					_ = config.SaveProfile(envFilePath, selectedProfile, envVars)
					tui.PrintOk(i18n.TF("saved_password_success", selectedProfile))
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
