package proxyparse

import "testing"

// T-PXP-001：带认证的 socks5 URL
func TestParseURL_Socks5(t *testing.T) {
	r, err := ParseURL("socks5://user:pass@1.2.3.4:1080")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if r.Type != "socks5" {
		t.Errorf("Type = %q，期望 socks5", r.Type)
	}
	if r.Server != "1.2.3.4" || r.Port != 1080 {
		t.Errorf("Server:Port = %s:%d，期望 1.2.3.4:1080", r.Server, r.Port)
	}
	if r.Params["username"] != "user" || r.Params["password"] != "pass" {
		t.Errorf("Params = %v", r.Params)
	}
	if r.Plugin != nil {
		t.Errorf("socks5 的 Plugin 应为 nil")
	}
}

// T-PXP-002：不带认证的 socks5 URL
func TestParseURL_Socks5NoAuth(t *testing.T) {
	r, err := ParseURL("socks5://1.2.3.4:1080")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if len(r.Params) != 0 {
		t.Errorf("Params = %v，期望为空", r.Params)
	}
}

// T-PXP-003：带认证的 http URL
func TestParseURL_HTTP(t *testing.T) {
	r, err := ParseURL("http://admin:secret@10.0.0.1:8080")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if r.Type != "http" {
		t.Errorf("Type = %q，期望 http", r.Type)
	}
	if r.Server != "10.0.0.1" || r.Port != 8080 {
		t.Errorf("Server:Port = %s:%d", r.Server, r.Port)
	}
	if r.Params["username"] != "admin" || r.Params["password"] != "secret" {
		t.Errorf("Params = %v", r.Params)
	}
}

// T-PXP-004：userinfo 为 base64 的 SS URL
func TestParseURL_SSBase64(t *testing.T) {
	r, err := ParseURL("ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if r.Type != "ss" {
		t.Errorf("Type = %q，期望 ss", r.Type)
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
		t.Errorf("Plugin 应为 nil")
	}
}

// T-PXP-005：userinfo 为明文的 SS URL（SIP002）
func TestParseURL_SSPlainUserinfo(t *testing.T) {
	r, err := ParseURL("ss://aes-256-gcm:mypass@1.2.3.4:8388")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if r.Params["cipher"] != "aes-256-gcm" || r.Params["password"] != "mypass" {
		t.Errorf("Params = %v", r.Params)
	}
}

// T-PXP-006：带 plugin query 参数的 SS URL
func TestParseURL_SSWithPlugin(t *testing.T) {
	r, err := ParseURL("ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dexample.com")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if r.Plugin == nil {
		t.Fatal("Plugin 不应为 nil")
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

// T-PXP-007：SS 的 fragment 被忽略
func TestParseURL_SSFragmentIgnored(t *testing.T) {
	r, err := ParseURL("ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388#SomeNodeName")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if r.Type != "ss" || r.Server != "1.2.3.4" {
		t.Errorf("解析结果：Type=%q Server=%q", r.Type, r.Server)
	}
}

// T-PXP-008：解析 IPv6 主机
func TestParseURL_IPv6(t *testing.T) {
	r, err := ParseURL("socks5://[::1]:1080")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if r.Server != "::1" || r.Port != 1080 {
		t.Errorf("Server:Port = %s:%d", r.Server, r.Port)
	}
}

// T-PXP-009：不支持的协议返回错误
func TestParseURL_UnsupportedScheme(t *testing.T) {
	_, err := ParseURL("vmess://1.2.3.4:1080")
	if err == nil {
		t.Fatal("不支持的协议应返回错误")
	}
}

// T-PXP-010：缺少端口返回错误
func TestParseURL_MissingPort(t *testing.T) {
	_, err := ParseURL("socks5://1.2.3.4")
	if err == nil {
		t.Fatal("缺少端口应返回错误")
	}
}

// T-PXP-011：端口超出范围返回错误
func TestParseURL_PortOutOfRange(t *testing.T) {
	_, err := ParseURL("socks5://1.2.3.4:70000")
	if err == nil {
		t.Fatal("端口超出范围应返回错误")
	}
}

// T-PXP-012：socks5 拒绝非空 path
func TestParseURL_Socks5RejectsPath(t *testing.T) {
	_, err := ParseURL("socks5://1.2.3.4:1080/typo")
	if err == nil {
		t.Fatal("socks5 带 path 应返回错误")
	}
}

// T-PXP-013：socks5 拒绝末尾斜杠
func TestParseURL_Socks5RejectsTrailingSlash(t *testing.T) {
	_, err := ParseURL("socks5://1.2.3.4:1080/")
	if err == nil {
		t.Fatal("末尾带斜杠应返回错误")
	}
}

// T-PXP-014：http 拒绝 query 参数
func TestParseURL_HTTPRejectsQuery(t *testing.T) {
	_, err := ParseURL("http://1.2.3.4:8080?foo=bar")
	if err == nil {
		t.Fatal("http 带 query 应返回错误")
	}
}

// T-PXP-015：socks5 拒绝 fragment
func TestParseURL_Socks5RejectsFragment(t *testing.T) {
	_, err := ParseURL("socks5://1.2.3.4:1080#frag")
	if err == nil {
		t.Fatal("socks5 带 fragment 应返回错误")
	}
}

// T-PXP-016：SS 缺少 @ 返回错误
func TestParseURL_SSMissingAt(t *testing.T) {
	_, err := ParseURL("ss://YWVzLTI1Ni1nY206bXlwYXNz")
	if err == nil {
		t.Fatal("SS URI 缺少 @ 应返回错误")
	}
}

// T-PXP-017：SS 主机为空返回错误
func TestParseURL_SSEmptyHost(t *testing.T) {
	_, err := ParseURL("ss://YWVzLTI1Ni1nY206bXlwYXNz@:8388")
	if err == nil {
		t.Fatal("主机为空应返回错误")
	}
}

// T-PXP-018：SS 端口边界值
func TestParseURL_SSPortBoundary(t *testing.T) {
	r, err := ParseURL("ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:65535")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if r.Port != 65535 {
		t.Errorf("Port = %d，期望 65535", r.Port)
	}
}
