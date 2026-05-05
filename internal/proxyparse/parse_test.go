package proxyparse

import "testing"

// T-PXP-001: socks5 URL with auth
func TestParseURL_Socks5(t *testing.T) {
	r, err := ParseURL("socks5://user:pass@1.2.3.4:1080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Type != "socks5" {
		t.Errorf("Type = %q, want socks5", r.Type)
	}
	if r.Server != "1.2.3.4" || r.Port != 1080 {
		t.Errorf("Server:Port = %s:%d, want 1.2.3.4:1080", r.Server, r.Port)
	}
	if r.Params["username"] != "user" || r.Params["password"] != "pass" {
		t.Errorf("Params = %v", r.Params)
	}
	if r.Plugin != nil {
		t.Errorf("Plugin should be nil for socks5")
	}
}

// T-PXP-002: socks5 URL without auth
func TestParseURL_Socks5NoAuth(t *testing.T) {
	r, err := ParseURL("socks5://1.2.3.4:1080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Params) != 0 {
		t.Errorf("Params = %v, want empty", r.Params)
	}
}

// T-PXP-003: http URL with auth
func TestParseURL_HTTP(t *testing.T) {
	r, err := ParseURL("http://admin:secret@10.0.0.1:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Type != "http" {
		t.Errorf("Type = %q, want http", r.Type)
	}
	if r.Server != "10.0.0.1" || r.Port != 8080 {
		t.Errorf("Server:Port = %s:%d", r.Server, r.Port)
	}
	if r.Params["username"] != "admin" || r.Params["password"] != "secret" {
		t.Errorf("Params = %v", r.Params)
	}
}

// T-PXP-004: SS URL with base64 userinfo
func TestParseURL_SSBase64(t *testing.T) {
	r, err := ParseURL("ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Type != "ss" {
		t.Errorf("Type = %q, want ss", r.Type)
	}
	if r.Server != "1.2.3.4" || r.Port != 8388 {
		t.Errorf("Server:Port = %s:%d", r.Server, r.Port)
	}
	if r.Params["cipher"] != "aes-256-gcm" {
		t.Errorf("cipher = %q", r.Params["cipher"])
	}
	if r.Params["password"] != "mypass" {
		t.Errorf("password = %q", r.Params["password"])
	}
	if r.Plugin != nil {
		t.Errorf("Plugin should be nil")
	}
}

// T-PXP-005: SS URL with plain userinfo (SIP002)
func TestParseURL_SSPlainUserinfo(t *testing.T) {
	r, err := ParseURL("ss://aes-256-gcm:mypass@1.2.3.4:8388")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Params["cipher"] != "aes-256-gcm" || r.Params["password"] != "mypass" {
		t.Errorf("Params = %v", r.Params)
	}
}

// T-PXP-006: SS URL with plugin query params
func TestParseURL_SSWithPlugin(t *testing.T) {
	r, err := ParseURL("ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dexample.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Plugin == nil {
		t.Fatal("Plugin should not be nil")
	}
	if r.Plugin.Name != "obfs-local" {
		t.Errorf("Plugin.Name = %q", r.Plugin.Name)
	}
	if r.Plugin.Opts["obfs"] != "http" {
		t.Errorf("Plugin.Opts[obfs] = %q", r.Plugin.Opts["obfs"])
	}
	if r.Plugin.Opts["obfs-host"] != "example.com" {
		t.Errorf("Plugin.Opts[obfs-host] = %q", r.Plugin.Opts["obfs-host"])
	}
}

// T-PXP-007: SS fragment is ignored
func TestParseURL_SSFragmentIgnored(t *testing.T) {
	r, err := ParseURL("ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388#SomeNodeName")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Type != "ss" || r.Server != "1.2.3.4" {
		t.Errorf("parsed result: Type=%q Server=%q", r.Type, r.Server)
	}
}

// T-PXP-008: IPv6 host parsing
func TestParseURL_IPv6(t *testing.T) {
	r, err := ParseURL("socks5://[::1]:1080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Server != "::1" || r.Port != 1080 {
		t.Errorf("Server:Port = %s:%d", r.Server, r.Port)
	}
}

// T-PXP-009: unsupported scheme returns error
func TestParseURL_UnsupportedScheme(t *testing.T) {
	_, err := ParseURL("vmess://1.2.3.4:1080")
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}

// T-PXP-010: missing port returns error
func TestParseURL_MissingPort(t *testing.T) {
	_, err := ParseURL("socks5://1.2.3.4")
	if err == nil {
		t.Fatal("expected error for missing port")
	}
}

// T-PXP-011: port out of range returns error
func TestParseURL_PortOutOfRange(t *testing.T) {
	_, err := ParseURL("socks5://1.2.3.4:70000")
	if err == nil {
		t.Fatal("expected error for port out of range")
	}
}

// T-PXP-012: socks5 rejects non-empty path
func TestParseURL_Socks5RejectsPath(t *testing.T) {
	_, err := ParseURL("socks5://1.2.3.4:1080/typo")
	if err == nil {
		t.Fatal("expected error for socks5 path")
	}
}

// T-PXP-013: socks5 rejects trailing slash
func TestParseURL_Socks5RejectsTrailingSlash(t *testing.T) {
	_, err := ParseURL("socks5://1.2.3.4:1080/")
	if err == nil {
		t.Fatal("expected error for trailing slash")
	}
}

// T-PXP-014: http rejects query params
func TestParseURL_HTTPRejectsQuery(t *testing.T) {
	_, err := ParseURL("http://1.2.3.4:8080?foo=bar")
	if err == nil {
		t.Fatal("expected error for http query")
	}
}

// T-PXP-015: socks5 rejects fragment
func TestParseURL_Socks5RejectsFragment(t *testing.T) {
	_, err := ParseURL("socks5://1.2.3.4:1080#frag")
	if err == nil {
		t.Fatal("expected error for socks5 fragment")
	}
}

// T-PXP-016: SS missing @ returns error
func TestParseURL_SSMissingAt(t *testing.T) {
	_, err := ParseURL("ss://YWVzLTI1Ni1nY206bXlwYXNz")
	if err == nil {
		t.Fatal("expected error for SS URI missing @")
	}
}

// T-PXP-017: SS empty host returns error
func TestParseURL_SSEmptyHost(t *testing.T) {
	_, err := ParseURL("ss://YWVzLTI1Ni1nY206bXlwYXNz@:8388")
	if err == nil {
		t.Fatal("expected error for empty host")
	}
}

// T-PXP-018: SS port boundary values
func TestParseURL_SSPortBoundary(t *testing.T) {
	r, err := ParseURL("ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:65535")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Port != 65535 {
		t.Errorf("Port = %d, want 65535", r.Port)
	}
}
