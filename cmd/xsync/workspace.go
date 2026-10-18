package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"xsync/internal/i18n"
	"xsync/internal/session"
	"xsync/internal/sessioncli"
	"xsync/internal/tui"
	"xsync/internal/webui"
)

func workspace() error {
	for {
		chosen, err := tui.ChooseItems("xsync", []tui.MenuItem{
			{ID: "sessions", Label: tui.Text("Phiên đồng bộ", "Sync sessions")},
			{ID: "new", Label: tui.Text("Tạo phiên mới", "Create session")},
			{ID: "servers", Label: tui.Text("Máy chủ", "Servers")},
			{ID: "web", Label: tui.Text("Mở giao diện web", "Open web interface")},
			{ID: "settings", Label: tui.Text("Cài đặt", "Settings")},
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
			err = webMenu()
		case "settings":
			err = settingsMenu()
		case "exit":
			return nil
		}
		if err != nil && !errors.Is(err, tui.ErrCancelled) {
			if err = tui.ShowText(tui.Text("Thông báo", "Notice"), err.Error()); err != nil {
				return err
			}
		}
	}
}

func storageText() (string, error) {
	root, err := session.Root()
	if err != nil {
		return "", err
	}
	return tui.Text("Phiên đồng bộ", "Sync sessions") + ":\n" + root + "\n\n" +
		tui.Text("Mật khẩu và lịch sử key", "Passwords and key history") + ":\n" + filepath.Join(filepath.Dir(root), "servers.json") + "\n\n" +
		tui.Text("Cài đặt", "Settings") + ":\n" + i18n.GetGlobalConfigPath(), nil
}

func settingsMenu() error {
	for {
		language := "Tiếng Việt"
		if i18n.GetLanguage() == i18n.LangEN {
			language = "English"
		}
		chosen, err := tui.ChooseItems(tui.Text("Cài đặt", "Settings"), []tui.MenuItem{
			{ID: "language", Label: tui.Text("Ngôn ngữ", "Language") + " · " + language},
			{ID: "storage", Label: tui.Text("Nơi lưu dữ liệu", "Storage locations")},
			{ID: "about", Label: tui.Text("Thông tin xsync", "About xsync")},
			{ID: "back", Label: tui.Text("Quay lại", "Back")},
		}, nil)
		if err != nil {
			return err
		}
		switch chosen.ID {
		case "language":
			choice, err := tui.ChooseItems(tui.Text("Ngôn ngữ", "Language"), []tui.MenuItem{
				{ID: i18n.LangVI, Label: "Tiếng Việt"}, {ID: i18n.LangEN, Label: "English"},
			}, nil)
			if errors.Is(err, tui.ErrCancelled) {
				continue
			}
			if err != nil {
				return err
			}
			if err = i18n.SaveGlobalLanguage(choice.ID); err != nil {
				return err
			}
		case "storage":
			text, err := storageText()
			if err != nil {
				return err
			}
			if err = tui.ShowText(tui.Text("Nơi lưu dữ liệu", "Storage locations"), text); err != nil {
				return err
			}
		case "about":
			if err = tui.ShowText(versionLabel(), help()); err != nil {
				return err
			}
		case "back":
			return nil
		}
	}
}

func parseWebPort(value string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 0 || port > 65535 {
		return 0, fmt.Errorf("%s", tui.Text("Nhập cổng từ 0 đến 65535; 0 để tự chọn cổng trống.", "Enter a port from 0 to 65535; 0 chooses an available port."))
	}
	return port, nil
}

func webMenu() error {
	port, openBrowser := 8787, true
	for {
		portLabel := strconv.Itoa(port)
		if port == 0 {
			portLabel = tui.Text("Tự chọn cổng trống", "Automatically choose an available port")
		}
		browserLabel := tui.Text("Có", "Yes")
		if !openBrowser {
			browserLabel = tui.Text("Không", "No")
		}
		chosen, err := tui.ChooseItems(tui.Text("Giao diện web", "Web interface"), []tui.MenuItem{
			{ID: "open", Label: tui.Text("Khởi chạy giao diện web", "Start web interface")},
			{ID: "port", Label: tui.Text("Cổng", "Port") + " · " + portLabel},
			{ID: "browser", Label: tui.Text("Tự mở trình duyệt", "Open browser automatically") + " · " + browserLabel},
			{ID: "back", Label: tui.Text("Quay lại", "Back")},
		}, nil)
		if err != nil {
			return err
		}
		switch chosen.ID {
		case "open":
			if err = webui.Run(port, openBrowser); err != nil {
				if err = tui.ShowText(tui.Text("Không mở được giao diện web", "Cannot start web interface"), err.Error()); err != nil {
					return err
				}
			}
		case "port":
			value, err := tui.InputText(tui.Text("Cổng giao diện web (0 = tự chọn)", "Web interface port (0 = automatic)"), strconv.Itoa(port), func(value string) error {
				_, err := parseWebPort(value)
				return err
			})
			if errors.Is(err, tui.ErrCancelled) {
				continue
			}
			if err != nil {
				return err
			}
			port, _ = parseWebPort(value)
		case "browser":
			openBrowser = !openBrowser
		case "back":
			return nil
		}
	}
}
