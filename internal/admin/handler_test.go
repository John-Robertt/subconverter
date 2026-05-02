package admin

import (
	"bytes"
	"context"
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
