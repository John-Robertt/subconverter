package pipeline

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// T-SRC-ANYTLS-001: AnyTLS URI 解析器接受机场提供的 URI 形式。
func TestParseAnyTLSURI_Valid(t *testing.T) {
	tests := []struct {
		name       string
		uri        string
		wantName   string
		wantServer string
		wantPort   int
		wantParams map[string]string
	}{
		{
			name:       "provider sample",
			uri:        "anytls://1b3614e427e3451b@demo.com:3383/?sni=cache-proxy.example.com&insecure=1&group=TmV4#%F0%9F%87%AD%F0%9F%87%B0%20Hong%20Kong%2001",
			wantName:   "🇭🇰 Hong Kong 01",
			wantServer: "demo.com",
			wantPort:   3383,
			wantParams: map[string]string{"password": "1b3614e427e3451b", "sni": "cache-proxy.example.com", "skip-cert-verify": "true"},
		},
		{
			name:       "peer query maps to sni",
			uri:        "anytls://test-password@node.example.com:22001/?peer=cdn.example.com&insecure=1&udp=1&group=metadata#TEST%2001",
			wantName:   "TEST 01",
			wantServer: "node.example.com",
			wantPort:   22001,
			wantParams: map[string]string{"password": "test-password", "sni": "cdn.example.com", "skip-cert-verify": "true"},
		},
		{
			name:       "sni query takes precedence over peer alias",
			uri:        "anytls://secret@example.com:443/?peer=peer.example.com&sni=sni.example.com#HK-02",
			wantName:   "HK-02",
			wantServer: "example.com",
			wantPort:   443,
			wantParams: map[string]string{"password": "secret", "sni": "sni.example.com"},
		},
		{
			name:       "default port",
			uri:        "anytls://secret@example.com/?sni=edge.example.com#HK-01",
			wantName:   "HK-01",
			wantServer: "example.com",
			wantPort:   443,
			wantParams: map[string]string{"password": "secret", "sni": "edge.example.com"},
		},
		{
			name:       "ipv6 host",
			uri:        "anytls://secret@[2001:db8::1]:8443/#IPV6",
			wantName:   "IPV6",
			wantServer: "2001:db8::1",
			wantPort:   8443,
			wantParams: map[string]string{"password": "secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAnyTLSURI(tt.uri)
			if err != nil {
				t.Fatalf("意外错误：%v", err)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q，期望 %q", got.Name, tt.wantName)
			}
			if got.Type != "anytls" {
				t.Errorf("Type = %q，期望 anytls", got.Type)
			}
			if got.Kind != model.KindSubscription {
				t.Errorf("Kind = %q，期望 %q", got.Kind, model.KindSubscription)
			}
			if got.Server != tt.wantServer {
				t.Errorf("Server = %q，期望 %q", got.Server, tt.wantServer)
			}
			if got.Port != tt.wantPort {
				t.Errorf("Port = %d，期望 %d", got.Port, tt.wantPort)
			}
			for key, want := range tt.wantParams {
				if got.Params[key] != want {
					t.Errorf("Params[%q] = %q，期望 %q", key, got.Params[key], want)
				}
			}
		})
	}
}

// T-SRC-ANYTLS-002: 格式错误的 AnyTLS URI 返回 source 阶段的构建错误。
func TestParseAnyTLSURI_Invalid(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{"wrong scheme", "ss://secret@example.com:443#HK"},
		{"missing password", "anytls://example.com:443#HK"},
		{"missing name", "anytls://secret@example.com:443"},
		{"bad port", "anytls://secret@example.com:70000#HK"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAnyTLSURI(tt.uri)
			if err == nil {
				t.Fatal("期望返回错误，实际为 nil")
			}
			var buildErr *errtype.BuildError
			if !errors.As(err, &buildErr) {
				t.Fatalf("错误类型 = %T，期望 *errtype.BuildError", err)
			}
			if buildErr.Code != errtype.CodeBuildAnyTLSURIInvalid {
				t.Errorf("Code = %q，期望 %q", buildErr.Code, errtype.CodeBuildAnyTLSURIInvalid)
			}
		})
	}
}

