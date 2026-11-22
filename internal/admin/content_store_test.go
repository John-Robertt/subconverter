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

// T-ADM-STORE-001：正文目录只读时返回指明路径的错误，且不改变配置的写入能力。
func TestContentStore_DirectoryErrorAndRetry(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	dataDir := filepath.Join(dir, "versions")
	if err := os.WriteFile(configPath, []byte(appTestConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dataDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dataDir, 0o700) // #nosec G302 -- 临时目录需要属主的执行权限。
	})
	svc, err := app.New(context.Background(), app.Options{ConfigLocation: configPath, DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	authSvc, err := auth.New(auth.Options{StatePath: filepath.Join(dir, "auth.json"), SetupToken: "setup-secret", Logger: log.New(io.Discard, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	handler := New(svc, authSvc)
	setup := serve(handler, http.MethodPost, "/api/auth/setup", `{"username":"admin","password":"long-enough-password","setup_token":"setup-secret"}`, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup 失败：%d", setup.Code)
	}
	headers := map[string]string{"Cookie": auth.SessionCookieName + "=" + setup.Result().Cookies()[0].Value}
	snapshot := serve(handler, http.MethodGet, "/api/config", "", headers)
	var input struct {
		Revision string         `json:"config_revision"`
		Config   map[string]any `json:"config"`
	}
	if err := json.Unmarshal(snapshot.Body.Bytes(), &input); err != nil {
		t.Fatal(err)
	}
	sources := input.Config["sources"].(map[string]any)
	sources["subscriptions"] = []any{map[string]any{"content": "anytls://secret-value@hk.example.com:443#HK"}}
	body, err := json.Marshal(map[string]any{"config_revision": input.Revision, "config": input.Config})
	if err != nil {
		t.Fatal(err)
	}
	failed := serve(handler, http.MethodPut, "/api/config", string(body), headers)
	if failed.Code != http.StatusConflict || !strings.Contains(failed.Body.String(), "resource_content_write_failed") || !strings.Contains(failed.Body.String(), dataDir) || strings.Contains(failed.Body.String(), "secret-value") {
		t.Fatalf("存储失败的响应不符合预期：%d %s", failed.Code, failed.Body.String())
	}
	status := serve(handler, http.MethodGet, "/api/status", "", headers)
	if !strings.Contains(status.Body.String(), `"config_write":true`) || !strings.Contains(status.Body.String(), `"writable":true`) {
		t.Fatalf("正文写入错误改变了配置的可写状态：%s", status.Body.String())
	}
	if err := os.Chmod(dataDir, 0o700); err != nil { // #nosec G302 -- 恢复属主对临时目录的访问权限。
		t.Fatal(err)
	}
	retry := serve(handler, http.MethodPut, "/api/config", string(body), headers)
	if retry.Code != http.StatusOK {
		t.Fatalf("重试失败：%d %s", retry.Code, retry.Body.String())
	}
}
