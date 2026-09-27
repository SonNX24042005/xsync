package i18n

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	currentLang = "vi"
	mu          sync.RWMutex
)

// Supported languages
const (
	LangVI = "vi"
	LangEN = "en"
)

// SetLanguage sets the active language ("vi" or "en").
func SetLanguage(lang string) {
	mu.Lock()
	defer mu.Unlock()
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "en" || lang == "english" {
		currentLang = LangEN
	} else {
		currentLang = LangVI
	}
}

// GetLanguage returns the active language code.
func GetLanguage() string {
	mu.RLock()
	defer mu.RUnlock()
	return currentLang
}

// GetGlobalConfigPath returns ~/.config/xsync/config.ini
func GetGlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "xsync", "config.ini")
}

// LoadSavedLanguage reads the configured language from:
// 1. Environment variable XSYNC_LANG
// 2. Local xsync.ini under [settings] language = ...
// 3. Global ~/.config/xsync/config.ini under [settings] language = ...
// Fallbacks to "vi".
func LoadSavedLanguage(localIniPath string) string {
	if env := os.Getenv("XSYNC_LANG"); env != "" {
		SetLanguage(env)
		return GetLanguage()
	}

	// Check local ini if provided
	if localIniPath != "" {
		if lang := readLanguageFromIni(localIniPath); lang != "" {
			SetLanguage(lang)
			return GetLanguage()
		}
	}

	// Check global config
	globalPath := GetGlobalConfigPath()
	if globalPath != "" {
		if lang := readLanguageFromIni(globalPath); lang != "" {
			SetLanguage(lang)
			return GetLanguage()
		}
	}

	SetLanguage(LangVI)
	return GetLanguage()
}

// SaveGlobalLanguage writes the selected language to ~/.config/xsync/config.ini
func SaveGlobalLanguage(lang string) error {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang != LangEN && lang != LangVI {
		lang = LangVI
	}

	globalPath := GetGlobalConfigPath()
	if globalPath == "" {
		return fmt.Errorf("unable to determine user home directory")
	}

	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		return err
	}

	content := fmt.Sprintf("[settings]\nlanguage = %s\n", lang)
	if err := os.WriteFile(globalPath, []byte(content), 0o644); err != nil {
		return err
	}

	SetLanguage(lang)
	return nil
}

func readLanguageFromIni(iniPath string) string {
	file, err := os.Open(iniPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	currentSection := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		if currentSection == "settings" {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.ToLower(strings.TrimSpace(parts[0]))
				val := strings.ToLower(strings.TrimSpace(parts[1]))
				if key == "language" || key == "lang" {
					if val == "en" || val == "english" {
						return LangEN
					}
					if val == "vi" || val == "vietnamese" {
						return LangVI
					}
				}
			}
		}
	}
	return ""
}

// T returns the translated string for the given key.
func T(key string) string {
	lang := GetLanguage()
	if dict, exists := translations[lang]; exists {
		if val, ok := dict[key]; ok {
			return val
		}
	}
	// Fallback to Vietnamese
	if dict, exists := translations[LangVI]; exists {
		if val, ok := dict[key]; ok {
			return val
		}
	}
	return key
}

// TF formats the translated string with args.
func TF(key string, args ...any) string {
	return fmt.Sprintf(T(key), args...)
}

