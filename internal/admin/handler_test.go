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
	"sync"
	"testing"
	"time"

	"github.com/John-Robertt/subconverter/internal/app"
	"github.com/John-Robertt/subconverter/internal/auth"
	"github.com/John-Robertt/subconverter/internal/generate"
)

// T-ADM-017：校验 setup token（缺失、错误、正确）
// T-ADM-018：认证状态不保存明文密码或原始 session token
// T-ADM-019：setup 设置 HttpOnly 的 session cookie
// T-ADM-012：受保护接口需要有效 session；bearer token 不授予管理员权限
// T-ADM-009：GET /api/config 返回 config_revision
// T-ADM-023：生效 YAML 导出与 YAML 导入需要 session，并保持配置结构不变
// T-PRV-013：缺少 base_url 时返回 400 base_url_required
func TestAuthSetupAndProtectedConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(appTestConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	appSvc, err := app.New(context.Background(), app.Options{ConfigLocation: configPath})
	if err != nil {
		t.Fatalf("app.New 失败：%v", err)
	}
	authSvc, err := auth.New(auth.Options{
		StatePath:  filepath.Join(dir, "auth.json"),
		SetupToken: "setup-secret",
		Logger:     log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("auth.New 失败：%v", err)
	}
	handler := New(appSvc, authSvc)

	statusResp := serve(handler, http.MethodGet, "/api/auth/status", "", nil)
	if statusResp.Code != http.StatusOK || !strings.Contains(statusResp.Body.String(), `"setup_required":true`) {
		t.Fatalf("状态码 = %d 响应体 = %s", statusResp.Code, statusResp.Body.String())
	}

	missingToken := serve(handler, http.MethodPost, "/api/auth/setup", `{"username":"admin","password":"long-enough-password"}`, nil)
	if missingToken.Code != http.StatusUnauthorized || !strings.Contains(missingToken.Body.String(), "setup_token_required") {
		t.Fatalf("缺少 token 时 状态码 = %d 响应体 = %s", missingToken.Code, missingToken.Body.String())
	}

	missingLoginField := serve(handler, http.MethodPost, "/api/auth/login", `{"username":"admin"}`, nil)
	if missingLoginField.Code != http.StatusBadRequest || !strings.Contains(missingLoginField.Body.String(), "invalid_request") {
		t.Fatalf("缺少登录字段时 状态码 = %d 响应体 = %s", missingLoginField.Code, missingLoginField.Body.String())
	}

	setup := serve(handler, http.MethodPost, "/api/auth/setup", `{"username":"admin","password":"long-enough-password","setup_token":"setup-secret"}`, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup 状态码 = %d 响应体 = %s", setup.Code, setup.Body.String())
	}
	cookies := setup.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != auth.SessionCookieName || !cookies[0].HttpOnly {
		t.Fatalf("setup 返回的 cookie = %+v", cookies)
	}
	stateBytes, err := os.ReadFile(filepath.Join(dir, "auth.json")) // #nosec G304 -- dir 由本测试中的 t.TempDir 创建。
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stateBytes, []byte("long-enough-password")) || bytes.Contains(stateBytes, []byte(cookies[0].Value)) {
		t.Fatalf("认证状态泄露了明文密钥：%s", stateBytes)
	}

	noCookie := serve(handler, http.MethodGet, "/api/config", "", nil)
	if noCookie.Code != http.StatusUnauthorized || !strings.Contains(noCookie.Body.String(), "auth_required") {
		t.Fatalf("无 cookie 时 状态码 = %d 响应体 = %s", noCookie.Code, noCookie.Body.String())
	}
	noCookieExport := serve(handler, http.MethodGet, "/api/config/effective.yaml", "", nil)
	if noCookieExport.Code != http.StatusUnauthorized || !strings.Contains(noCookieExport.Body.String(), "auth_required") {
		t.Fatalf("无 cookie 导出 状态码 = %d 响应体 = %s", noCookieExport.Code, noCookieExport.Body.String())
	}
	noCookieArchive := serve(handler, http.MethodGet, "/api/config/effective.zip", "", nil)
	if noCookieArchive.Code != http.StatusUnauthorized || !strings.Contains(noCookieArchive.Body.String(), "auth_required") {
		t.Fatalf("无 cookie 导出压缩包 状态码 = %d 响应体 = %s", noCookieArchive.Code, noCookieArchive.Body.String())
	}
	noCookieSavedArchive := serve(handler, http.MethodGet, "/api/config/saved.zip", "", nil)
	if noCookieSavedArchive.Code != http.StatusUnauthorized {
		t.Fatalf("无 cookie 导出已保存压缩包 状态码 = %d，期望 401", noCookieSavedArchive.Code)
	}
	bearerOnly := serve(handler, http.MethodGet, "/api/config", "", map[string]string{"Authorization": "Bearer subscription-token"})
	if bearerOnly.Code != http.StatusUnauthorized {
		t.Fatalf("仅带 bearer 时 状态码 = %d 响应体 = %s", bearerOnly.Code, bearerOnly.Body.String())
	}

	// T-ADM-INLINE-001：读取资源需要 session，且请求必须同源。
	resourceRequest, err := json.Marshal(map[string]string{"location": configPath})
	if err != nil {
		t.Fatal(err)
	}
	denied := serve(handler, http.MethodPost, "/api/resources/read", string(resourceRequest), nil)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("无 session 读取资源：%d", denied.Code)
	}
	resource := serve(handler, http.MethodPost, "/api/resources/read", string(resourceRequest), map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value})
	if resource.Code != http.StatusOK || !strings.Contains(resource.Body.String(), "content") {
		t.Fatalf("读取资源：%d %s", resource.Code, resource.Body.String())
	}
	crossOrigin := serve(handler, http.MethodPost, "/api/resources/read", string(resourceRequest), map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value, "Origin": "https://untrusted.example"})
	if crossOrigin.Code != http.StatusForbidden {
		t.Fatalf("跨源读取资源：%d", crossOrigin.Code)
	}
	// T-ADM-CHECK-001：订阅检查需要 session，遵守 CSRF 防护，并返回解析结果。
	checkBody := `{"content":"anytls://pw@hk.example.com:443#HK"}`
	checkDenied := serve(handler, http.MethodPost, "/api/subscriptions/check", checkBody, nil)
	if checkDenied.Code != http.StatusUnauthorized {
		t.Fatalf("匿名检查=%d", checkDenied.Code)
	}
	checkHeaders := map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value}
	checked := serve(handler, http.MethodPost, "/api/subscriptions/check", checkBody, checkHeaders)
	if checked.Code != http.StatusOK || !strings.Contains(checked.Body.String(), `"node_count":1`) || !strings.Contains(checked.Body.String(), `"format":"text"`) {
		t.Fatalf("检查=%d %s", checked.Code, checked.Body.String())
	}
	checkInvalid := serve(handler, http.MethodPost, "/api/subscriptions/check", `{"content":"<html>expired</html>"}`, checkHeaders)
	if checkInvalid.Code != http.StatusBadRequest {
		t.Fatalf("无效内容检查=%d %s", checkInvalid.Code, checkInvalid.Body.String())
	}
	checkHeaders["Origin"] = "https://untrusted.example"
	checkCross := serve(handler, http.MethodPost, "/api/subscriptions/check", checkBody, checkHeaders)
	if checkCross.Code != http.StatusForbidden {
		t.Fatalf("跨源检查=%d", checkCross.Code)
	}
	withCookie := serve(handler, http.MethodGet, "/api/config", "", map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value})
	if withCookie.Code != http.StatusOK || !strings.Contains(withCookie.Body.String(), `"config_revision"`) {
		t.Fatalf("带 cookie 时 状态码 = %d 响应体 = %s", withCookie.Code, withCookie.Body.String())
	}
	effective := serve(handler, http.MethodGet, "/api/config/effective.yaml", "", map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value})
	if effective.Code != http.StatusOK || !strings.Contains(effective.Body.String(), "fallback: proxy") {
		t.Fatalf("生效配置 状态码 = %d 响应体 = %s", effective.Code, effective.Body.String())
	}
	if got := effective.Header().Get("Content-Type"); got != "application/x-yaml; charset=utf-8" {
		t.Fatalf("生效配置的 Content-Type = %q", got)
	}
	if got := effective.Header().Get("Content-Disposition"); got != `attachment; filename="config.yaml"; filename*=UTF-8''config.yaml` {
		t.Fatalf("生效配置的 Content-Disposition = %q", got)
	}
	archive := serve(handler, http.MethodGet, "/api/config/effective.zip", "", map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value})
	if archive.Code != http.StatusOK {
		t.Fatalf("生效配置压缩包 状态码 = %d 响应体 = %s", archive.Code, archive.Body.String())
	}
	if got := archive.Header().Get("Content-Type"); got != "application/zip" {
		t.Fatalf("压缩包的 Content-Type = %q", got)
	}
	if got := archive.Header().Get("Content-Disposition"); got != `attachment; filename="subconverter-config.zip"; filename*=UTF-8''subconverter-config.zip` {
		t.Fatalf("压缩包的 Content-Disposition = %q", got)
	}
	if entries := adminArchiveEntries(t, archive.Body.Bytes()); !strings.Contains(string(entries["config.yaml"]), "fallback: proxy") {
		t.Fatalf("压缩包中的 config.yaml 缺少 fallback：%q", entries["config.yaml"])
	}
	importBody, err := json.Marshal(map[string]string{"yaml": appTestConfigYAML})
	if err != nil {
		t.Fatal(err)
	}
	imported := serve(handler, http.MethodPost, "/api/config/import", string(importBody), map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value})
	if imported.Code != http.StatusOK || !strings.Contains(imported.Body.String(), `"groups":[{"key":"HK"`) {
		t.Fatalf("导入配置 状态码 = %d 响应体 = %s", imported.Code, imported.Body.String())
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
		t.Fatalf("导入压缩包 状态码 = %d 响应体 = %s", importedArchive.Code, importedArchive.Body.String())
	}
	importedArchiveConfig := adminImportedArchiveConfig(t, importedArchive.Body.Bytes())
	if importedArchiveConfig.Templates.Clash != "" || importedArchiveConfig.Templates.Inline["clash"] != "mixed-port: 7890\n" {
		t.Fatalf("导入的模板 = %+v", importedArchiveConfig.Templates)
	}
	if _, err := os.Stat(filepath.Join(dir, ".imports")); !os.IsNotExist(err) {
		t.Fatalf("导入操作创建了隐藏目录：%v", err)
	}
	savedArchive := serve(handler, http.MethodGet, "/api/config/saved.zip", "", map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value})
	if savedArchive.Code != http.StatusOK || savedArchive.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("已保存压缩包 状态码=%d 响应体=%s", savedArchive.Code, savedArchive.Body.String())
	}
	if got, err := os.ReadFile(filepath.Join(dir, "templates", "clash.yaml")); err != nil || string(got) != "old clash\n" { // #nosec G304 -- 路径位于本测试的 t.TempDir 之下。
		t.Fatalf("已有的 clash 模板 = %q err=%v", got, err)
	}
	missingBaseURL := serve(handler, http.MethodGet, "/api/generate/link?format=surge", "", map[string]string{"Cookie": auth.SessionCookieName + "=" + cookies[0].Value})
	if missingBaseURL.Code != http.StatusBadRequest || !strings.Contains(missingBaseURL.Body.String(), "base_url_required") {
		t.Fatalf("缺少 base_url 时生成链接 状态码 = %d 响应体 = %s", missingBaseURL.Code, missingBaseURL.Body.String())
	}
}

