// Package sessioncli connects the session engine to the interactive CLI.
package sessioncli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"

	"golang.org/x/term"
	"xsync/internal/config"
	"xsync/internal/fsbrowse"
	"xsync/internal/i18n"
	"xsync/internal/serverstore"
	"xsync/internal/session"
	"xsync/internal/tui"
)

var tr = tui.Text

// Dispatch handles session and server commands without project configuration files.
func Dispatch(args []string) (bool, int) {
	var remaining []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--lang" && i+1 < len(args) {
			i++
			i18n.SetLanguage(args[i])
			continue
		}
		if strings.HasPrefix(args[i], "--lang=") {
			i18n.SetLanguage(strings.TrimPrefix(args[i], "--lang="))
			continue
		}
		remaining = append(remaining, args[i])
	}
	if len(remaining) > 0 && remaining[0] == "server" {
		err := serverCommand(remaining[1:])
		if err == nil || errors.Is(err, tui.ErrCancelled) {
			return true, 0
		}
		fmt.Fprintln(os.Stderr, err)
		return true, 1
	}
	if len(remaining) == 0 || remaining[0] != "session" {
		return false, 0
	}
	if err := command(remaining[1:]); err != nil {
		if errors.Is(err, tui.ErrCancelled) {
			return true, 0
		}
		fmt.Fprintln(os.Stderr, err)
		return true, 1
	}
	return true, 0
}

func Help() string {
	return tr(`Phiên đồng bộ thư mục (local A → remote B/A)

  xsync session new           Chọn server, xác thực, thư mục và dữ liệu
  xsync session list          Xem các phiên đã lưu
  xsync session status <id>   Xem trạng thái và lỗi gần nhất
  xsync session start <id>    Chạy nền và bật tự khởi động
  xsync session background <id> Chạy nền, không tự khởi động
  xsync session stop <id>     Dừng dịch vụ và tắt tự khởi động
  xsync session run <id>      Chạy trong terminal, Ctrl+C để dừng
  xsync session preview <id>  Xem trước thay đổi, không truyền/xóa
  xsync session select <id>   Chọn lại dữ liệu khi phiên đã dừng
  xsync session logs <id>     Xem log gần nhất

Phiên phản chiếu cả thao tác xóa trong phần dữ liệu đã chọn.
Linux: tự chạy khi boot qua systemd user + linger.
macOS/Windows: tự chạy khi đăng nhập tài khoản.`, `Folder mirror sessions (local A → remote B/A)

  xsync session new           Choose server, authentication, folders and data
  xsync session list          List saved sessions
  xsync session status <id>   Show status and the latest error
  xsync session start <id>    Run in background and enable startup
  xsync session background <id> Run in background without startup
  xsync session stop <id>     Stop the service and disable startup
  xsync session run <id>      Run in this terminal, Ctrl+C to stop
  xsync session preview <id>  Preview changes without transferring/deleting
  xsync session select <id>   Change selection while the session is stopped
  xsync session logs <id>     Show recent logs

Sessions also mirror deletions within the selected scope.
Linux: boot startup via systemd user + linger.
macOS/Windows: startup when the user logs in.`)
}