// T-SRC-ANYTLS-003: 通用订阅解析器接受 URI、Surge 和 Quantumult X
// 三种来源的 AnyTLS 纯文本行格式。
func TestParseSubscriptionLine_AnyTLSFormats(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantName   string
		wantServer string
		wantPort   int
		wantSNI    string
		wantSkip   string
	}{
		{
			name:       "uri",
			line:       "anytls://1b3614e427e3451b@demo.com:3383/?peer=cache-proxy.example.com&insecure=1#HK-01",
			wantName:   "HK-01",
			wantServer: "demo.com",
			wantPort:   3383,
			wantSNI:    "cache-proxy.example.com",
			wantSkip:   "true",
		},
		{
			name:       "surge",
			line:       "HK-02 = anytls, demo.com, 3814, password=1b3614e427e3451b, tls=true, sni=cache-proxy.example.com, skip-cert-verify=true",
			wantName:   "HK-02",
			wantServer: "demo.com",
			wantPort:   3814,
			wantSNI:    "cache-proxy.example.com",
			wantSkip:   "true",
		},
		{
			name:       "quanx",
			line:       "anytls=demo.com:3451, password=1b3614e427e3451b, over-tls=true, tls-host=cache-proxy.example.com, tls-verification=false, tag=HK-03",
			wantName:   "HK-03",
			wantServer: "demo.com",
			wantPort:   3451,
			wantSNI:    "cache-proxy.example.com",
			wantSkip:   "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSubscriptionLine(tt.line)
			if err != nil {
				t.Fatalf("意外错误：%v", err)
			}
			if got.Name != tt.wantName || got.Server != tt.wantServer || got.Port != tt.wantPort {
				t.Fatalf("节点 = {%q %q %d}，期望 {%q %q %d}", got.Name, got.Server, got.Port, tt.wantName, tt.wantServer, tt.wantPort)
			}
			if got.Type != "anytls" || got.Kind != model.KindSubscription {
				t.Fatalf("Type/Kind = %q/%q，期望 anytls/%q", got.Type, got.Kind, model.KindSubscription)
			}
			if got.Params["password"] != "1b3614e427e3451b" {
				t.Errorf("password = %q，期望为样例密码", got.Params["password"])
			}
			if got.Params["sni"] != tt.wantSNI {
				t.Errorf("sni = %q，期望 %q", got.Params["sni"], tt.wantSNI)
			}
			if got.Params["skip-cert-verify"] != tt.wantSkip {
				t.Errorf("skip-cert-verify = %q，期望 %q", got.Params["skip-cert-verify"], tt.wantSkip)
			}
		})
	}
}

// T-SRC-ANYTLS-004: 格式错误的 AnyTLS 行使用 AnyTLS 行对应的错误码。
func TestParseSubscriptionLine_AnyTLSInvalidLine(t *testing.T) {
	_, err := ParseSubscriptionLine("HK = anytls, demo.com, 443")
	if err == nil {
		t.Fatal("期望返回错误，实际为 nil")
	}
	var buildErr *errtype.BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.BuildError", err)
	}
	if buildErr.Code != errtype.CodeBuildAnyTLSLineInvalid {
		t.Errorf("Code = %q，期望 %q", buildErr.Code, errtype.CodeBuildAnyTLSLineInvalid)
	}
}

