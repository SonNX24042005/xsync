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
	"xsync/internal/serverstore"
	"xsync/internal/session"
	"xsync/internal/tui"
)

var tr = tui.Text

// RunWorker is the noninteractive entry point used by background processes and services.
func RunWorker(id string) error {
	temporary := os.Getenv("XSYNC_SESSION_PASSWORD")
	os.Unsetenv("XSYNC_SESSION_PASSWORD")
	c, err := session.Load(id)
	if err != nil {
		return err
	}
	if c.Password == "" {
		c.Password = temporary
	}
	if c.Auth == "password" && c.Password == "" {
		return fmt.Errorf("%s", tr("Worker cần mật khẩu đã lưu hoặc mật khẩu tạm từ tiến trình khởi chạy.", "The worker needs a saved password or a temporary password from its launcher."))
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return session.Run(ctx, c)
}

func runForeground(c session.Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- session.Run(ctx, c); cancel() }()
	err := tui.ShowLiveText(ctx, tr("Đồng bộ trực tiếp · Esc để dừng", "Live sync · Esc to stop"), func() string { return statusText(c) })
	wasCancelled := ctx.Err() != nil
	cancel()
	if runErr := <-done; runErr != nil {
		return runErr
	}
	if wasCancelled {
		return nil
	}
	return err
}

func changeSelection(c session.Config) error {
	if session.Running(c.ID) {
		return fmt.Errorf("%s", tr("Dừng phiên trước khi đổi dữ liệu được chọn.", "Stop the session before changing its selection."))
	}
	mode, selections, err := selectScope(c)
	if err != nil {
		return err
	}
	c.Selections = selections
	c.SelectionMode = mode
	return session.Save(c)
}

func New() error { return wizard() }