func command(args []string) error {
	if len(args) == 0 {
		return Menu()
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(Help())
		return nil
	}
	if args[0] == "new" && len(args) == 1 {
		return wizard()
	}
	if args[0] == "list" && len(args) == 1 {
		return list()
	}
	if len(args) != 2 {
		return fmt.Errorf("%s", Help())
	}
	c, err := session.Load(args[1])
	if err != nil {
		return err
	}
	switch args[0] {
	case "status":
		return status(c)
	case "background":
		if c.Auth == "password" && c.Password == "" {
			c.Password, err = readPassword()
			if err != nil {
				return err
			}
		}
		return session.StartDetached(c)
	case "start":
		if err = session.Start(c.ID); err != nil {
			return err
		}
		fmt.Println(tr("Đã bật phiên nền và tự khởi động.", "Background session and startup enabled."))
		return nil
	case "stop":
		if err = session.StopAny(c.ID); err != nil {
			return err
		}
		fmt.Println(tr("Đã dừng và tắt tự khởi động.", "Stopped and startup disabled."))
		return nil
	case "run":
		temporary := os.Getenv("XSYNC_SESSION_PASSWORD")
		os.Unsetenv("XSYNC_SESSION_PASSWORD")
		if c.Password == "" {
			c.Password = temporary
		}
		if c.Auth == "password" && c.Password == "" {
			c.Password, err = readPassword()
			if err != nil {
				return err
			}
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		fmt.Printf("%s → %s:%s\n", c.LocalDir, c.Host, c.Target())
		return session.Run(ctx, c)
	case "preview":
		if c.Auth == "password" && c.Password == "" {
			c.Password, err = readPassword()
			if err != nil {
				return err
			}
		}
		t, err := session.NewTransport(c.Host, c.Auth, c.Password)
		if err != nil {
			return err
		}
		defer t.Close()
		var out string
		err = interruptible(func(ctx context.Context) error { var err error; out, err = t.Sync(ctx, c, true); return err })
		fmt.Print(safeOutput(out))
		return err
	case "select":
		if session.Running(c.ID) {
			return fmt.Errorf("%s", tr("Dừng phiên trước khi đổi dữ liệu được chọn.", "Stop the session before changing its selection."))
		}
		selections, err := selectData(c.LocalDir, c.Selections)
		if err != nil {
			return err
		}
		c.Selections = selections
		return session.Save(c)
	case "logs":
		text, err := logsText(c)
		if err != nil {
			return err
		}
		fmt.Println(safeOutput(text))
		return nil
	}
	return fmt.Errorf("%s", Help())
}

func New() error { return wizard() }

func sessionItems() ([]tui.MenuItem, error) {
	all, err := session.List()
	if err != nil {
		return nil, err
	}
	items := []tui.MenuItem{{ID: "new", Label: tr("Tạo phiên mới", "Create session")}, {ID: "servers", Label: tr("Máy chủ", "Servers")}}
	for _, c := range all {
		phase := phaseLabel(session.Running(c.ID), session.ReadStatus(c.ID).Phase)
		items = append(items, tui.MenuItem{ID: c.ID, Label: fmt.Sprintf("[%s] %s · %s → %s:%s", phase, filepath.Base(c.LocalDir), c.LocalDir, c.Host, c.Target())})
	}
	return items, nil
}
func Menu() error {
	for {
		items, err := sessionItems()
		if err != nil {
			return err
		}
		chosen, err := tui.ChooseItems(tr("Phiên đồng bộ", "Sync sessions"), items, sessionItems)
		if err != nil {
			return err
		}
		if chosen.ID == "new" {
			err = wizard()
		} else if chosen.ID == "servers" {
			err = Servers()
		} else {
			err = sessionActions(chosen.ID)
		}
		if err != nil && !errors.Is(err, tui.ErrCancelled) {
			if err = tui.ShowText(tr("Thông báo", "Notice"), err.Error()); err != nil {
				return err
			}
		}
	}
}
func sessionActions(id string) error {
	c, err := session.Load(id)
	if err != nil {
		return err
	}
	actions := []tui.MenuItem{{ID: "status", Label: tr("Chi tiết và trạng thái", "Details and status")}}
	if session.Running(c.ID) {
		actions = append(actions, tui.MenuItem{ID: "stop", Label: tr("Dừng đồng bộ", "Stop syncing")})
	} else {
		actions = append(actions, tui.MenuItem{ID: "start", Label: tr("Bắt đầu đồng bộ", "Start syncing")}, tui.MenuItem{ID: "select", Label: tr("Chọn lại dữ liệu", "Change selected data")})
	}
	actions = append(actions, tui.MenuItem{ID: "logs", Label: tr("Hoạt động gần đây", "Recent activity")}, tui.MenuItem{ID: "preview", Label: tr("Xem trước thay đổi", "Preview changes")})
	chosen, err := tui.ChooseItems(filepath.Base(c.LocalDir)+" · "+c.Host, actions, nil)
	if err != nil {
		return err
	}
	switch chosen.ID {
	case "status":
		return tui.ShowText(c.ID, statusText(c))
	case "logs":
		text, err := logsText(c)
		if err != nil {
			return err
		}
		return tui.ShowText(c.ID, text)
	case "preview":
		if c.Auth == "password" && c.Password == "" {
			c.Password, err = readPassword()
			if err != nil {
				return err
			}
		}
		transport, err := session.NewTransport(c.Host, c.Auth, c.Password)
		if err != nil {
			return err
		}
		defer transport.Close()
		var out string
		err = interruptible(func(ctx context.Context) error { var err error; out, err = transport.Sync(ctx, c, true); return err })
		if err != nil {
			return err
		}
		return tui.ShowText(tr("Xem trước", "Preview"), out)
	case "start":
		mode, err := tui.ChooseMenu(tr("Bắt đầu phiên", "Start session"), []string{tr("Chạy nền, không tự khởi động", "Background without startup"), tr("Chạy nền và tự khởi động", "Background with startup"), tr("Chạy trong terminal", "Run in this terminal")}, 0)
		if err != nil {
			return err
		}
		if mode == 1 {
			return command([]string{"start", id})
		}
		if mode == 2 {
			return command([]string{"run", id})
		}
		if c.Auth == "password" && c.Password == "" {
			c.Password, err = readPassword()
			if err != nil {
				return err
			}
		}
		return session.StartDetached(c)
	default:
		return command([]string{chosen.ID, id})
	}
}

func readPassword() (string, error) {
	fmt.Print(tr("Mật khẩu SSH (ẩn khi nhập): ", "SSH password (hidden): "))
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", err
	}
	if len(password) == 0 {
		return "", fmt.Errorf("%s", tr("Mật khẩu trống.", "Empty password."))
	}
	return string(password), nil
}