var translations = map[string]map[string]string{
	LangVI: {
		"help_usage": `xsync - Cong cu dong bo du lieu song song hai chieu qua SSH va rsync

Cach su dung:
  xsync [flags]

Cac co (flags):
  -h, --help           Hien thi thong tin tro giup nay
  -v, --version        Hien thi thong tin phien ban
      --lang <en|vi>   Chi dinh ngon ngu hien thi (Tieng Anh hoac Tieng Viet)
      --set-lang <en|vi> Luu ngon ngu mac dinh vao ~/.config/xsync/config.ini

Tinh nang noi bat:
  - SSH Multiplexing: Tai su dung phien ket noi SSH Master Socket da xac thuc
  - Truyen tai song song: Phan chia du lieu thanh nhieu luong rsync dong thoi
  - Whitelist: Loc chinh xac thu muc va tap tin can day (push) hoac tai (pull)
  - Dry-Run: Chay thu an toan, xem truoc danh sach thay doi truoc khi thuc hien
  - Live Dashboard: Theo doi tien do, toc do truyen tai tung luong theo thoi gian thuc

Cac tep cau hinh:
  Toan cuc: ~/.config/xsync/config.ini  (luu ngon ngu va cai dat mac dinh)
  Cuc bo:   xsync.ini                  (luu SSH host va thu muc remote)
            xsync.push.ini             (danh sach trang day len server)
            xsync.pull.ini             (danh sach trang tai ve tu server)

Tai lieu va ma nguon:
  https://github.com/SonNX24042005/xsync`,

		"version_info":                 "xsync phien ban %s (biên dịch bằng Go)\n",
		"migrated_config":              "Da tu dong di tru %s sang %s",
		"cannot_get_cwd":               "Khong the lay thu muc hien tai: %v",
		"not_found_xsync_ini":          "Chua tim thay file xsync.ini tai thu muc hien tai.",
		"created_xsync_ini":            "Da tu dong tao file mau xsync.ini tai thu muc hien tai.",
		"prompt_open_xsync_ini":        "Chuan bi mo xsync.ini trong trinh soan thao de ban khai bao SSH Host va REMOTE_DIR...",
		"press_enter_to_open_config":   "  An Enter de mo file cau hinh...",
		"no_ssh_host_found":            "Khong tim thay SSH Host nao duoc cau hinh trong ~/.ssh/config.",
		"prompt_open_ssh_config":       "Chuan bi mo ~/.ssh/config de ban tu them Host...",
		"press_enter_to_open_ssh":      "  An Enter de mo file cau hinh SSH...",
		"menu_no_ssh_host_title":       "CHUA CO SSH HOST",
		"menu_reopen_ssh_config":       "Mo lai file ~/.ssh/config de them Host",
		"menu_stop":                    "Dung lai",
		"menu_ssh_profile_title":       "CHON SSH PROFILE (tu ~/.ssh/config)",
		"menu_open_ssh_config_opt":     "Mo file ~/.ssh/config de tu them/sua SSH Host...",
		"menu_open_xsync_ini_opt":      "Mo file xsync.ini de tu sua thu muc REMOTE_DIR/REMOTE_PULL_DIR...",
		"menu_exit":                    "Thoat",
		"opening_ssh_config":           "Dang mo file ~/.ssh/config trong trinh soan thao...",
		"opening_xsync_ini":            "Dang mo file xsync.ini trong trinh soan thao...",
		"warn_host_no_remote_dir":      "Host '%s' chua duoc khai bao thu muc REMOTE_DIR hop le trong xsync.ini.",
		"menu_missing_remote_dir_title": "CAU HINH THIEU REMOTE_DIR",
		"menu_fill_remote_dir":         "Mo file xsync.ini trong trinh soan thao de dien REMOTE_DIR",
		"menu_reselect_host":           "Chon lai SSH Host khac",
		"ssh_connecting":               "Dang thiet lap ket noi SSH Master cho host '%s'...",
		"ssh_connected":                "Da thiet lap SSH Master thanh cong!",
		"ssh_connect_failed":           "Khong the ket noi SSH toi host '%s': %s",
		"menu_ssh_auth_error_title":    "LOI XAC THUC / KET NOI SSH",
		"menu_enter_pwd_manually":      "Nhap mat khau SSH bang tay de thu lai",
		"menu_fix_xsync_ini":           "Mo file xsync.ini de kiem tra va sua lai cau hinh",
		"prompt_enter_pwd":             "  Nhap mat khau SSH cho '%s': ",
		"menu_sync_mode_title":         "CHON CHE DO DONG BO",
		"menu_mode_push":               "Day du lieu tu may len server  (PUSH)",
		"menu_mode_pull":               "Tai du lieu tu server ve may   (PULL)",
		"menu_delete_flag_title":       "CO DUNG FLAG --delete?",
		"menu_delete_yes":              "Co  (Xoa file tren Server neu khong co trong danh sach push)",
		"menu_delete_no":               "Khong (Giu lai file tren Server)",
		"title_push_prepare":           "CHUAN BI DAY DU LIEU LEN SERVER",
		"title_push_start":             "BAT DAU DAY DU LIEU",
		"title_push_done":              "HOAN TAT DAY DU LIEU!",
		"title_push_fail":              "Day du lieu that bai. Kiem tra ket noi SSH va duong dan.",
		"title_pull_prepare":           "CHUAN BI TAI DU LIEU TU SERVER VE MAY",
		"title_pull_start":             "BAT DAU TAI DU LIEU",
		"title_pull_done":              "HOAN TAT TAI DU LIEU!",
		"title_pull_fail":              "Tai du lieu that bai. Kiem tra ket noi SSH va duong dan.",
		"warn_whitelist_not_found":     "Chua tim thay %s tai thu muc hien tai.",
		"ok_created_whitelist":         "Da tu dong tao file mau %s.",
		"info_open_whitelist":          "Chuan bi mo %s trong trinh soan thao de ban nhap danh sach...",
		"press_enter_to_open_file":     "  An Enter de mo file...",
		"cannot_read_whitelist":        "Khong the doc file %s. Se dong bo toan bo thu muc.",
		"applied_whitelist":            "Da ap dung whitelist %s (%d quy tac)",
		"warn_empty_whitelist":         "Whitelist %s khong co muc hop le nao.",
		"menu_empty_whitelist_title":   "WHITELIST TRONG",
		"menu_sync_all_folder":         "Tiep tuc dong bo TOAN BO thu muc",
		"menu_reopen_whitelist":        "Mo lai file %s de nhap danh sach",
		"sync_all_notice":              "Che do: %s TOAN BO thu muc.",
		"kv_source":                    "Nguon",
		"kv_dest":                      "Dich",
		"kv_ssh_host":                  "SSH Host",
		"kv_config_dir":                "Config dir",
		"header_dry_run":               "DANG CHAY THU (DRY-RUN)",
		"dry_run_failed":               "Dry-run that bai. Vui long kiem tra SSH, quyen truy cap va duong dan remote.",
		"menu_confirm_run_title":       "XAC NHAN CHAY THAT",
		"menu_confirm_continue":        "Tiep tuc (chay THAT)",
		"menu_save_password_title":     "LUU MAT KHAU",
		"save_password_label":          "Luu mat khau SSH vua nhap vao xsync.ini",
		"saved_password_success":       "Da luu mat khau cho profile '%s' vao xsync.ini",
	},

	LangEN: {
		"help_usage": `xsync - High-performance parallel bidirectional file sync via SSH and rsync

Usage:
  xsync [flags]

Flags:
  -h, --help           Show this help message
  -v, --version        Show version information
      --lang <en|vi>   Override display language (English or Vietnamese)
      --set-lang <en|vi> Save default language to ~/.config/xsync/config.ini

Key Features:
  - SSH Multiplexing: Reuses a single authenticated SSH Master Socket
  - Parallel Sync: Divides files across multiple concurrent rsync processes
  - Whitelist: Precisely filters files/directories to push or pull
  - Dry-Run: Safe preview showing change list before real modifications
  - Live Dashboard: Real-time progress bar and speed tracking per worker

Configuration Files:
  Global:  ~/.config/xsync/config.ini  (stores default language and settings)
  Local:   xsync.ini                  (stores remote SSH host and directories)
           xsync.push.ini             (whitelist of files/directories to push)
           xsync.pull.ini             (whitelist of files/directories to pull)

Documentation and source code:
  https://github.com/SonNX24042005/xsync`,

		"version_info":                 "xsync version %s (compiled with Go)\n",
		"migrated_config":              "Automatically migrated %s to %s",
		"cannot_get_cwd":               "Unable to get current directory: %v",
		"not_found_xsync_ini":          "xsync.ini not found in current directory.",
		"created_xsync_ini":            "Automatically created template xsync.ini in current directory.",
		"prompt_open_xsync_ini":        "Opening xsync.ini in editor for you to declare SSH Host and REMOTE_DIR...",
		"press_enter_to_open_config":   "  Press Enter to open configuration file...",
		"no_ssh_host_found":            "No SSH Host configured in ~/.ssh/config.",
		"prompt_open_ssh_config":       "Opening ~/.ssh/config in editor for you to add a Host...",
		"press_enter_to_open_ssh":      "  Press Enter to open SSH config file...",
		"menu_no_ssh_host_title":       "NO SSH HOST CONFIGURED",
		"menu_reopen_ssh_config":       "Reopen ~/.ssh/config to add Host",
		"menu_stop":                    "Cancel / Exit",
		"menu_ssh_profile_title":       "SELECT SSH PROFILE (from ~/.ssh/config)",
		"menu_open_ssh_config_opt":     "Open ~/.ssh/config to add/edit SSH Host...",
		"menu_open_xsync_ini_opt":      "Open xsync.ini to edit REMOTE_DIR...",
		"menu_exit":                    "Exit",
		"opening_ssh_config":           "Opening ~/.ssh/config in text editor...",
		"opening_xsync_ini":            "Opening xsync.ini in text editor...",
		"warn_host_no_remote_dir":      "Host '%s' has no valid REMOTE_DIR defined in xsync.ini.",
		"menu_missing_remote_dir_title": "CONFIGURATION MISSING REMOTE_DIR",
		"menu_fill_remote_dir":         "Open xsync.ini in editor to enter REMOTE_DIR",
		"menu_reselect_host":           "Select a different SSH Host",
		"ssh_connecting":               "Establishing SSH Master connection for host '%s'...",
		"ssh_connected":                "SSH Master connection established successfully!",
		"ssh_connect_failed":           "Failed to connect to SSH host '%s': %s",
		"menu_ssh_auth_error_title":    "SSH AUTHENTICATION / CONNECTION ERROR",
		"menu_enter_pwd_manually":      "Enter SSH password manually to retry",
		"menu_fix_xsync_ini":           "Open xsync.ini to inspect and fix configuration",
		"prompt_enter_pwd":             "  Enter SSH password for '%s': ",
		"menu_sync_mode_title":         "SELECT SYNC MODE",
		"menu_mode_push":               "Push local data to remote server (PUSH)",
		"menu_mode_pull":               "Pull remote data to local machine (PULL)",
		"menu_delete_flag_title":       "USE --delete FLAG?",
		"menu_delete_yes":              "Yes (Delete files on server if not in push list)",
		"menu_delete_no":               "No  (Keep existing files on server)",
		"title_push_prepare":           "PREPARING TO PUSH DATA TO SERVER",
		"title_push_start":             "STARTING DATA PUSH",
		"title_push_done":              "DATA PUSH COMPLETED!",
		"title_push_fail":              "Data push failed. Check SSH connection and remote path.",
		"title_pull_prepare":           "PREPARING TO PULL DATA FROM SERVER",
		"title_pull_start":             "STARTING DATA PULL",
		"title_pull_done":              "DATA PULL COMPLETED!",
		"title_pull_fail":              "Data pull failed. Check SSH connection and remote path.",
		"warn_whitelist_not_found":     "%s not found in current directory.",
		"ok_created_whitelist":         "Automatically created template %s.",
		"info_open_whitelist":          "Opening %s in editor for you to specify paths...",
		"press_enter_to_open_file":     "  Press Enter to open file...",
		"cannot_read_whitelist":        "Unable to read %s. Will sync entire directory.",
		"applied_whitelist":            "Applied %s whitelist (%d rules)",
		"warn_empty_whitelist":         "Whitelist %s has no valid entries.",
		"menu_empty_whitelist_title":   "WHITELIST EMPTY",
		"menu_sync_all_folder":         "Continue and sync ENTIRE directory",
		"menu_reopen_whitelist":        "Reopen %s to enter file list",
		"sync_all_notice":              "Mode: %s ENTIRE directory.",
		"kv_source":                    "Source",
		"kv_dest":                      "Destination",
		"kv_ssh_host":                  "SSH Host",
		"kv_config_dir":                "Config dir",
		"header_dry_run":               "RUNNING DRY-RUN",
		"dry_run_failed":               "Dry-run failed. Please check SSH, permissions and remote directory.",
		"menu_confirm_run_title":       "CONFIRM EXECUTION",
		"menu_confirm_continue":        "Continue (execute REAL sync)",
		"menu_save_password_title":     "SAVE PASSWORD",
		"save_password_label":          "Save entered SSH password into xsync.ini",
		"saved_password_success":       "Saved password for profile '%s' into xsync.ini",
	},
}
