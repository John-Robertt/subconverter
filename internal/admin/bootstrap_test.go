package admin

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/app"
	"github.com/John-Robertt/subconverter/internal/auth"
)

// T-ADM-BOOT-001：即使没有主配置，setup、导入、首次保存和 reload 仍然需要认证。
func TestAdmin_BootstrapConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	svc, err := app.New(context.Background(), app.Options{ConfigLocation: path, DataDir: filepath.Join(dir, "data")})
	if err != nil {
		t.Fatal(err)
	}
	authSvc, err := auth.New(auth.Options{StatePath: filepath.Join(dir, "auth.json"), SetupToken: "setup-secret", Logger: log.New(io.Discard, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	handler := New(svc, authSvc)
	anonymous := serve(handler, http.MethodGet, "/api/config", "", nil)
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("初始化阶段在没有 session 时泄露了配置：%d", anonymous.Code)
	}
	setup := serve(handler, http.MethodPost, "/api/auth/setup", `{"username":"admin","password":"long-enough-password","setup_token":"setup-secret"}`, nil)
	if setup.Code != http.StatusOK || len(setup.Result().Cookies()) == 0 {
		t.Fatalf("setup 状态码=%d 响应体=%s", setup.Code, setup.Body.String())
	}
	headers := map[string]string{"Cookie": auth.SessionCookieName + "=" + setup.Result().Cookies()[0].Value}
	status := serve(handler, http.MethodGet, "/api/status", "", headers)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"exists":false`) || !strings.Contains(status.Body.String(), `"config_write":true`) {
		t.Fatalf("setup 后状态接口 状态码=%d 响应体=%s", status.Code, status.Body.String())
	}
	snapshot := serve(handler, http.MethodGet, "/api/config", "", headers)
	var baseline struct {
		ConfigRevision string `json:"config_revision"`
	}
	if err := json.Unmarshal(snapshot.Body.Bytes(), &baseline); err != nil {
		t.Fatal(err)
	}
	if snapshot.Code != http.StatusOK || baseline.ConfigRevision == "" {
		t.Fatalf("空白草稿 状态码=%d 响应体=%s", snapshot.Code, snapshot.Body.String())
	}
	for _, endpoint := range []string{"/api/config/saved.zip", "/api/config/effective.yaml", "/api/config/effective.zip", "/api/preview/nodes", "/api/preview/groups", "/api/generate/preview?format=clash", "/api/generate/link?format=clash"} {
		response := serve(handler, http.MethodGet, endpoint, "", headers)
		if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"config_not_initialized"`) {
			t.Fatalf("%s 状态码=%d 响应体=%s", endpoint, response.Code, response.Body.String())
		}
	}
	const raw = "sources:\n  subscriptions:\n    - {format: text, content_file: subscriptions/1.txt}\ngroups:\n  HK: {match: HK, strategy: select}\nrouting:\n  proxy: [HK, DIRECT]\nfallback: proxy\n"
	archive := adminArchive(t, map[string]string{"config.yaml": raw, "subscriptions/1.txt": "anytls://pw@hk.example.com:443#HK_BOOT"})
	zipHeaders := map[string]string{"Cookie": headers["Cookie"], "Content-Type": "application/zip"}
	imported := serve(handler, http.MethodPost, "/api/config/import", string(archive), zipHeaders)
	if imported.Code != http.StatusOK {
		t.Fatalf("首次 ZIP 导入 状态码=%d 响应体=%s", imported.Code, imported.Body.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("导入操作创建了主配置文件：%v", err)
	}
	var draft struct {
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(imported.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"config_revision": baseline.ConfigRevision, "config": draft.Config})
	if err != nil {
		t.Fatal(err)
	}
	saved := serve(handler, http.MethodPut, "/api/config", string(body), headers)
	if saved.Code != http.StatusOK {
		t.Fatalf("首次保存 状态码=%d 响应体=%s", saved.Code, saved.Body.String())
	}
	reloaded := serve(handler, http.MethodPost, "/api/reload", "", headers)
	if reloaded.Code != http.StatusOK {
		t.Fatalf("首次 reload 状态码=%d 响应体=%s", reloaded.Code, reloaded.Body.String())
	}
	for _, format := range []string{"clash", "surge", "loon"} {
		preview := serve(handler, http.MethodGet, "/api/generate/preview?format="+format, "", headers)
		if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), "HK_BOOT") {
			t.Fatalf("%s 状态码=%d 响应体=%s", format, preview.Code, preview.Body.String())
		}
	}
}