func selectData(root string, initial ...[]session.Selection) ([]session.Selection, error) {
	var selected []tui.BrowserSelection
	if len(initial) > 0 {
		for _, s := range initial[0] {
			selected = append(selected, tui.BrowserSelection{Path: s.Path, Dir: s.Dir})
		}
	}
	_, items, err := tui.Browse(context.Background(), tui.BrowserOptions{
		Title: tr("Chọn file/thư mục con cần phản chiếu; a để chọn tất cả", "Select files/subfolders to mirror; a selects everything"),
		Start: filepath.ToSlash(root), Root: filepath.ToSlash(root), List: fsbrowse.Local, Multi: true, Selected: selected,
	})
	if err != nil {
		return nil, err
	}
	var result []session.Selection
	for _, s := range items {
		result = append(result, session.Selection{Path: s.Path, Dir: s.Dir})
	}
	return result, nil
}

func connectWizard() (session.Config, *session.Transport, error) {
	hosts := config.ParseSSHHosts()
	if len(hosts) == 0 {
		return session.Config{}, nil, fmt.Errorf("%s", tr("Thêm Host vào ~/.ssh/config trước.", "Add a Host to ~/.ssh/config first."))
	}
	for {
		settings, err := serverstore.Read()
		if err != nil {
			return session.Config{}, nil, err
		}
		credentials := serverstore.EffectiveCredentials(settings)
		labels := make([]string, len(hosts))
		for i, host := range hosts {
			labels[i] = host
			if !settings[host].KeyVerifiedAt.IsZero() {
				labels[i] += " · " + tr("Đã dùng key", "Key verified")
			}
			if credentials[host].Password != "" {
				labels[i] += " · " + tr("Đã lưu mật khẩu", "Password saved")
			}
		}
		index, err := tui.ChooseMenu(tr("1/4 · Chọn máy chủ", "1/4 · Choose server"), labels, 0)
		if err != nil {
			return session.Config{}, nil, err
		}
		host := hosts[index]
		for {
			options := []tui.MenuItem{{ID: "key", Label: "SSH key / agent"}}
			if credentials[host].Password != "" {
				options = append(options, tui.MenuItem{ID: "saved", Label: tr("Dùng mật khẩu đã lưu", "Use saved password")})
			}
			options = append(options, tui.MenuItem{ID: "password", Label: tr("Nhập mật khẩu", "Enter password")}, tui.MenuItem{ID: "back", Label: tr("Quay lại", "Back")})
			auth, err := tui.ChooseItems(host, options, nil)
			if errors.Is(err, tui.ErrCancelled) || auth.ID == "back" {
				break
			}
			if err != nil {
				return session.Config{}, nil, err
			}
			c := session.Config{ID: session.NewID(), Host: host, Auth: "key", CreatedAt: time.Now()}
			if auth.ID != "key" {
				c.Auth = "password"
				if auth.ID == "saved" {
					c.Password = credentials[host].Password
				} else {
					c.Password, err = readPassword()
					if err != nil {
						return c, nil, err
					}
				}
			}
			transport, err := session.NewTransport(host, c.Auth, c.Password)
			if err == nil {
				err = interruptible(func(ctx context.Context) error {
					if c.Auth == "key" {
						if _, err := transport.Run(ctx, "true"); err != nil {
							return err
						}
						if err := serverstore.Update(host, func(s *serverstore.Setting) { s.KeyVerifiedAt = time.Now().UTC() }); err != nil {
							return err
						}
					}
					return transport.Check(ctx)
				})
			}
			if err == nil {
				return c, transport, nil
			}
			if transport != nil {
				transport.Close()
			}
			if errors.Is(err, tui.ErrCancelled) {
				return c, nil, err
			}
			if err = tui.ShowText(tr("Kết nối chưa sẵn sàng", "Connection not ready"), err.Error()); err != nil {
				return c, nil, err
			}
		}
	}
}

