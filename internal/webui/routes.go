package webui

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xsync/internal/config"
	"xsync/internal/fsbrowse"
	"xsync/internal/session"
)

func (a *App) connect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host             string `json:"host"`
		Auth             string `json:"auth"`
		Password         string `json:"password"`
		UseSavedPassword bool   `json:"use_saved_password"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, 400, err)
		return
	}
	found := false
	for _, h := range config.ParseSSHHosts() {
		if h == req.Host {
			found = true
		}
	}
	if !found || (req.Auth != "key" && req.Auth != "password") {
		jsonError(w, 400, fmt.Errorf("Chọn một server trong SSH config và cách xác thực hợp lệ."))
		return
	}
	if req.Auth == "key" {
		req.Password = ""
	} else if req.UseSavedPassword {
		settings, err := readSettings()
		if err != nil {
			jsonError(w, 500, err)
			return
		}
		req.Password = effectiveCredentials(settings)[req.Host].Password
		if req.Password == "" {
			jsonError(w, 400, fmt.Errorf("Không tìm thấy mật khẩu đã lưu cho server này. Nhập mật khẩu hoặc dùng SSH key."))
			return
		}
	}
	t, err := session.NewTransport(req.Host, req.Auth, req.Password)
	if err == nil && req.Auth == "key" {
		_, err = t.Run(r.Context(), "true")
		if err == nil {
			err = updateSetting(req.Host, func(s *serverSetting) { s.KeyVerifiedAt = time.Now().UTC() })
		}
	}
	if err == nil {
		err = t.Check(r.Context())
	}
	if err != nil {
		if t != nil {
			t.Close()
		}
		jsonError(w, 400, err)
		return
	}
	id := session.NewID()
	a.mu.Lock()
	if len(a.connections) >= 32 {
		a.mu.Unlock()
		t.Close()
		jsonError(w, 409, fmt.Errorf("Đóng bớt phiên kết nối trước khi tạo kết nối mới."))
		return
	}
	a.connections[id] = &connection{transport: t, lastUsed: time.Now()}
	a.mu.Unlock()
	jsonResponse(w, 200, map[string]string{"id": id, "host": req.Host})
}

// lockConnection always locks in the same order as expiration and shutdown.
func (a *App) lockConnection(id string) (*connection, error) {
	a.mu.Lock()
	c := a.connections[id]
	if c == nil {
		a.mu.Unlock()
		return nil, fmt.Errorf("Kết nối đã đóng hoặc hết hạn. Kết nối lại với server.")
	}
	a.mu.Unlock()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, fmt.Errorf("Kết nối đã đóng. Hãy kết nối lại với server.")
	}
	c.lastUsed = time.Now()
	return c, nil
}
func (a *App) disconnect(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	c := a.connections[r.PathValue("id")]
	delete(a.connections, r.PathValue("id"))
	a.mu.Unlock()
	if c != nil {
		c.mu.Lock()
		c.closed = true
		c.transport.Close()
		c.mu.Unlock()
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}

func (a *App) local(w http.ResponseWriter, r *http.Request) {
	listing, err := fsbrowse.Local(r.Context(), r.URL.Query().Get("path"))
	if err == nil && r.URL.Query().Get("root") != "" {
		root, resolveErr := filepath.EvalSymlinks(filepath.FromSlash(r.URL.Query().Get("root")))
		if resolveErr != nil {
			err = resolveErr
		} else {
			rel, relErr := filepath.Rel(root, filepath.FromSlash(listing.Path))
			if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				err = fmt.Errorf("Đường dẫn phải nằm trong thư mục nguồn.")
			}
		}
	}
	if err != nil {
		jsonError(w, 400, err)
		return
	}
	jsonResponse(w, 200, listing)
}
func (a *App) remote(w http.ResponseWriter, r *http.Request) {
	c, err := a.lockConnection(r.URL.Query().Get("connection"))
	if err != nil {
		jsonError(w, 400, err)
		return
	}
	defer c.mu.Unlock()
	listing, err := c.transport.List(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		jsonError(w, 400, err)
		return
	}
	jsonResponse(w, 200, listing)
}
func (a *App) mkdir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Connection string `json:"connection"`
		Path       string `json:"path"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, 400, err)
		return
	}
	c, err := a.lockConnection(req.Connection)
	if err != nil {
		jsonError(w, 400, err)
		return
	}
	defer c.mu.Unlock()
	if err := c.transport.Mkdir(r.Context(), req.Path); err != nil {
		jsonError(w, 400, err)
		return
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}

type draft struct {
	Connection   string              `json:"connection"`
	LocalDir     string              `json:"local_dir"`
	RemoteParent string              `json:"remote_parent"`
	Selections   []session.Selection `json:"selections"`
	SavePassword bool                `json:"save_password"`
	Start        bool                `json:"start"`
	Autostart    bool                `json:"autostart"`
}

