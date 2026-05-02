package admin

import (
	"bytes"
	"context"
	"encoding/base64"
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
	stateBytes, err := os.ReadFile(filepath.Join(dir, "auth.json"))
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
	bearerOnly := serve(handler, http.MethodGet, "/api/config", "", map[string]string{"Authorization": "Bearer subscription-token"})
	if bearerOnly.Code != http.StatusUnauthorized {
		t.Fatalf("bearer-only status = %d body = %s", bearerOnly.Code, bearerOnly.Body.String())
	}

	withCookie := serve(handler, http.MethodGet, "/api/config", "", map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value})
	if withCookie.Code != http.StatusOK || !strings.Contains(withCookie.Body.String(), `"config_revision"`) {
		t.Fatalf("with cookie status = %d body = %s", withCookie.Code, withCookie.Body.String())
	}
	missingBaseURL := serve(handler, http.MethodGet, "/api/generate/link?format=surge", "", map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value})
	if missingBaseURL.Code != http.StatusBadRequest || !strings.Contains(missingBaseURL.Body.String(), "base_url_required") {
		t.Fatalf("missing base_url link status = %d body = %s", missingBaseURL.Code, missingBaseURL.Body.String())
	}
}

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
