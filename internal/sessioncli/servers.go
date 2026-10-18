package sessioncli

import (
	"context"
	"fmt"
	"strings"
	"xsync/internal/config"
	"xsync/internal/serverstore"
	"xsync/internal/tui"
)

func serverItems() ([]tui.MenuItem, error) {
	settings, err := serverstore.Read()
	if err != nil {
		return nil, err
	}
	credentials := serverstore.EffectiveCredentials(settings)
	var items []tui.MenuItem
	for _, host := range config.ParseSSHHosts() {
		label := host
		if !settings[host].KeyVerifiedAt.IsZero() {
			label += " · " + tr("Đã dùng key", "Key verified")
		}
		if credentials[host].Password != "" {
			label += " · " + tr("Đã lưu mật khẩu", "Password saved")
		}
		items = append(items, tui.MenuItem{ID: host, Label: label})
	}
	return items, nil
}

func serverAction(host, action string) (string, error) {
	if !serverstore.ConfiguredHost(host) {
		return "", fmt.Errorf("%s", tr("Chọn máy chủ trong SSH config.", "Choose a host from SSH config."))
	}
	switch action {
	case "password":
		password, err := readPassword()
		if err != nil {
			return "", err
		}
		err = serverstore.Update(host, func(s *serverstore.Setting) { s.Password = &password })
		return tr("Đã lưu mật khẩu cho kết nối mới.", "Password saved for new connections."), err
	case "disable-password":
		empty := ""
		err := serverstore.Update(host, func(s *serverstore.Setting) { s.Password = &empty })
		return tr("Đã ngừng dùng mật khẩu cho kết nối mới. Phiên đã lưu giữ xác thực riêng.", "Saved passwords disabled for new connections. Existing sessions retain their authentication."), err
	case "check-key":
		err := interruptible(func(ctx context.Context) error { return serverstore.CheckKey(ctx, host) })
		return tr("SSH key đã xác thực thành công.", "SSH key authentication succeeded."), err
	case "check":
		var result serverstore.ConnectionCheck
		err := interruptible(func(ctx context.Context) error {
			var err error
			result, err = serverstore.CheckConnection(ctx, host)
			return err
		})
		if err != nil {
			return "", err
		}
		if !result.SSHOK {
			return "", fmt.Errorf("%s", result.Error)
		}
		text := tr("SSH: kết nối thành công bằng ", "SSH: connected using ") + result.Auth
		if result.SyncReady {
			text += "\nrsync: " + tr("Sẵn sàng đồng bộ", "Ready to sync")
		} else {
			text += "\nrsync: " + result.SyncError
		}
		if result.Warning != "" {
			text += "\n" + result.Warning
		}
		return text, nil
	}
	return "", fmt.Errorf("unknown server action: %s", action)
}

func Servers() error {
	for {
		items, err := serverItems()
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return tui.ShowText(tr("Máy chủ", "Servers"), tr("Thêm Host vào ~/.ssh/config để kết nối.", "Add a Host to ~/.ssh/config to connect."))
		}
		chosen, err := tui.ChooseItems(tr("Máy chủ", "Servers"), items, serverItems)
		if err != nil {
			return err
		}
		actions := []tui.MenuItem{{ID: "check", Label: tr("Kiểm tra kết nối", "Check connection")}, {ID: "check-key", Label: tr("Kiểm tra key", "Check key")}, {ID: "password", Label: tr("Lưu hoặc thay mật khẩu", "Save or replace password")}, {ID: "disable-password", Label: tr("Ngừng dùng mật khẩu", "Disable saved password")}, {ID: "back", Label: tr("Quay lại", "Back")}}
		action, err := tui.ChooseItems(chosen.ID, actions, nil)
		if err == tui.ErrCancelled {
			continue
		}
		if err != nil {
			return err
		}
		if action.ID == "back" {
			continue
		}
		out, err := serverAction(chosen.ID, action.ID)
		if err != nil {
			out = err.Error()
		}
		if err = tui.ShowText(chosen.ID, strings.TrimSpace(out)); err != nil {
			return err
		}
	}
}
