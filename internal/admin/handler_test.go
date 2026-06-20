package admin

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/app"
	"github.com/John-Robertt/subconverter/internal/auth"
	"github.com/John-Robertt/subconverter/internal/generate"
)

// T-ADM-017: setup token validation (missing/wrong/correct)
// T-ADM-018: auth state does not store plaintext password or raw session token
// T-ADM-019: setup sets HttpOnly session cookie
// T-ADM-012: protected endpoints require valid session; bearer token does not grant admin access
// T-ADM-009: GET /api/config returns config_revision
// T-ADM-023: effective YAML export and YAML import require session and preserve config shape
// T-PRV-013: missing base_url returns 400 base_url_required
func TestAuthSetupAndProtectedConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(appTestConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	appSvc, err := app.New(context.Background(), app.Options{ConfigLocation: configPath})
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	authSvc, err := auth.New(auth.Options{
		StatePath:  filepath.Join(dir, "auth.json"),
		SetupToken: "setup-secret",
		Logger:     log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	handler := New(appSvc, authSvc)

	statusResp := serve(handler, http.MethodGet, "/api/auth/status", "", nil)
	if statusResp.Code != http.StatusOK || !strings.Contains(statusResp.Body.String(), `"setup_required":true`) {
		t.Fatalf("status = %d body = %s", statusResp.Code, statusResp.Body.String())
	}

	missingToken := serve(handler, http.MethodPost, "/api/auth/setup", `{"username":"admin","password":"long-enough-password"}`, nil)
	if missingToken.Code != http.StatusUnauthorized || !strings.Contains(missingToken.Body.String(), "setup_token_required") {
		t.Fatalf("missing token status = %d body = %s", missingToken.Code, missingToken.Body.String())
	}

	missingLoginField := serve(handler, http.MethodPost, "/api/auth/login", `{"username":"admin"}`, nil)
	if missingLoginField.Code != http.StatusBadRequest || !strings.Contains(missingLoginField.Body.String(), "invalid_request") {
		t.Fatalf("missing login field status = %d body = %s", missingLoginField.Code, missingLoginField.Body.String())
	}

	setup := serve(handler, http.MethodPost, "/api/auth/setup", `{"username":"admin","password":"long-enough-password","setup_token":"setup-secret"}`, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup status = %d body = %s", setup.Code, setup.Body.String())
	}
	cookies := setup.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != auth.SessionCookieName || !cookies[0].HttpOnly {
		t.Fatalf("setup cookies = %+v", cookies)
	}
	stateBytes, err := os.ReadFile(filepath.Join(dir, "auth.json")) // #nosec G304 -- dir is created by t.TempDir in this test.
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stateBytes, []byte("long-enough-password")) || bytes.Contains(stateBytes, []byte(cookies[0].Value)) {
		t.Fatalf("auth state leaked plaintext secret: %s", stateBytes)
	}

	noCookie := serve(handler, http.MethodGet, "/api/config", "", nil)
	if noCookie.Code != http.StatusUnauthorized || !strings.Contains(noCookie.Body.String(), "auth_required") {
		t.Fatalf("no cookie status = %d body = %s", noCookie.Code, noCookie.Body.String())
	}
	noCookieExport := serve(handler, http.MethodGet, "/api/config/effective.yaml", "", nil)
	if noCookieExport.Code != http.StatusUnauthorized || !strings.Contains(noCookieExport.Body.String(), "auth_required") {
		t.Fatalf("no-cookie export status = %d body = %s", noCookieExport.Code, noCookieExport.Body.String())
	}
	noCookieArchive := serve(handler, http.MethodGet, "/api/config/effective.zip", "", nil)
	if noCookieArchive.Code != http.StatusUnauthorized || !strings.Contains(noCookieArchive.Body.String(), "auth_required") {
		t.Fatalf("no-cookie archive status = %d body = %s", noCookieArchive.Code, noCookieArchive.Body.String())
	}
	bearerOnly := serve(handler, http.MethodGet, "/api/config", "", map[string]string{"Authorization": "Bearer subscription-token"})
	if bearerOnly.Code != http.StatusUnauthorized {
		t.Fatalf("bearer-only status = %d body = %s", bearerOnly.Code, bearerOnly.Body.String())
	}

	withCookie := serve(handler, http.MethodGet, "/api/config", "", map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value})
	if withCookie.Code != http.StatusOK || !strings.Contains(withCookie.Body.String(), `"config_revision"`) {
		t.Fatalf("with cookie status = %d body = %s", withCookie.Code, withCookie.Body.String())
	}
	effective := serve(handler, http.MethodGet, "/api/config/effective.yaml", "", map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value})
	if effective.Code != http.StatusOK || !strings.Contains(effective.Body.String(), "fallback: proxy") {
		t.Fatalf("effective config status = %d body = %s", effective.Code, effective.Body.String())
	}
	if got := effective.Header().Get("Content-Type"); got != "application/x-yaml; charset=utf-8" {
		t.Fatalf("effective content type = %q", got)
	}
	if got := effective.Header().Get("Content-Disposition"); got != `attachment; filename="config.yaml"; filename*=UTF-8''config.yaml` {
		t.Fatalf("effective content disposition = %q", got)
	}
	archive := serve(handler, http.MethodGet, "/api/config/effective.zip", "", map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value})
	if archive.Code != http.StatusOK {
		t.Fatalf("effective archive status = %d body = %s", archive.Code, archive.Body.String())
	}
	if got := archive.Header().Get("Content-Type"); got != "application/zip" {
		t.Fatalf("archive content type = %q", got)
	}
	if got := archive.Header().Get("Content-Disposition"); got != `attachment; filename="subconverter-config.zip"; filename*=UTF-8''subconverter-config.zip` {
		t.Fatalf("archive content disposition = %q", got)
	}
	if entries := adminArchiveEntries(t, archive.Body.Bytes()); !strings.Contains(string(entries["config.yaml"]), "fallback: proxy") {
		t.Fatalf("archive config.yaml missing fallback: %q", entries["config.yaml"])
	}
	importBody, err := json.Marshal(map[string]string{"yaml": appTestConfigYAML})
	if err != nil {
		t.Fatal(err)
	}
	imported := serve(handler, http.MethodPost, "/api/config/import", string(importBody), map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value})
	if imported.Code != http.StatusOK || !strings.Contains(imported.Body.String(), `"groups":[{"key":"HK"`) {
		t.Fatalf("import config status = %d body = %s", imported.Code, imported.Body.String())
	}
	importArchiveBody := adminArchive(t, map[string]string{
		"config.yaml":          appTestConfigYAML,
		"templates/clash.yaml": "mixed-port: 7890\n",
		"templates/surge.conf": "[General]\nloglevel = notify\n",
	})
	if err := os.Mkdir(filepath.Join(dir, "templates"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "templates", "clash.yaml"), []byte("old clash\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	importedArchive := serve(handler, http.MethodPost, "/api/config/import", string(importArchiveBody), map[string]string{
		"Cookie":       auth.SessionCookieName + "=" + cookies[0].Value,
		"Content-Type": "application/zip",
	})
	if importedArchive.Code != http.StatusOK || !strings.Contains(importedArchive.Body.String(), `"templates"`) {
		t.Fatalf("import archive status = %d body = %s", importedArchive.Code, importedArchive.Body.String())
	}
	importedArchiveConfig := adminImportedArchiveConfig(t, importedArchive.Body.Bytes())
	if !strings.HasPrefix(importedArchiveConfig.Templates.Clash, filepath.Join(dir, ".imports", "import-")) {
		t.Fatalf("imported clash template path = %q", importedArchiveConfig.Templates.Clash)
	}
	if got, err := os.ReadFile(importedArchiveConfig.Templates.Clash); err != nil || string(got) != "mixed-port: 7890\n" {
		t.Fatalf("imported clash template = %q err=%v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "templates", "clash.yaml")); err != nil || string(got) != "old clash\n" { // #nosec G304 -- path is built under t.TempDir in this test.
		t.Fatalf("existing clash template = %q err=%v", got, err)
	}
	missingBaseURL := serve(handler, http.MethodGet, "/api/generate/link?format=surge", "", map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value})
	if missingBaseURL.Code != http.StatusBadRequest || !strings.Contains(missingBaseURL.Body.String(), "base_url_required") {
		t.Fatalf("missing base_url link status = %d body = %s", missingBaseURL.Code, missingBaseURL.Body.String())
	}
}

