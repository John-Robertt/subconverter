package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
)

// T-SRV-001: resolveFilename sanitizes filenames and uses format defaults
func TestResolveFilename(t *testing.T) {
	tests := []struct {
		name    string
		query   url.Values
		format  string
		want    string
		wantErr string
	}{
		{
			name:   "default clash filename",
			query:  url.Values{},
			format: "clash",
			want:   "clash.yaml",
		},
		{
			name:   "basename gets default extension",
			query:  url.Values{"filename": {"my-profile"}},
			format: "surge",
			want:   "my-profile.conf",
		},
		{
			name:   "uppercase extension allowed",
			query:  url.Values{"filename": {"PROFILE.YAML"}},
			format: "clash",
			want:   "PROFILE.YAML",
		},
		{
			name:    "unicode rejected",
			query:   url.Values{"filename": {"配置"}},
			format:  "clash",
			wantErr: "filename 参数无效：仅允许 ASCII 字母、数字、点号(.)、连字符(-)、下划线(_)",
		},
		{
			name:    "space rejected",
			query:   url.Values{"filename": {"my profile"}},
			format:  "clash",
			wantErr: "filename 参数无效：仅允许 ASCII 字母、数字、点号(.)、连字符(-)、下划线(_)",
		},
		{
			name:    "quote rejected",
			query:   url.Values{"filename": {`a"b`}},
			format:  "clash",
			wantErr: "filename 参数无效：仅允许 ASCII 字母、数字、点号(.)、连字符(-)、下划线(_)",
		},
		{
			name:    "basename required",
			query:   url.Values{"filename": {".yaml"}},
			format:  "clash",
			wantErr: "filename 参数无效：文件名主体不能为空",
		},
		{
			name:    "wrong extension rejected",
			query:   url.Values{"filename": {"profile.yaml"}},
			format:  "surge",
			wantErr: "filename 参数无效：Surge 配置必须使用 .conf 扩展名",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveFilename(tc.query, tc.format)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("resolveFilename() error = nil, want %q", tc.wantErr)
				}
				if err.Error() != tc.wantErr {
					t.Fatalf("resolveFilename() error = %q, want %q", err.Error(), tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveFilename() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("resolveFilename() = %q, want %q", got, tc.want)
			}
		})
	}
}

// T-SRV-002: Content-Disposition value formatting
func TestContentDispositionValue(t *testing.T) {
	got := contentDispositionValue("my-profile.yaml")
	want := `attachment; filename="my-profile.yaml"; filename*=UTF-8''my-profile.yaml`
	if got != want {
		t.Fatalf("contentDispositionValue() = %q, want %q", got, want)
	}
}

// T-SRV-003: Web UI serves SPA fallback and hashed assets
func TestWebUIServesSPAFallbackAndAssets(t *testing.T) {
	srv := New(nil, Options{
		WebFS: fstest.MapFS{
			"index.html":          {Data: []byte("<!doctype html><div id=\"root\"></div>")},
			"assets/app-1234.js":  {Data: []byte("console.log('ok')")},
			"favicon.ico":         {Data: []byte("ico")},
			"assets/style.css":    {Data: []byte("body{}")},
			"assets/nested/skip":  {Data: []byte("nested")},
			"assets/nested/index": {Data: []byte("nested-index")},
		},
	})
	handler := srv.Handler()

	cases := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
		wantCache  string
	}{
		{
			name:       "root serves index",
			path:       "/",
			wantStatus: http.StatusOK,
			wantBody:   "root",
			wantCache:  webUICacheControlHTML,
		},
		{
			name:       "spa route falls back to index",
			path:       "/sources",
			wantStatus: http.StatusOK,
			wantBody:   "root",
			wantCache:  webUICacheControlHTML,
		},
		{
			name:       "index explicit",
			path:       "/index.html",
			wantStatus: http.StatusOK,
			wantBody:   "root",
			wantCache:  webUICacheControlHTML,
		},
		{
			name:       "asset uses immutable cache",
			path:       "/assets/app-1234.js",
			wantStatus: http.StatusOK,
			wantBody:   "console.log",
			wantCache:  webUICacheControlAsset,
		},
		{
			name:       "missing asset does not fall back",
			path:       "/assets/missing.js",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "api path does not fall back",
			path:       "/api/unknown",
			wantStatus: http.StatusNotFound,
			wantBody:   "404 page not found",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			handler.ServeHTTP(rec, req)
			resp := rec.Result()
			defer func() { _ = resp.Body.Close() }()
			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, tc.wantStatus, body)
			}
			if tc.wantBody != "" && !strings.Contains(string(body), tc.wantBody) {
				t.Fatalf("body = %q, want containing %q", body, tc.wantBody)
			}
			if tc.wantCache != "" && resp.Header.Get("Cache-Control") != tc.wantCache {
				t.Fatalf("Cache-Control = %q, want %q", resp.Header.Get("Cache-Control"), tc.wantCache)
			}
		})
	}
}
