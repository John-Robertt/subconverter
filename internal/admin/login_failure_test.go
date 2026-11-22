package admin

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/John-Robertt/subconverter/internal/auth"
)

// T-ADM-020：失败记录过期后，HTTP 层的剩余尝试次数恢复，401/423 响应字段保持不变。
func TestAuth_LoginFailureExpiry(t *testing.T) {
	now := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	svc, err := auth.New(auth.Options{
		StatePath: filepath.Join(t.TempDir(), "auth.json"), SetupToken: "setup-secret",
		Logger: log.New(io.Discard, "", 0), Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := New(nil, svc)
	setup := serve(handler, http.MethodPost, "/api/auth/setup", `{"username":"admin","password":"long-enough-password","setup_token":"setup-secret"}`, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup 状态码=%d 响应体=%s", setup.Code, setup.Body.String())
	}
	fail := func(wantStatus int, wantCode string, wantRemaining int) string {
		t.Helper()
		response := serve(handler, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"wrong-password"}`, nil)
		var result struct {
			Error struct {
				Code      string `json:"code"`
				Remaining int    `json:"remaining"`
				Until     string `json:"until"`
			} `json:"error"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if response.Code != wantStatus || result.Error.Code != wantCode || result.Error.Remaining != wantRemaining {
			t.Fatalf("状态码=%d 响应体=%s，期望 状态码=%d code=%s remaining=%d", response.Code, response.Body.String(), wantStatus, wantCode, wantRemaining)
		}
		return result.Error.Until
	}
	fail(http.StatusUnauthorized, "invalid_credentials", 4)
	fail(http.StatusUnauthorized, "invalid_credentials", 3)
	now = now.Add(15 * time.Minute)
	fail(http.StatusUnauthorized, "invalid_credentials", 4)
	for remaining := 3; remaining > 0; remaining-- {
		fail(http.StatusUnauthorized, "invalid_credentials", remaining)
	}
	until := fail(http.StatusLocked, "auth_locked", 0)
	if want := now.Add(15 * time.Minute).Format(time.RFC3339); until != want {
		t.Fatalf("锁定截止时间=%q，期望 %q", until, want)
	}
	now = now.Add(15 * time.Minute)
	status := serve(handler, http.MethodGet, "/api/auth/status", "", nil)
	var state struct {
		LockedUntil string `json:"locked_until"`
	}
	if err := json.Unmarshal(status.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if status.Code != http.StatusOK || state.LockedUntil != "" {
		t.Fatalf("锁定过期后 状态码=%d 响应体=%s", status.Code, status.Body.String())
	}
	fail(http.StatusUnauthorized, "invalid_credentials", 4)
	login := serve(handler, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"long-enough-password"}`, nil)
	if login.Code != http.StatusOK || len(login.Result().Cookies()) == 0 {
		t.Fatalf("正确密码登录 状态码=%d 响应体=%s", login.Code, login.Body.String())
	}
	fail(http.StatusUnauthorized, "invalid_credentials", 4)
}