// T-ADM-024: ZIP import template write failures return the template-specific conflict code
func TestConfigArchiveImportTemplateWriteFailureReturnsTemplateError(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(appTestConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".imports"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	appSvc, err := app.New(context.Background(), app.Options{ConfigLocation: configPath})
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	authSvc, err := auth.New(auth.Options{
		StatePath:  filepath.Join(dir, "auth.json"),
		SetupToken: "setup-secret",
		Logger:     log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	handler := New(appSvc, authSvc)
	setup := serve(handler, http.MethodPost, "/api/auth/setup", `{"username":"admin","password":"long-enough-password","setup_token":"setup-secret"}`, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup status = %d body = %s", setup.Code, setup.Body.String())
	}
	headers := map[string]string{
		"Cookie":       auth.SessionCookieName + "=" + setup.Result().Cookies()[0].Value,
		"Content-Type": "application/zip",
	}
	archiveBody := adminArchive(t, map[string]string{
		"config.yaml":          appTestConfigYAML,
		"templates/clash.yaml": "mixed-port: 7890\n",
	})

	resp := serve(handler, http.MethodPost, "/api/config/import", string(archiveBody), headers)
	if resp.Code != http.StatusConflict || !strings.Contains(resp.Body.String(), "template_file_not_writable") {
		t.Fatalf("template write failure status = %d body = %s", resp.Code, resp.Body.String())
	}
}

// T-ADM-012: status/preview/link require session
// T-PRV-004: status returns version and config_write capability
// T-PRV-001: preview nodes returns node list with name and active_count
// T-PRV-013: generate link returns URL with token_included
// T-PRV-003: generate preview output matches generate output (no Content-Disposition)
func TestM7EndpointsRequireSessionAndReturnExpectedShapes(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(appTestM7ConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	appSvc, err := app.New(context.Background(), app.Options{
		ConfigLocation: configPath,
		Fetcher: &adminMapFetcher{responses: map[string][]byte{
			"https://sub.example.com/api": []byte(base64.StdEncoding.EncodeToString([]byte("ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01"))),
		}},
		Generate: generate.Options{AccessToken: "server-token"},
		Version:  "2.0.0",
		Commit:   "abc123",
	})
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	authSvc, err := auth.New(auth.Options{
		StatePath:  filepath.Join(dir, "auth.json"),
		SetupToken: "setup-secret",
		Logger:     log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	handler := New(appSvc, authSvc)

	noCookie := serve(handler, http.MethodGet, "/api/status", "", nil)
	if noCookie.Code != http.StatusUnauthorized || !strings.Contains(noCookie.Body.String(), "auth_required") {
		t.Fatalf("no-cookie status = %d body = %s", noCookie.Code, noCookie.Body.String())
	}

	setup := serve(handler, http.MethodPost, "/api/auth/setup", `{"username":"admin","password":"long-enough-password","setup_token":"setup-secret"}`, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup status = %d body = %s", setup.Code, setup.Body.String())
	}
	cookie := setup.Result().Cookies()[0]
	headers := map[string]string{"Cookie": auth.SessionCookieName + "=" + cookie.Value}

	status := serve(handler, http.MethodGet, "/api/status", "", headers)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"version":"2.0.0"`) || !strings.Contains(status.Body.String(), `"config_write":true`) {
		t.Fatalf("status endpoint = %d body = %s", status.Code, status.Body.String())
	}

	nodes := serve(handler, http.MethodGet, "/api/preview/nodes", "", headers)
	if nodes.Code != http.StatusOK || !strings.Contains(nodes.Body.String(), `"name":"HK-01"`) || !strings.Contains(nodes.Body.String(), `"active_count":1`) {
		t.Fatalf("nodes endpoint = %d body = %s", nodes.Code, nodes.Body.String())
	}

	link := serve(handler, http.MethodGet, "/api/generate/link?format=surge&filename=phone.conf", "", headers)
	if link.Code != http.StatusOK || !strings.Contains(link.Body.String(), `token_included":true`) || !strings.Contains(link.Body.String(), `token=server-token`) {
		t.Fatalf("link endpoint = %d body = %s", link.Code, link.Body.String())
	}

	preview := serve(handler, http.MethodGet, "/api/generate/preview?format=clash", "", headers)
	if preview.Code != http.StatusOK {
		t.Fatalf("generate preview = %d body = %s", preview.Code, preview.Body.String())
	}
	if preview.Header().Get("Content-Disposition") != "" {
		t.Fatalf("generate preview should not set Content-Disposition, got %q", preview.Header().Get("Content-Disposition"))
	}
	if !strings.Contains(preview.Body.String(), "HK-01") {
		t.Fatalf("generate preview body missing node: %s", preview.Body.String())
	}
	generated, err := appSvc.Generate(context.Background(), generate.Request{Format: "clash", Filename: "clash.yaml"})
	if err != nil {
		t.Fatalf("app Generate: %v", err)
	}
	if preview.Body.String() != string(generated.Body) {
		t.Fatalf("generate preview body differs from generate output\npreview:\n%s\ngenerate:\n%s", preview.Body.String(), generated.Body)
	}
}

// T-ADM-022: CSRF same-origin check covers scheme, host, and port matching
func TestSameOriginChecksSchemeAndHost(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		headers map[string]string
		want    bool
	}{
		{
			name:   "https request accepts matching https origin",
			target: "https://example.test/api/auth/setup",
			headers: map[string]string{
				"Origin": "https://example.test",
			},
			want: true,
		},
		{
			name:   "https request rejects same host with http origin",
			target: "https://example.test/api/auth/setup",
			headers: map[string]string{
				"Origin": "http://example.test",
			},
			want: false,
		},
		{
			name:   "forwarded https scheme accepts matching origin",
			target: "http://example.test/api/auth/setup",
			headers: map[string]string{
				"Origin":            "https://example.test",
				"X-Forwarded-Proto": "https,http",
			},
			want: true,
		},
		{
			name:   "host comparison preserves port",
			target: "http://localhost:8080/api/auth/setup",
			headers: map[string]string{
				"Origin": "http://localhost:8080",
			},
			want: true,
		},
		{
			name:   "port mismatch is rejected",
			target: "http://localhost:8080/api/auth/setup",
			headers: map[string]string{
				"Origin": "http://localhost",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.target, nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			if got := sameOrigin(req); got != tt.want {
				t.Fatalf("sameOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

type adminMapFetcher struct {
	responses map[string][]byte
}

func (f *adminMapFetcher) Fetch(_ context.Context, rawURL string) ([]byte, error) {
	return append([]byte(nil), f.responses[rawURL]...), nil
}

func serve(handler http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "http://example.test"+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		req.Header.Set("Origin", "http://example.test")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func adminArchive(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create archive entry %q: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write archive entry %q: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return buf.Bytes()
}

func adminArchiveEntries(t *testing.T, body []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	entries := make(map[string][]byte, len(zr.File))
	for _, file := range zr.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open archive entry %q: %v", file.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read archive entry %q: %v", file.Name, err)
		}
		entries[file.Name] = data
	}
	return entries
}

func adminImportedArchiveConfig(t *testing.T, body []byte) struct {
	Templates struct {
		Clash string `json:"clash"`
		Surge string `json:"surge"`
	} `json:"templates"`
} {
	t.Helper()
	var envelope struct {
		Config struct {
			Templates struct {
				Clash string `json:"clash"`
				Surge string `json:"surge"`
			} `json:"templates"`
		} `json:"config"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("unmarshal imported archive response: %v", err)
	}
	return envelope.Config
}

const appTestConfigYAML = `
sources: {}
filters: {}
groups:
  HK:
    match: "(HK)"
    strategy: select
routing:
  proxy:
    - HK
    - DIRECT
rulesets: {}
rules: []
fallback: proxy
`

const appTestM7ConfigYAML = `
base_url: https://example.com
sources:
  subscriptions:
    - url: "https://sub.example.com/api"
filters: {}
groups:
  HK:
    match: "(HK)"
    strategy: select
routing:
  proxy:
    - HK
    - DIRECT
rulesets: {}
rules: []
fallback: proxy
`