// T-ADM-024：ZIP 导入把草稿正文保留在内存中，即使旧的导入路径被占用也不受影响。
func TestConfigArchiveImportDoesNotWriteTemplateFiles(t *testing.T) {
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
		t.Fatalf("app.New 失败：%v", err)
	}
	authSvc, err := auth.New(auth.Options{
		StatePath:  filepath.Join(dir, "auth.json"),
		SetupToken: "setup-secret",
		Logger:     log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("auth.New 失败：%v", err)
	}
	handler := New(appSvc, authSvc)
	setup := serve(handler, http.MethodPost, "/api/auth/setup", `{"username":"admin","password":"long-enough-password","setup_token":"setup-secret"}`, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup 状态码 = %d 响应体 = %s", setup.Code, setup.Body.String())
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
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"inline"`) {
		t.Fatalf("草稿导入 状态码 = %d 响应体 = %s", resp.Code, resp.Body.String())
	}
}

// T-ADM-012：status、preview、link 接口需要 session
// T-PRV-004：status 返回 version 和 config_write 能力
// T-PRV-001：节点预览返回带 name 和 active_count 的节点列表
// T-PRV-013：生成链接返回 URL 及 token_included
// T-PRV-003：生成预览的输出与 generate 输出一致（不带 Content-Disposition）
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
		t.Fatalf("app.New 失败：%v", err)
	}
	authSvc, err := auth.New(auth.Options{
		StatePath:  filepath.Join(dir, "auth.json"),
		SetupToken: "setup-secret",
		Logger:     log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("auth.New 失败：%v", err)
	}
	handler := New(appSvc, authSvc)

	noCookie := serve(handler, http.MethodGet, "/api/status", "", nil)
	if noCookie.Code != http.StatusUnauthorized || !strings.Contains(noCookie.Body.String(), "auth_required") {
		t.Fatalf("无 cookie 时 状态码 = %d 响应体 = %s", noCookie.Code, noCookie.Body.String())
	}

	setup := serve(handler, http.MethodPost, "/api/auth/setup", `{"username":"admin","password":"long-enough-password","setup_token":"setup-secret"}`, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup 状态码 = %d 响应体 = %s", setup.Code, setup.Body.String())
	}
	cookie := setup.Result().Cookies()[0]
	headers := map[string]string{"Cookie": auth.SessionCookieName + "=" + cookie.Value}

	// T-ADM-LOON-001：已认证的运行时预览、草稿预览和订阅链接都支持 Loon。
	snapshot, err := appSvc.ConfigSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		body := ""
		if method == http.MethodPost {
			body = `{"config":` + string(snapshot.Config) + `}`
		}
		preview := serve(handler, method, "/api/generate/preview?format=loon", body, headers)
		if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), "[Remote Rule]") || preview.Header().Get("Content-Disposition") != "" {
			t.Fatalf("Loon %s 预览：状态码=%d 响应体=%s", method, preview.Code, preview.Body.String())
		}
	}
	loonLink := serve(handler, http.MethodGet, "/api/generate/link?format=loon", "", headers)
	if loonLink.Code != http.StatusOK || !strings.Contains(loonLink.Body.String(), "loon.lcf") {
		t.Fatalf("Loon 链接：状态码=%d 响应体=%s", loonLink.Code, loonLink.Body.String())
	}

	status := serve(handler, http.MethodGet, "/api/status", "", headers)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"version":"2.0.0"`) || !strings.Contains(status.Body.String(), `"config_write":true`) {
		t.Fatalf("status 接口 = %d 响应体 = %s", status.Code, status.Body.String())
	}

	nodes := serve(handler, http.MethodGet, "/api/preview/nodes", "", headers)
	if nodes.Code != http.StatusOK || !strings.Contains(nodes.Body.String(), `"name":"HK-01"`) || !strings.Contains(nodes.Body.String(), `"active_count":1`) {
		t.Fatalf("nodes 接口 = %d 响应体 = %s", nodes.Code, nodes.Body.String())
	}

	link := serve(handler, http.MethodGet, "/api/generate/link?format=surge&filename=phone.conf", "", headers)
	if link.Code != http.StatusOK || !strings.Contains(link.Body.String(), `token_included":true`) || !strings.Contains(link.Body.String(), `token=server-token`) {
		t.Fatalf("link 接口 = %d 响应体 = %s", link.Code, link.Body.String())
	}

	preview := serve(handler, http.MethodGet, "/api/generate/preview?format=clash", "", headers)
	if preview.Code != http.StatusOK {
		t.Fatalf("生成预览 = %d 响应体 = %s", preview.Code, preview.Body.String())
	}
	if preview.Header().Get("Content-Disposition") != "" {
		t.Fatalf("生成预览不应设置 Content-Disposition，实际为 %q", preview.Header().Get("Content-Disposition"))
	}
	if !strings.Contains(preview.Body.String(), "HK-01") {
		t.Fatalf("生成预览的响应体缺少节点：%s", preview.Body.String())
	}
	generated, err := appSvc.Generate(context.Background(), generate.Request{Format: "clash", Filename: "clash.yaml"})
	if err != nil {
		t.Fatalf("app Generate 失败：%v", err)
	}
	if preview.Body.String() != string(generated.Body) {
		t.Fatalf("生成预览的响应体与 generate 输出不一致\npreview:\n%s\ngenerate:\n%s", preview.Body.String(), generated.Body)
	}
}

