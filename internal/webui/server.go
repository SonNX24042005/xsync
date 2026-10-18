// Package webui serves the local graphical interface, embedded in the xsync binary.
package webui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"xsync/internal/config"
	"xsync/internal/session"
)

//go:embed web/*
var assets embed.FS

type connection struct {
	mu        sync.Mutex
	transport *session.Transport
	lastUsed  time.Time
	closed    bool
}
type worker struct {
	cancel context.CancelFunc
	done   chan struct{}
}

type App struct {
	token, host string
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	connections map[string]*connection
	workers     map[string]*worker
}

func New(parent context.Context) (*App, error) {
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	a := &App{token: hex.EncodeToString(secret[:]), ctx: ctx, cancel: cancel, connections: map[string]*connection{}, workers: map[string]*worker{}}
	go a.expireConnections()
	return a, nil
}

func (a *App) Close() {
	a.cancel()
	a.mu.Lock()
	var waits []chan struct{}
	for _, w := range a.workers {
		w.cancel()
		waits = append(waits, w.done)
	}
	connections := a.connections
	a.connections = map[string]*connection{}
	a.mu.Unlock()
	for _, done := range waits {
		<-done
	}
	for _, c := range connections {
		c.mu.Lock()
		c.closed = true
		c.transport.Close()
		c.mu.Unlock()
	}
}

func (a *App) expireConnections() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-t.C:
			a.mu.Lock()
			for id, c := range a.connections {
				if c.mu.TryLock() {
					if time.Since(c.lastUsed) > 30*time.Minute {
						delete(a.connections, id)
						c.closed = true
						c.transport.Close()
					}
					c.mu.Unlock()
				}
			}
			a.mu.Unlock()
		}
	}
}

func (a *App) Handler(host string) http.Handler {
	a.host = host
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/info", func(w http.ResponseWriter, r *http.Request) {
		cwd, _ := os.Getwd()
		home, _ := os.UserHomeDir()
		hosts := config.ParseSSHHosts()
		settings, err := readSettings()
		if err != nil {
			jsonError(w, 500, err)
			return
		}
		credentials := effectiveCredentials(settings)
		saved := map[string]string{}
		for _, host := range hosts {
			if credential := credentials[host]; credential.password != "" {
				saved[host] = credential.source
			}
		}
		jsonResponse(w, 200, map[string]any{"hosts": hosts, "servers": serverInfos(hosts, settings, credentials), "saved_passwords": saved, "cwd": filepath.ToSlash(cwd), "home": filepath.ToSlash(home), "platform": runtime.GOOS})
	})
	mux.HandleFunc("PUT /api/servers/{host}", a.saveServer)
	mux.HandleFunc("POST /api/servers/{host}/check-key", a.checkServerKey)
	mux.HandleFunc("POST /api/servers/{host}/check-connection", a.checkServerConnection)
	mux.HandleFunc("GET /api/sessions", a.listSessions)
	mux.HandleFunc("POST /api/connections", a.connect)
	mux.HandleFunc("DELETE /api/connections/{id}", a.disconnect)
	mux.HandleFunc("GET /api/local", a.local)
	mux.HandleFunc("GET /api/remote", a.remote)
	mux.HandleFunc("POST /api/remote/mkdir", a.mkdir)
	mux.HandleFunc("POST /api/preview", a.preview)
	mux.HandleFunc("POST /api/sessions", a.createSession)
	mux.HandleFunc("POST /api/sessions/{id}/start", a.startSession)
	mux.HandleFunc("POST /api/sessions/{id}/stop", a.stopSession)
	mux.HandleFunc("GET /api/sessions/{id}/logs", a.logs)
	files, _ := fs.Sub(assets, "web")
	fileServer := http.FileServer(http.FS(files))
	mux.Handle("GET /", fileServer)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'")
		if r.Host != host {
			jsonError(w, 403, fmt.Errorf("invalid local host"))
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+host {
			jsonError(w, 403, fmt.Errorf("cross-origin requests are not allowed"))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			cookieName := "xsync-ui-" + strings.ReplaceAll(host, ":", "-")
			if provided == "" {
				if cookie, err := r.Cookie(cookieName); err == nil {
					provided = cookie.Value
				}
			}
			if subtle.ConstantTimeCompare([]byte(provided), []byte(a.token)) != 1 {
				jsonError(w, 401, fmt.Errorf("Mở giao diện bằng đường dẫn được in bởi lệnh xsync ui."))
				return
			}
			if r.URL.Path == "/api/info" && strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") && strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") != "" {
				http.SetCookie(w, &http.Cookie{Name: cookieName, Value: a.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
			}
			r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
		}
		mux.ServeHTTP(w, r)
	})
}

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func jsonError(w http.ResponseWriter, status int, err error) {
	jsonResponse(w, status, map[string]string{"error": err.Error()})
}
func decode(r *http.Request, v any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return fmt.Errorf("expected application/json")
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		return fmt.Errorf("expected one JSON object")
	}
	return nil
}

func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// Serve binds only IPv4 loopback. The bearer token stays in the browser URL fragment.
func Serve(ctx context.Context, port int, open bool) error {
	listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	defer listener.Close()
	a, err := New(ctx)
	if err != nil {
		return err
	}
	defer a.Close()
	host := listener.Addr().String()
	url := "http://" + host + "/#token=" + a.token
	fmt.Printf("xsync · giao diện web local\n%s\nGiữ tiến trình này chạy. Đóng tab không dừng đồng bộ; Ctrl+C dừng giao diện và các phiên tạm do giao diện mở.\n", url)
	if open {
		if err := OpenBrowser(url); err != nil {
			fmt.Fprintf(os.Stderr, "Không mở được trình duyệt: %v\n", err)
		}
	}
	server := &http.Server{Handler: a.Handler(host), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
