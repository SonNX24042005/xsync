package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
	"xsync/internal/i18n"
	"xsync/internal/migration"
	"xsync/internal/session"
	"xsync/internal/sessioncli"
	"xsync/internal/tui"
	"xsync/internal/webui"
)

const Version = "1.0.1"

func main() { os.Exit(run(os.Args[1:])) }

func help() string {
	return tui.Text(`xsync · Đồng bộ theo phiên

  xsync                       Mở menu tương tác
  xsync ui                    Mở giao diện web
  xsync session               Quản lý phiên trong terminal
  xsync session new           Tạo phiên
  xsync session --help         Các lệnh quản lý phiên
  xsync server                Quản lý máy chủ, mật khẩu và SSH key
  xsync server --help          Các lệnh quản lý máy chủ
  xsync config-path           Xem nơi lưu dữ liệu
  xsync --lang vi|en           Chọn ngôn ngữ cho lần chạy này
  xsync --set-lang vi|en       Lưu ngôn ngữ mặc định
  xsync --version             Xem phiên bản

Cấu hình, dữ liệu được chọn và xác thực được lưu tập trung theo phiên.
Không cần xsync.ini, xsync.push.ini hoặc xsync.pull.ini.`, `xsync · Session-based synchronization

  xsync                       Open the interactive menu
  xsync ui                    Open the web interface
  xsync session               Manage sessions in the terminal
  xsync session new           Create a session
  xsync session --help         Session commands
  xsync server                Manage hosts, passwords and SSH keys
  xsync server --help          Server commands
  xsync config-path           Show storage locations
  xsync --lang vi|en           Set language for this invocation
  xsync --set-lang vi|en       Save the default language
  xsync --version             Show version

Configuration, selections and authentication are stored centrally per session.
No xsync.ini, xsync.push.ini or xsync.pull.ini is needed.`)
}

func run(args []string) int {
	legacyPreferences := ""
	if dir, err := os.Getwd(); err == nil {
		legacyPreferences = filepath.Join(dir, "xsync.ini")
	}
	i18n.LoadSavedLanguage(legacyPreferences)
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--lang" || arg == "--set-lang":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Missing language: vi|en")
				return 1
			}
			i++
			if arg == "--set-lang" {
				if err := i18n.SaveGlobalLanguage(args[i]); err != nil {
					fmt.Fprintln(os.Stderr, err)
					return 1
				}
				return 0
			}
			i18n.SetLanguage(args[i])
		case strings.HasPrefix(arg, "--lang="):
			i18n.SetLanguage(strings.TrimPrefix(arg, "--lang="))
		case strings.HasPrefix(arg, "--set-lang="):
			if err := i18n.SaveGlobalLanguage(strings.TrimPrefix(arg, "--set-lang=")); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			return 0
		default:
			rest = append(rest, arg)
		}
	}
	if len(rest) == 1 {
		switch rest[0] {
		case "help", "--help", "-h":
			fmt.Println(help())
			return 0
		case "version", "--version", "-v":
			fmt.Printf("xsync %s\n", Version)
			return 0
		case "config-path":
			root, err := session.Root()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			fmt.Println(root)
			fmt.Println(strings.TrimSuffix(root, "sessions") + "servers.json")
			fmt.Println(i18n.GetGlobalConfigPath())
			return 0
		}
	}
	if len(rest) > 0 && rest[0] != "session" && rest[0] != "server" && rest[0] != "ui" {
		fmt.Fprintln(os.Stderr, help())
		return 1
	}
	// Background workers never inspect a project's configuration files.
	if !(len(rest) >= 2 && rest[0] == "session" && rest[1] == "run") {
		dir, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		backup, err := migration.Legacy(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if backup != "" {
			fmt.Fprintln(os.Stderr, tui.Text("Đã chuyển cấu hình cũ; bản sao lưu: ", "Legacy configuration migrated; backup: ")+backup)
		}
	}
	if handled, code := webui.Dispatch(rest); handled {
		return code
	}
	if handled, code := sessioncli.Dispatch(rest); handled {
		return code
	}
	if len(rest) > 0 {
		fmt.Fprintln(os.Stderr, help())
		return 1
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Println(help())
		return 0
	}
	if err := workspace(); err != nil && !errors.Is(err, tui.ErrCancelled) {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func workspace() error {
	for {
		chosen, err := tui.ChooseItems("xsync", []tui.MenuItem{
			{ID: "sessions", Label: tui.Text("Phiên đồng bộ", "Sync sessions")},
			{ID: "new", Label: tui.Text("Tạo phiên mới", "Create session")},
			{ID: "servers", Label: tui.Text("Máy chủ", "Servers")},
			{ID: "web", Label: tui.Text("Mở giao diện web", "Open web interface")},
			{ID: "exit", Label: tui.Text("Thoát", "Exit")},
		}, nil)
		if err != nil {
			return err
		}
		switch chosen.ID {
		case "sessions":
			err = sessioncli.Menu()
		case "new":
			err = sessioncli.New()
		case "servers":
			err = sessioncli.Servers()
		case "web":
			_, code := webui.Dispatch([]string{"ui"})
			if code != 0 {
				err = fmt.Errorf("web interface exited with code %d", code)
			}
		case "exit":
			return nil
		}
		if err != nil && !errors.Is(err, tui.ErrCancelled) {
			tui.PrintErr(err.Error())
		}
	}
}
