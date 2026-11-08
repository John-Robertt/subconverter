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

// T-ADM-BOOT-001: setup/import/first-save/reload remains authenticated even without a main config.
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
		t.Fatalf("bootstrap leaked config without a session: %d", anonymous.Code)
	}
	setup := serve(handler, http.MethodPost, "/api/auth/setup", `{"username":"admin","password":"long-enough-password","setup_token":"setup-secret"}`, nil)
	if setup.Code != http.StatusOK || len(setup.Result().Cookies()) == 0 {
		t.Fatalf("setup status=%d body=%s", setup.Code, setup.Body.String())
	}
	headers := map[string]string{"Cookie": auth.SessionCookieName + "=" + setup.Result().Cookies()[0].Value}
	status := serve(handler, http.MethodGet, "/api/status", "", headers)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"exists":false`) || !strings.Contains(status.Body.String(), `"config_write":true`) {
		t.Fatalf("setup status=%d body=%s", status.Code, status.Body.String())
	}
	snapshot := serve(handler, http.MethodGet, "/api/config", "", headers)
	var baseline struct {
		ConfigRevision string `json:"config_revision"`
	}
	if err := json.Unmarshal(snapshot.Body.Bytes(), &baseline); err != nil {
		t.Fatal(err)
	}
	if snapshot.Code != http.StatusOK || baseline.ConfigRevision == "" {
		t.Fatalf("blank draft status=%d body=%s", snapshot.Code, snapshot.Body.String())
	}
	for _, endpoint := range []string{"/api/config/saved.zip", "/api/config/effective.yaml", "/api/config/effective.zip", "/api/preview/nodes", "/api/preview/groups", "/api/generate/preview?format=clash", "/api/generate/link?format=clash"} {
		response := serve(handler, http.MethodGet, endpoint, "", headers)
		if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"config_not_initialized"`) {
			t.Fatalf("%s status=%d body=%s", endpoint, response.Code, response.Body.String())
		}
	}
	const raw = "sources:\n  subscriptions:\n    - {format: text, content_file: subscriptions/1.txt}\ngroups:\n  HK: {match: HK, strategy: select}\nrouting:\n  proxy: [HK, DIRECT]\nfallback: proxy\n"
	archive := adminArchive(t, map[string]string{"config.yaml": raw, "subscriptions/1.txt": "anytls://pw@hk.example.com:443#HK_BOOT"})
	zipHeaders := map[string]string{"Cookie": headers["Cookie"], "Content-Type": "application/zip"}
	imported := serve(handler, http.MethodPost, "/api/config/import", string(archive), zipHeaders)
	if imported.Code != http.StatusOK {
		t.Fatalf("first ZIP import status=%d body=%s", imported.Code, imported.Body.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("import created the main file: %v", err)
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
		t.Fatalf("first save status=%d body=%s", saved.Code, saved.Body.String())
	}
	reloaded := serve(handler, http.MethodPost, "/api/reload", "", headers)
	if reloaded.Code != http.StatusOK {
		t.Fatalf("first reload status=%d body=%s", reloaded.Code, reloaded.Body.String())
	}
	for _, format := range []string{"clash", "surge", "loon"} {
		preview := serve(handler, http.MethodGet, "/api/generate/preview?format="+format, "", headers)
		if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), "HK_BOOT") {
			t.Fatalf("%s status=%d body=%s", format, preview.Code, preview.Body.String())
		}
	}
}