// T-ADM-022：CSRF 同源检查覆盖协议、主机和端口的匹配
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
				t.Fatalf("sameOrigin() = %v，期望 %v", got, tt.want)
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

type blockingAdminFetcher struct {
	mu       sync.Mutex
	body     []byte
	blocking bool
	entered  chan struct{}
	release  chan struct{}
}

func (f *blockingAdminFetcher) Fetch(ctx context.Context, _ string) ([]byte, error) {
	f.mu.Lock()
	shouldBlock := f.blocking
	entered := f.entered
	release := f.release
	f.mu.Unlock()

	if shouldBlock {
		entered <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]byte(nil), f.body...), nil
}

// T-RLD-001：已有 reload 在执行时，POST /api/reload 返回 429 reload_in_progress
// T-RLD-008：进行中的 reload 完成后，POST /api/reload 可以再次成功
func TestReloadEndpointReturns429WhenInProgress(t *testing.T) {
	dir := t.TempDir()
	remoteURL := "https://config.example.com/config.yaml"
	fetcher := &blockingAdminFetcher{
		body:    []byte(appTestConfigYAML),
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	appSvc, err := app.New(context.Background(), app.Options{
		ConfigLocation: remoteURL,
		Fetcher:        fetcher,
	})
	if err != nil {
		t.Fatalf("app.New 失败：%v", err)
	}
	authSvc, err := auth.New(auth.Options{
		StatePath:  filepath.Join(dir, "auth.json"),
		SetupToken: "setup-secret",
		Logger:     log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("auth.New 失败：%v", err)
	}
	handler := New(appSvc, authSvc)

	setup := serve(handler, http.MethodPost, "/api/auth/setup", `{"username":"admin","password":"long-enough-password","setup_token":"setup-secret"}`, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup 状态码 = %d 响应体 = %s", setup.Code, setup.Body.String())
	}
	headers := map[string]string{"Cookie": auth.SessionCookieName + "=" + setup.Result().Cookies()[0].Value}

	fetcher.mu.Lock()
	fetcher.blocking = true
	fetcher.mu.Unlock()

	firstRespCh := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		firstRespCh <- serve(handler, http.MethodPost, "/api/reload", "", headers)
	}()

	select {
	case <-fetcher.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("等待第一个 POST /api/reload 开始拉取时超时")
	}

	second := serve(handler, http.MethodPost, "/api/reload", "", headers)
	if second.Code != http.StatusTooManyRequests || !strings.Contains(second.Body.String(), `"code":"reload_in_progress"`) {
		t.Fatalf("并发 reload 状态码 = %d 响应体 = %s，期望 429 reload_in_progress", second.Code, second.Body.String())
	}

	close(fetcher.release)
	select {
	case first := <-firstRespCh:
		if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"success":true`) {
			t.Fatalf("第一次 reload 状态码 = %d 响应体 = %s，期望 200 success", first.Code, first.Body.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待第一个 POST /api/reload 完成时超时")
	}

	fetcher.mu.Lock()
	fetcher.blocking = false
	fetcher.mu.Unlock()

	third := serve(handler, http.MethodPost, "/api/reload", "", headers)
	if third.Code != http.StatusOK || !strings.Contains(third.Body.String(), `"success":true`) {
		t.Fatalf("重试 reload 状态码 = %d 响应体 = %s，期望 200 success", third.Code, third.Body.String())
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

func adminArchive(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("创建压缩包条目 %q 失败：%v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("写入压缩包条目 %q 失败：%v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("关闭压缩包失败：%v", err)
	}
	return buf.Bytes()
}

func adminArchiveEntries(t *testing.T, body []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("打开压缩包失败：%v", err)
	}
	entries := make(map[string][]byte, len(zr.File))
	for _, file := range zr.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("打开压缩包条目 %q 失败：%v", file.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("读取压缩包条目 %q 失败：%v", file.Name, err)
		}
		entries[file.Name] = data
	}
	return entries
}

func adminImportedArchiveConfig(t *testing.T, body []byte) struct {
	Templates struct {
		Clash  string            `json:"clash"`
		Inline map[string]string `json:"inline"`
		Surge  string            `json:"surge"`
	} `json:"templates"`
} {
	t.Helper()
	var envelope struct {
		Config struct {
			Templates struct {
				Clash  string            `json:"clash"`
				Inline map[string]string `json:"inline"`
				Surge  string            `json:"surge"`
			} `json:"templates"`
		} `json:"config"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("解析压缩包导入响应失败：%v", err)
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