func wizard() error {
	c, transport, err := connectWizard()
	if err != nil {
		return err
	}
	defer transport.Close()
	savePassword := true
	if c.Auth == "password" {
		choice, err := tui.ChooseMenu(tr("Lưu mật khẩu trong phiên", "Save password in session"), []string{tr("Lưu để dùng lại và tự khởi động", "Save for reuse and startup"), tr("Chỉ giữ trong bộ nhớ khi chạy", "Keep in memory while running")}, 0)
		if err != nil {
			return err
		}
		savePassword = choice == 0
	}
	for step := 1; ; {
		switch step {
		case 1:
			local, _, err := tui.Browse(context.Background(), tui.BrowserOptions{Title: tr("2/4 · Thư mục local A", "2/4 · Local folder A"), Start: filepath.ToSlash(c.LocalDir), List: fsbrowse.Local})
			if err != nil {
				return err
			}
			if c.LocalDir != filepath.FromSlash(local) {
				c.Selections = nil
			}
			c.LocalDir = filepath.FromSlash(local)
			parent, _, err := tui.Browse(context.Background(), tui.BrowserOptions{Title: tr("2/4 · Thư mục chứa B · Esc quay lại local", "2/4 · Remote parent B · Esc back to local"), Start: c.RemoteParent, List: transport.List, Mkdir: transport.Mkdir})
			if errors.Is(err, tui.ErrCancelled) {
				continue
			}
			if err != nil {
				return err
			}
			c.RemoteParent = parent
			step = 2
		case 2:
			selections, err := selectData(c.LocalDir, c.Selections)
			if errors.Is(err, tui.ErrCancelled) {
				step = 1
				continue
			}
			if err != nil {
				return err
			}
			c.Selections = selections
			step = 3
		case 3:
			if err = c.Validate(); err != nil {
				return err
			}
			var preview string
			err = interruptible(func(ctx context.Context) error {
				var err error
				preview, err = transport.Sync(ctx, c, true)
				return err
			})
			if err != nil {
				return err
			}
			if err = tui.ShowText(tr("4/4 · Xem trước thay đổi", "4/4 · Preview changes"), c.LocalDir+" → "+c.Host+":"+c.Target()+"\n\n"+preview); err != nil {
				return err
			}
			actions := []tui.MenuItem{{ID: "background", Label: tr("Lưu và chạy nền, không tự khởi động", "Save and run in background without startup")}}
			if c.Auth == "key" || savePassword {
				actions = append(actions, tui.MenuItem{ID: "autostart", Label: tr("Lưu, chạy nền và tự khởi động", "Save, run in background and enable startup")})
			}
			actions = append(actions, tui.MenuItem{ID: "save", Label: tr("Chỉ lưu phiên", "Save session only")}, tui.MenuItem{ID: "back", Label: tr("Quay lại chọn dữ liệu", "Back to data selection")}, tui.MenuItem{ID: "cancel", Label: tr("Hủy", "Cancel")})
			choice, err := tui.ChooseItems(tr("Phản chiếu cả thao tác xóa trong phạm vi đã chọn", "Mirror deletions within the selected scope"), actions, nil)
			if err != nil {
				return err
			}
			if choice.ID == "back" {
				step = 2
				continue
			}
			if choice.ID == "cancel" {
				return tui.ErrCancelled
			}
			saved := c
			if !savePassword {
				saved.Password = ""
			}
			if err = session.Save(saved); err != nil {
				return err
			}
			switch choice.ID {
			case "background":
				err = session.StartDetached(c)
			case "autostart":
				err = session.Start(c.ID)
			}
			if err != nil {
				return fmt.Errorf("%s %s: %w", tr("Đã lưu phiên; chưa khởi chạy", "Session saved; not started"), c.ID, err)
			}
			return tui.ShowText(tr("Đã lưu phiên", "Session saved"), c.ID+"\n"+c.LocalDir+" → "+c.Host+":"+c.Target())
		}
	}
}