// T-SRC-ANYTLS-FIX-001: 机场风格的 AnyTLS 样例在各种支持的订阅形式下
// 都能被接受。
func TestParseSubscriptionLine_AnyTLSSamples(t *testing.T) {
	tests := []struct {
		name        string
		lines       []string
		base64Body  bool
		wantCount   int
		wantFirst   string
		wantLast    string
		wantUDP     string
		wantTFO     string
		wantNoParam string
	}{
		{
			name:      "uri list",
			lines:     sampleAnyTLSURILines(),
			wantCount: 3,
			wantFirst: "🇭🇰 Hong Kong 01",
			wantLast:  "🇭🇰 Hong Kong 03",
		},
		{
			name:       "base64 uri list",
			lines:      sampleAnyTLSURILines(),
			base64Body: true,
			wantCount:  3,
			wantFirst:  "🇭🇰 Hong Kong 01",
			wantLast:   "🇭🇰 Hong Kong 03",
		},
		{
			name:        "surge list",
			lines:       sampleAnyTLSSurgeLines(),
			wantCount:   5,
			wantFirst:   "0.80 G | 500.00 G",
			wantLast:    "🇭🇰 Hong Kong 02",
			wantNoParam: "tls",
		},
		{
			name:      "quanx list",
			lines:     sampleAnyTLSQuanXLines(),
			wantCount: 5,
			wantFirst: "0.76 G | 500.00 G",
			wantLast:  "🇭🇰 Hong Kong 02",
			wantUDP:   "true",
			wantTFO:   "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := tt.lines
			if tt.base64Body {
				body := []byte(base64.StdEncoding.EncodeToString([]byte(strings.Join(tt.lines, "\n"))))
				text, err := subscriptionBodyText(body)
				if err != nil {
					t.Fatalf("解码生成的 base64 正文：%v", err)
				}
				lines = splitSubscriptionLines(text)
			}
			if len(lines) != tt.wantCount {
				t.Fatalf("样例行数 = %d，期望 %d：%#v", len(lines), tt.wantCount, lines)
			}

			var proxies []model.Proxy
			for _, line := range lines {
				proxy, err := ParseSubscriptionLine(line)
				if err != nil {
					t.Fatalf("ParseSubscriptionLine(%q) 出错：%v", line, err)
				}
				if proxy.Type != "anytls" || proxy.Kind != model.KindSubscription {
					t.Fatalf("Type/Kind = %q/%q，期望 anytls/%q", proxy.Type, proxy.Kind, model.KindSubscription)
				}
				if proxy.Params["password"] == "" {
					t.Fatalf("节点 %q 缺少 password", proxy.Name)
				}
				if proxy.Params["sni"] == "" {
					t.Fatalf("节点 %q 缺少 sni", proxy.Name)
				}
				if tt.wantNoParam != "" && proxy.Params[tt.wantNoParam] != "" {
					t.Fatalf("节点 %q 保留了仅用于输入的参数 %q=%q", proxy.Name, tt.wantNoParam, proxy.Params[tt.wantNoParam])
				}
				proxies = append(proxies, proxy)
			}

			if proxies[0].Name != tt.wantFirst {
				t.Errorf("第一个节点名 = %q，期望 %q", proxies[0].Name, tt.wantFirst)
			}
			if proxies[len(proxies)-1].Name != tt.wantLast {
				t.Errorf("最后一个节点名 = %q，期望 %q", proxies[len(proxies)-1].Name, tt.wantLast)
			}
			if tt.wantUDP != "" && proxies[0].Params["udp-relay"] != tt.wantUDP {
				t.Errorf("udp-relay = %q，期望 %q", proxies[0].Params["udp-relay"], tt.wantUDP)
			}
			if tt.wantTFO != "" && proxies[0].Params["tfo"] != tt.wantTFO {
				t.Errorf("tfo = %q，期望 %q", proxies[0].Params["tfo"], tt.wantTFO)
			}
		})
	}
}

func sampleAnyTLSURILines() []string {
	return []string{
		"anytls://1b3614e427e3451b@demo.com:3383/?sni=cache-proxy.example.com&insecure=1&group=TmV4#%F0%9F%87%AD%F0%9F%87%B0%20Hong%20Kong%2001",
		"anytls://1b3614e427e3451b@demo.com:3814/?sni=cache-proxy.example.com&insecure=1&group=TmV4#%F0%9F%87%AD%F0%9F%87%B0%20Hong%20Kong%2002",
		"anytls://1b3614e427e3451b@demo.com:3451/?sni=cache-proxy.example.com&insecure=1&group=TmV4#%F0%9F%87%AD%F0%9F%87%B0%20Hong%20Kong%2003",
	}
}

func sampleAnyTLSSurgeLines() []string {
	return []string{
		"0.80 G | 500.00 G = anytls, info.example.com, 443, password=secret, tls=true, sni=cache-proxy.example.com",
		"Expire 2026-12-31 = anytls, expire.example.com, 443, password=secret, tls=true, sni=cache-proxy.example.com",
		"Reset Date 1 = anytls, reset.example.com, 443, password=secret, tls=true, sni=cache-proxy.example.com",
		"🇭🇰 Hong Kong 01 = anytls, hk1.example.com, 3383, password=secret, tls=true, sni=cache-proxy.example.com, skip-cert-verify=true, reuse=true, server-cert-fingerprint-sha256=abc123",
		"🇭🇰 Hong Kong 02 = anytls, hk2.example.com, 3814, password=secret, tls=true, sni=cache-proxy.example.com, skip-cert-verify=true",
	}
}

func sampleAnyTLSQuanXLines() []string {
	return []string{
		"anytls=info.example.com:443, password=secret, over-tls=true, tls-host=cache-proxy.example.com, tls-verification=false, udp-relay=true, fast-open=false, tag=0.76 G | 500.00 G",
		"anytls=expire.example.com:443, password=secret, over-tls=true, tls-host=cache-proxy.example.com, tls-verification=false, tag=Expire 2026-12-31",
		"anytls=reset.example.com:443, password=secret, over-tls=true, tls-host=cache-proxy.example.com, tls-verification=false, tag=Reset Date 1",
		"anytls=hk1.example.com:3383, password=secret, over-tls=true, tls-host=cache-proxy.example.com, tls-verification=false, udp-relay=true, fast-open=false, tag=🇭🇰 Hong Kong 01",
		"anytls=hk2.example.com:3814, password=secret, over-tls=true, tls-host=cache-proxy.example.com, tls-verification=false, tag=🇭🇰 Hong Kong 02",
	}
}
