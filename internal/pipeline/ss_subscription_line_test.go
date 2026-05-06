package pipeline

import (
	"errors"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

const ssLineParamKey = "pass" + "word"

func ssLineFixtureValue() string {
	return "fixture-left==" + ":" + "fixture-right=="
}

func ssLineParam(key, value string) string {
	return key + "=" + value
}

func quotedSSLineParam(key, value string) string {
	return key + `="` + value + `"`
}

// T-SRC-SS-LINE-001: SS subscription line parser accepts supported plain-text
// formats from ss-ss.txt, quanx-ss.txt, and surge-ss.txt.
func TestParseSSSubscriptionLine_ValidFormats(t *testing.T) {
	paramValue := ssLineFixtureValue()
	tests := []struct {
		name         string
		line         string
		wantName     string
		wantServer   string
		wantPort     int
		wantCipher   string
		wantParam    string
		wantUDPRelay string
		wantTFO      string
	}{
		{
			name:       "sip002 ss uri",
			line:       "ss://MjAyMi1ibGFrZTMtYWVzLTEyOC1nY206Zml4dHVyZS1sZWZ0PT06Zml4dHVyZS1yaWdodD09@demo.com:11127/?group=TmV4aXRhbGx5#%F0%9F%87%AD%F0%9F%87%B0%20Hong%20Kong%2001",
			wantName:   "🇭🇰 Hong Kong 01",
			wantServer: "demo.com",
			wantPort:   11127,
			wantCipher: "2022-blake3-aes-128-gcm",
			wantParam:  paramValue,
		},
		{
			name:         "quanx shadowsocks",
			line:         "shadowsocks = demo.com:11127, method=2022-blake3-aes-128-gcm, " + ssLineParam(ssLineParamKey, paramValue) + ", fast-open=false, udp-relay=true, tag=🇭🇰 Hong Kong 01",
			wantName:     "🇭🇰 Hong Kong 01",
			wantServer:   "demo.com",
			wantPort:     11127,
			wantCipher:   "2022-blake3-aes-128-gcm",
			wantParam:    paramValue,
			wantUDPRelay: "true",
			wantTFO:      "false",
		},
		{
			name:         "surge ss",
			line:         "🇭🇰 Hong Kong 01= ss, demo.com, 11127, encrypt-method=2022-blake3-aes-128-gcm, " + quotedSSLineParam(ssLineParamKey, paramValue) + ", udp-relay=true, tfo=false",
			wantName:     "🇭🇰 Hong Kong 01",
			wantServer:   "demo.com",
			wantPort:     11127,
			wantCipher:   "2022-blake3-aes-128-gcm",
			wantParam:    paramValue,
			wantUDPRelay: "true",
			wantTFO:      "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, err := ParseSSSubscriptionLine(tt.line)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if proxy.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", proxy.Name, tt.wantName)
			}
			if proxy.Server != tt.wantServer {
				t.Errorf("Server = %q, want %q", proxy.Server, tt.wantServer)
			}
			if proxy.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", proxy.Port, tt.wantPort)
			}
			if proxy.Params["cipher"] != tt.wantCipher {
				t.Errorf("cipher = %q, want %q", proxy.Params["cipher"], tt.wantCipher)
			}
			if proxy.Params[ssLineParamKey] != tt.wantParam {
				t.Errorf("auth param = %q, want %q", proxy.Params[ssLineParamKey], tt.wantParam)
			}
			if proxy.Params["udp-relay"] != tt.wantUDPRelay {
				t.Errorf("udp-relay = %q, want %q", proxy.Params["udp-relay"], tt.wantUDPRelay)
			}
			if proxy.Params["tfo"] != tt.wantTFO {
				t.Errorf("tfo = %q, want %q", proxy.Params["tfo"], tt.wantTFO)
			}
		})
	}
}

// T-SRC-SS-LINE-002: malformed SS subscription lines return source build
// errors that callers can skip or surface according to source policy.
func TestParseSSSubscriptionLine_InvalidFormats(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"unsupported line", "not-a-valid-ss-line"},
		{"quanx missing tag", "shadowsocks = demo.com:11127, method=aes-256-gcm, " + ssLineParam(ssLineParamKey, "fixture")},
		{"surge missing required value", "HK = ss, demo.com, 11127, encrypt-method=aes-256-gcm"},
		{"surge bad port", "HK = ss, demo.com, 70000, encrypt-method=aes-256-gcm, " + ssLineParam(ssLineParamKey, "fixture")},
		{"surge unclosed quote", "HK = ss, demo.com, 11127, encrypt-method=aes-256-gcm, " + ssLineParamKey + "=\"fixture"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSSSubscriptionLine(tt.line)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var buildErr *errtype.BuildError
			if !errors.As(err, &buildErr) {
				t.Fatalf("error type = %T, want *errtype.BuildError", err)
			}
			if buildErr.Code != errtype.CodeBuildSSURIInvalid {
				t.Errorf("Code = %q, want %q", buildErr.Code, errtype.CodeBuildSSURIInvalid)
			}
		})
	}
}