func list() error {
	all, err := session.List()
	if err != nil {
		return err
	}
	if len(all) == 0 {
		fmt.Println(tr("Chưa có phiên. Chạy: xsync session new", "No sessions yet. Run: xsync session new"))
	}
	for _, c := range all {
		phase := phaseLabel(session.Running(c.ID), session.ReadStatus(c.ID).Phase)
		fmt.Printf("%s [%s] %s → %s:%s\n", c.ID, phase, c.LocalDir, c.Host, c.Target())
	}
	return nil
}

func status(c session.Config) error { fmt.Print(statusText(c)); return nil }
func statusText(c session.Config) string {
	var out strings.Builder
	s := session.ReadStatus(c.ID)
	phase := phaseLabel(session.Running(c.ID), s.Phase)
	fmt.Fprintf(&out, "%s [%s]\n%s → %s:%s\n", c.ID, phase, c.LocalDir, c.Host, c.Target())
	fmt.Fprintf(&out, "%s: %s\n", tr("Xác thực", "Authentication"), c.Auth)
	for _, item := range c.Selections {
		fmt.Fprintf(&out, "  • %s\n", item.Path)
	}
	if !s.LastSuccess.IsZero() {
		fmt.Fprintf(&out, "%s: %s\n", tr("Đồng bộ thành công gần nhất", "Last successful sync"), s.LastSuccess.Local().Format(time.RFC3339))
	}
	if s.LastError != "" {
		fmt.Fprintf(&out, "%s: %s\n", tr("Lỗi gần nhất", "Latest error"), safeOutput(s.LastError))
	}
	return out.String()
}

func safeOutput(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return '�'
		}
		return r
	}, s)
}

func interruptible(fn func(context.Context) error) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	err := fn(ctx)
	if ctx.Err() != nil {
		return tui.ErrCancelled
	}
	return err
}

func logsText(c session.Config) (string, error) {
	dir, err := session.Dir(c.ID)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, "session.log"))
	if os.IsNotExist(err) {
		return tr("Chưa có hoạt động.", "No activity yet."), nil
	}
	if err != nil {
		return "", err
	}
	return safeOutput(string(data)), nil
}

func phaseLabel(running bool, phase string) string {
	if !running {
		return tr("Đã dừng", "Stopped")
	}
	switch phase {
	case "watching":
		return tr("Đang theo dõi", "Watching")
	case "syncing":
		return tr("Đang đồng bộ", "Syncing")
	case "starting":
		return tr("Đang kết nối", "Connecting")
	case "retrying":
		return tr("Đang thử lại", "Retrying")
	}
	return tr("Đang chạy", "Running")
}