func sessionItems() ([]tui.MenuItem, error) {
	all, err := session.List()
	if err != nil {
		return nil, err
	}
	items := []tui.MenuItem{{ID: "new", Label: tr("Tạo phiên mới", "Create session")}, {ID: "servers", Label: tr("Máy chủ", "Servers")}}
	for _, c := range all {
		status := session.ReadStatus(c.ID)
		phase := phaseLabel(session.Running(c.ID), status.Phase)
		if status.Progress != nil {
			phase += " · " + progressSummary(status.Progress)
		}
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
		return tui.ShowLiveText(context.Background(), c.ID, func() string { return statusText(c) })
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
			return session.Start(c.ID)
		}
		if mode == 2 {
			if c.Auth == "password" && c.Password == "" {
				c.Password, err = readPassword()
				if err != nil {
					return err
				}
			}
			return runForeground(c)
		}
		if c.Auth == "password" && c.Password == "" {
			c.Password, err = readPassword()
			if err != nil {
				return err
			}
		}
		return session.StartDetached(c)
	case "stop":
		return session.StopAny(c.ID)
	case "select":
		return changeSelection(c)
	default:
		return fmt.Errorf("unknown session action: %s", chosen.ID)
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

func selectScope(c session.Config) (string, []session.Selection, error) {
	items := []tui.MenuItem{{ID: session.SelectionInclude, Label: tr("Chỉ đồng bộ mục đã chọn", "Sync selected items only")}, {ID: session.SelectionExclude, Label: tr("Đồng bộ tất cả, trừ mục đã chọn", "Sync everything except selected items")}}
	if c.Mode() == session.SelectionExclude {
		items[0], items[1] = items[1], items[0]
	}
	choice, err := tui.ChooseItems(tr("Chế độ đồng bộ", "Sync mode"), items, nil)
	if err != nil {
		return "", nil, err
	}
	initial := c.Selections
	if choice.ID != c.Mode() {
		initial = nil
	}
	selected, err := selectData(c.LocalDir, choice.ID, initial)
	return choice.ID, selected, err
}

func selectData(root, mode string, initial []session.Selection) ([]session.Selection, error) {
	var selected []tui.BrowserSelection
	for _, s := range initial {
		selected = append(selected, tui.BrowserSelection{Path: s.Path, Dir: s.Dir})
	}
	title := tr("Chọn file/thư mục con cần đồng bộ; a để chọn tất cả", "Select files/subfolders to sync; a selects everything")
	if mode == session.SelectionExclude {
		title = tr("Chọn mục loại trừ; Tab để đồng bộ tất cả nếu không loại trừ mục nào", "Select exclusions; Tab syncs everything if nothing is excluded")
	}
	_, items, err := tui.Browse(context.Background(), tui.BrowserOptions{
		Title: title, AllowEmpty: mode == session.SelectionExclude, ExcludeRoot: mode == session.SelectionExclude,
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
			mode, selections, err := selectScope(c)
			if errors.Is(err, tui.ErrCancelled) {
				step = 1
				continue
			}
			if err != nil {
				return err
			}
			c.Selections = selections
			c.SelectionMode = mode
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
			if err = tui.ShowText(tr("4/4 · Xem trước thay đổi", "4/4 · Preview changes"), c.LocalDir+" → "+c.Host+":"+c.Target()+"\n"+scopeLabel(c)+"\n\n"+preview); err != nil {
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

func statusText(c session.Config) string {
	var out strings.Builder
	s := session.ReadStatus(c.ID)
	phase := phaseLabel(session.Running(c.ID), s.Phase)
	fmt.Fprintf(&out, "%s [%s]\n%s → %s:%s\n", c.ID, phase, c.LocalDir, c.Host, c.Target())
	fmt.Fprintf(&out, "%s: %s\n", tr("Xác thực", "Authentication"), c.Auth)
	fmt.Fprintf(&out, "%s: %s\n", tr("Chế độ", "Mode"), scopeLabel(c))
	if p := s.Progress; p != nil {
		fmt.Fprintf(&out, "\n%s\n", progressSummary(p))
		if p.Percent != nil {
			percent := max(0, min(100, *p.Percent))
			filled := percent / 5
			fmt.Fprintf(&out, "[%s%s]\n", strings.Repeat("=", filled), strings.Repeat("·", 20-filled))
		}
		if p.BytesProcessed > 0 {
			fmt.Fprintf(&out, "%s: %s\n", tr("Dữ liệu đã xử lý", "Data processed"), byteLabel(p.BytesProcessed))
		}
		if s.Phase == "syncing" && p.Stage == "transferring" {
			if p.Rate != "" {
				fmt.Fprintf(&out, "%s: %s\n", tr("Tốc độ", "Rate"), p.Rate)
			}
			if p.ETA != "" {
				fmt.Fprintf(&out, "%s: ~%s\n", tr("Thời gian còn lại", "Time remaining"), p.ETA)
			}
		}
		if p.Stage == "renaming" || p.Stage == "deleting" {
			fmt.Fprintf(&out, "%d/%d %s\n", p.Checked, p.Total, tr("mục", "items"))
		}
		if p.Transfers > 0 {
			fmt.Fprintf(&out, "%s: %d\n", tr("Tệp đã truyền", "Files transferred"), p.Transfers)
		}
		out.WriteString("\n")
	}
	if c.Mode() == session.SelectionExclude {
		out.WriteString(tr("Loại trừ:\n", "Excluded:\n"))
	}
	for _, item := range c.Selections {
		fmt.Fprintf(&out, "  • %s\n", item.Path)
	}
	if !s.LastSuccess.IsZero() {
		fmt.Fprintf(&out, "%s: %s\n", tr("Đồng bộ thành công gần nhất", "Last successful sync"), s.LastSuccess.Local().Format(time.RFC3339))
	}
	if !s.LastChecked.IsZero() {
		fmt.Fprintf(&out, "%s: %s\n", tr("Kiểm tra thay đổi gần nhất", "Last change check"), s.LastChecked.Local().Format(time.RFC3339))
	}
	if s.LastError != "" {
		fmt.Fprintf(&out, "%s: %s\n", tr("Lỗi gần nhất", "Latest error"), safeOutput(s.LastError))
	}
	return out.String()
}

func scopeLabel(c session.Config) string {
	if c.Mode() == session.SelectionExclude {
		return tr("Đồng bộ tất cả, trừ mục đã chọn", "Sync everything except selected items")
	}
	return tr("Chỉ đồng bộ mục đã chọn", "Sync selected items only")
}

func progressSummary(p *session.Progress) string {
	labels := map[string]string{
		"connecting": tr("Đang kết nối", "Connecting"), "scanning": tr("Đang quét", "Scanning"),
		"renaming": tr("Đang xử lý đổi tên", "Processing renames"), "deleting": tr("Đang xóa mục đã xóa", "Removing deleted items"),
		"transferring": tr("Đang đồng bộ", "Syncing"), "finishing": tr("Đang hoàn tất", "Finishing"),
		"completed": tr("Đã đồng bộ", "Synced"),
	}
	label := labels[p.Stage]
	if p.Percent != nil {
		estimate := ""
		if p.Estimated {
			estimate = "~"
		}
		label += fmt.Sprintf(" %s%d%%", estimate, max(0, min(100, *p.Percent)))
	}
	return label
}

func byteLabel(n int64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	value, unit := float64(n), 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
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
		return tr("Chờ thay đổi", "Waiting for changes")
	case "checking":
		return tr("Đang kiểm tra thay đổi", "Checking for changes")
	case "syncing":
		return tr("Đang đồng bộ", "Syncing")
	case "starting":
		return tr("Đang kết nối", "Connecting")
	case "retrying":
		return tr("Đang thử lại", "Retrying")
	}
	return tr("Đang chạy", "Running")
}