func (d draft) config(c *connection) (session.Config, error) {
	local, err := filepath.EvalSymlinks(filepath.FromSlash(d.LocalDir))
	if err != nil {
		return session.Config{}, err
	}
	local, err = filepath.Abs(local)
	if err != nil {
		return session.Config{}, err
	}
	result := session.Config{ID: session.NewID(), Host: c.transport.Host, Auth: c.transport.Auth, Password: c.transport.Password, LocalDir: local, RemoteParent: d.RemoteParent, Selections: d.Selections, CreatedAt: time.Now()}
	return result, result.Validate()
}
func (a *App) preview(w http.ResponseWriter, r *http.Request) {
	var req draft
	if err := decode(r, &req); err != nil {
		jsonError(w, 400, err)
		return
	}
	c, err := a.lockConnection(req.Connection)
	if err != nil {
		jsonError(w, 400, err)
		return
	}
	defer c.mu.Unlock()
	cfg, err := req.config(c)
	if err != nil {
		jsonError(w, 400, err)
		return
	}
	out, err := c.transport.Sync(r.Context(), cfg, true)
	if err != nil {
		jsonError(w, 400, err)
		return
	}
	jsonResponse(w, 200, map[string]string{"output": out, "target": cfg.Target()})
}
func (a *App) createSession(w http.ResponseWriter, r *http.Request) {
	var req draft
	if err := decode(r, &req); err != nil {
		jsonError(w, 400, err)
		return
	}
	c, err := a.lockConnection(req.Connection)
	if err != nil {
		jsonError(w, 400, err)
		return
	}
	defer c.mu.Unlock()
	cfg, err := req.config(c)
	if err != nil {
		jsonError(w, 400, err)
		return
	}
	if req.Autostart && cfg.Auth == "password" && !req.SavePassword {
		jsonError(w, 400, fmt.Errorf("Tự khởi động cần SSH key hoặc mật khẩu đã lưu."))
		return
	}
	// Validate the target and selection with a fresh dry-run before persisting anything.
	if _, err = c.transport.Sync(r.Context(), cfg, true); err != nil {
		jsonError(w, 400, err)
		return
	}
	saved := cfg
	if !req.SavePassword {
		saved.Password = ""
	}
	if err = session.Save(saved); err != nil {
		jsonError(w, 409, err)
		return
	}
	var startErr error
	if req.Start {
		if req.Autostart {
			startErr = session.Start(cfg.ID)
		} else {
			startErr = a.run(cfg)
		}
	}
	response := map[string]any{"id": cfg.ID, "target": cfg.Target()}
	if startErr != nil {
		response["warning"] = "Đã lưu cấu hình nhưng chưa khởi chạy: " + startErr.Error()
	}
	jsonResponse(w, 201, response)
}

func (a *App) run(c session.Config) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.workers[c.ID] != nil || session.Running(c.ID) {
		return fmt.Errorf("Phiên đang chạy.")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	w := &worker{cancel: cancel, done: make(chan struct{})}
	a.workers[c.ID] = w
	go func() {
		defer close(w.done)
		_ = session.Run(ctx, c)
		a.mu.Lock()
		if a.workers[c.ID] == w {
			delete(a.workers, c.ID)
		}
		a.mu.Unlock()
	}()
	return nil
}
func (a *App) listSessions(w http.ResponseWriter, r *http.Request) {
	all, err := session.List()
	if err != nil {
		jsonError(w, 500, err)
		return
	}
	result := make([]map[string]any, 0, len(all))
	for _, c := range all {
		state := session.ReadStatus(c.ID)
		running := session.Running(c.ID)
		if !running {
			state.Phase = "stopped"
		}
		a.mu.Lock()
		owned := a.workers[c.ID] != nil
		a.mu.Unlock()
		result = append(result, map[string]any{"id": c.ID, "host": c.Host, "auth": c.Auth, "password_saved": c.Password != "", "local_dir": c.LocalDir, "remote_parent": c.RemoteParent, "target": c.Target(), "selections": c.Selections, "created_at": c.CreatedAt, "running": running, "managed": owned, "status": state})
	}
	jsonResponse(w, 200, result)
}
func (a *App) startSession(w http.ResponseWriter, r *http.Request) {
	c, err := session.Load(r.PathValue("id"))
	if err != nil {
		jsonError(w, 404, err)
		return
	}
	var req struct {
		Autostart bool   `json:"autostart"`
		Password  string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, 400, err)
		return
	}
	if req.Autostart {
		err = session.Start(c.ID)
	} else {
		if c.Password == "" {
			c.Password = req.Password
		}
		if c.Auth == "password" && c.Password == "" {
			jsonError(w, 400, fmt.Errorf("Nhập mật khẩu cho phiên chưa lưu mật khẩu."))
			return
		}
		err = a.run(c)
	}
	if err != nil {
		jsonError(w, 400, err)
		return
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
func (a *App) stopSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := session.Load(id); err != nil {
		jsonError(w, 404, err)
		return
	}
	a.mu.Lock()
	worker := a.workers[id]
	if worker != nil {
		worker.cancel()
	}
	a.mu.Unlock()
	if worker != nil {
		select {
		case <-worker.done:
		case <-r.Context().Done():
			return
		}
	} else {
		if err := session.StopAny(id); err != nil {
			jsonError(w, 400, fmt.Errorf("Không dừng được dịch vụ; nếu phiên chạy bằng 'session run', hãy Ctrl+C trong terminal của phiên. %w", err))
			return
		}
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
func (a *App) logs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := session.Load(id)
	if err != nil {
		jsonError(w, 404, err)
		return
	}
	dir, _ := session.Dir(id)
	data, err := os.ReadFile(filepath.Join(dir, "session.log"))
	if os.IsNotExist(err) {
		data = nil
	} else if err != nil {
		jsonError(w, 500, err)
		return
	}
	text := string(data)
	if c.Password != "" {
		text = strings.ReplaceAll(text, c.Password, "[redacted]")
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 150 {
		lines = lines[len(lines)-150:]
	}
	jsonResponse(w, 200, map[string]string{"output": strings.Join(lines, "\n")})
}
