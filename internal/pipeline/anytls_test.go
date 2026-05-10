package pipeline

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// T-SRC-ANYTLS-001: AnyTLS URI parser accepts the provider URI shape.
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
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if got.Type != "anytls" {
				t.Errorf("Type = %q, want anytls", got.Type)
			}
			if got.Kind != model.KindSubscription {
				t.Errorf("Kind = %q, want %q", got.Kind, model.KindSubscription)
			}
			if got.Server != tt.wantServer {
				t.Errorf("Server = %q, want %q", got.Server, tt.wantServer)
			}
			if got.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", got.Port, tt.wantPort)
			}
			for key, want := range tt.wantParams {
				if got.Params[key] != want {
					t.Errorf("Params[%q] = %q, want %q", key, got.Params[key], want)
				}
			}
		})
	}
}

// T-SRC-ANYTLS-002: malformed AnyTLS URIs return source build errors.
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
				t.Fatal("expected error, got nil")
			}
			var buildErr *errtype.BuildError
			if !errors.As(err, &buildErr) {
				t.Fatalf("error type = %T, want *errtype.BuildError", err)
			}
			if buildErr.Code != errtype.CodeBuildAnyTLSURIInvalid {
				t.Errorf("Code = %q, want %q", buildErr.Code, errtype.CodeBuildAnyTLSURIInvalid)
			}
		})
	}
}

// T-SRC-ANYTLS-003: generic subscription parser accepts AnyTLS plain-text
// line formats from URI, Surge, and Quantumult X sources.
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
			line:       "anytls://1b3614e427e3451b@demo.com:3383/?sni=cache-proxy.example.com&insecure=1#HK-01",
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
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Name != tt.wantName || got.Server != tt.wantServer || got.Port != tt.wantPort {
				t.Fatalf("proxy = {%q %q %d}, want {%q %q %d}", got.Name, got.Server, got.Port, tt.wantName, tt.wantServer, tt.wantPort)
			}
			if got.Type != "anytls" || got.Kind != model.KindSubscription {
				t.Fatalf("Type/Kind = %q/%q, want anytls/%q", got.Type, got.Kind, model.KindSubscription)
			}
			if got.Params["password"] != "1b3614e427e3451b" {
				t.Errorf("password = %q, want sample password", got.Params["password"])
			}
			if got.Params["sni"] != tt.wantSNI {
				t.Errorf("sni = %q, want %q", got.Params["sni"], tt.wantSNI)
			}
			if got.Params["skip-cert-verify"] != tt.wantSkip {
				t.Errorf("skip-cert-verify = %q, want %q", got.Params["skip-cert-verify"], tt.wantSkip)
			}
		})
	}
}

// T-SRC-ANYTLS-004: malformed AnyTLS line formats use the AnyTLS line code.
func TestParseSubscriptionLine_AnyTLSInvalidLine(t *testing.T) {
	_, err := ParseSubscriptionLine("HK = anytls, demo.com, 443")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var buildErr *errtype.BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("error type = %T, want *errtype.BuildError", err)
	}
	if buildErr.Code != errtype.CodeBuildAnyTLSLineInvalid {
		t.Errorf("Code = %q, want %q", buildErr.Code, errtype.CodeBuildAnyTLSLineInvalid)
	}
}

// T-SRC-ANYTLS-FIX-001: provider-style AnyTLS samples remain accepted across
// supported subscription shapes.
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
					t.Fatalf("decode generated base64 body: %v", err)
				}
				lines = splitSubscriptionLines(text)
			}
			if len(lines) != tt.wantCount {
				t.Fatalf("sample lines = %d, want %d: %#v", len(lines), tt.wantCount, lines)
			}

			var proxies []model.Proxy
			for _, line := range lines {
				proxy, err := ParseSubscriptionLine(line)
				if err != nil {
					t.Fatalf("ParseSubscriptionLine(%q) error: %v", line, err)
				}
				if proxy.Type != "anytls" || proxy.Kind != model.KindSubscription {
					t.Fatalf("Type/Kind = %q/%q, want anytls/%q", proxy.Type, proxy.Kind, model.KindSubscription)
				}
				if proxy.Params["password"] == "" {
					t.Fatalf("proxy %q missing password", proxy.Name)
				}
				if proxy.Params["sni"] == "" {
					t.Fatalf("proxy %q missing sni", proxy.Name)
				}
				if tt.wantNoParam != "" && proxy.Params[tt.wantNoParam] != "" {
					t.Fatalf("proxy %q preserved input-only param %q=%q", proxy.Name, tt.wantNoParam, proxy.Params[tt.wantNoParam])
				}
				proxies = append(proxies, proxy)
			}

			if proxies[0].Name != tt.wantFirst {
				t.Errorf("first proxy name = %q, want %q", proxies[0].Name, tt.wantFirst)
			}
			if proxies[len(proxies)-1].Name != tt.wantLast {
				t.Errorf("last proxy name = %q, want %q", proxies[len(proxies)-1].Name, tt.wantLast)
			}
			if tt.wantUDP != "" && proxies[0].Params["udp-relay"] != tt.wantUDP {
				t.Errorf("udp-relay = %q, want %q", proxies[0].Params["udp-relay"], tt.wantUDP)
			}
			if tt.wantTFO != "" && proxies[0].Params["tfo"] != tt.wantTFO {
				t.Errorf("tfo = %q, want %q", proxies[0].Params["tfo"], tt.wantTFO)
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
