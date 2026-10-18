package webui

import (
	"fmt"
	"net/http"
	"xsync/internal/serverstore"
)

type serverSetting = serverstore.Setting
type serverInfo = serverstore.Info

func settingsPath() (string, error)                   { return serverstore.Path() }
func readSettings() (map[string]serverSetting, error) { return serverstore.Read() }
func updateSetting(host string, update func(*serverSetting)) error {
	return serverstore.Update(host, update)
}
func effectiveCredentials(settings map[string]serverSetting) map[string]savedCredential {
	return serverstore.EffectiveCredentials(settings)
}
func serverInfos(hosts []string, settings map[string]serverSetting, credentials map[string]savedCredential) []serverInfo {
	return serverstore.Infos(hosts, settings, credentials)
}
func configuredHost(host string) bool { return serverstore.ConfiguredHost(host) }

func (a *App) saveServer(w http.ResponseWriter, r *http.Request) {
	host := r.PathValue("host")
	if !configuredHost(host) {
		jsonError(w, 400, fmt.Errorf("Chọn một máy chủ trong SSH config."))
		return
	}
	var req struct {
		Password *string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, 400, err)
		return
	}
	if req.Password == nil {
		jsonError(w, 400, fmt.Errorf("Nhập mật khẩu mới hoặc chuỗi rỗng để ngừng dùng mật khẩu đã lưu."))
		return
	}
	if err := updateSetting(host, func(s *serverSetting) { s.Password = req.Password }); err != nil {
		jsonError(w, 500, err)
		return
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}

func (a *App) checkServerKey(w http.ResponseWriter, r *http.Request) {
	if !configuredHost(r.PathValue("host")) {
		jsonError(w, 400, fmt.Errorf("Chọn một máy chủ trong SSH config."))
		return
	}
	if err := serverstore.CheckKey(r.Context(), r.PathValue("host")); err != nil {
		jsonError(w, 400, err)
		return
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}

type connectionCheck = serverstore.ConnectionCheck

func (a *App) checkServerConnection(w http.ResponseWriter, r *http.Request) {
	if !configuredHost(r.PathValue("host")) {
		jsonError(w, 400, fmt.Errorf("Chọn một máy chủ trong SSH config."))
		return
	}
	result, err := serverstore.CheckConnection(r.Context(), r.PathValue("host"))
	if err != nil {
		jsonError(w, 500, err)
		return
	}
	jsonResponse(w, 200, result)
}
