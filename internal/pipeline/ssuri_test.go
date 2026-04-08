package pipeline

import (
	"errors"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// T-SRC-001: Valid SS URI parsing
func TestParseSSURI_Valid(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		wantName string
		wantHost string
		wantPort int
		cipher   string
		password string
	}{
		{
			name:     "standard URI",
			uri:      "ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-01",
			wantName: "HK-01",
			wantHost: "hk.example.com",
			wantPort: 8388,
			cipher:   "aes-256-cfb",
			password: "password",
		},
		{
			name:     "padded base64",
			uri:      "ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ=@hk.example.com:8388#HK-02",
			wantName: "HK-02",
			wantHost: "hk.example.com",
			wantPort: 8388,
			cipher:   "aes-256-cfb",
			password: "password",
		},
		{
			name:     "URL-encoded CJK fragment",
			uri:      "ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@jp.example.com:8388#JP-%E4%B8%9C%E4%BA%AC-01",
			wantName: "JP-东京-01",
			wantHost: "jp.example.com",
			wantPort: 8388,
			cipher:   "aes-256-cfb",
			password: "password",
		},
		{
			name:     "unencoded CJK fragment",
			uri:      "ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@jp.example.com:8388#JP-东京-01",
			wantName: "JP-东京-01",
			wantHost: "jp.example.com",
			wantPort: 8388,
			cipher:   "aes-256-cfb",
			password: "password",
		},
		{
			name:     "password with colon",
			uri:      "ss://YWVzLTI1Ni1jZmI6cGFzczp3b3Jk@sg.example.com:443#SG-01",
			wantName: "SG-01",
			wantHost: "sg.example.com",
			wantPort: 443,
			cipher:   "aes-256-cfb",
			password: "pass:word",
		},
		{
			name:     "chacha20 cipher",
			uri:      "ss://Y2hhY2hhMjAtaWV0Zi1wb2x5MTMwNTpteXBhc3M@us.example.com:1234#US-01",
			wantName: "US-01",
			wantHost: "us.example.com",
			wantPort: 1234,
			cipher:   "chacha20-ietf-poly1305",
			password: "mypass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, err := ParseSSURI(tt.uri)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if proxy.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", proxy.Name, tt.wantName)
			}
			if proxy.Type != "ss" {
				t.Errorf("Type = %q, want %q", proxy.Type, "ss")
			}
			if proxy.Server != tt.wantHost {
				t.Errorf("Server = %q, want %q", proxy.Server, tt.wantHost)
			}
			if proxy.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", proxy.Port, tt.wantPort)
			}
			if proxy.Params["cipher"] != tt.cipher {
				t.Errorf("cipher = %q, want %q", proxy.Params["cipher"], tt.cipher)
			}
			if proxy.Params["password"] != tt.password {
				t.Errorf("password = %q, want %q", proxy.Params["password"], tt.password)
			}
			if proxy.Kind != model.KindSubscription {
				t.Errorf("Kind = %q, want %q", proxy.Kind, model.KindSubscription)
			}
		})
	}
}

// T-SRC-002: Invalid SS URI returns *errtype.BuildError
func TestParseSSURI_Invalid(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{"missing prefix", "http://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-01"},
		{"no prefix at all", "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-01"},
		{"missing @", "ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ#HK-01"},
		{"missing fragment", "ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388"},
		{"empty fragment", "ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#"},
		{"invalid base64", "ss://!!!invalid!!!@hk.example.com:8388#HK-01"},
		{"empty host", "ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@:8388#HK-01"},
		{"non-numeric port", "ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:abc#HK-01"},
		{"missing port", "ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com#HK-01"},
		{"empty method", "ss://OnBhc3N3b3Jk@hk.example.com:8388#HK-01"},       // base64(":password")
		{"no colon in userinfo", "ss://bm9jb2xvbg@hk.example.com:8388#HK-01"}, // base64("nocolon")
		{"port zero", "ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:0#HK-01"},
		{"port negative", "ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:-1#HK-01"},
		{"port too large", "ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:65536#HK-01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSSURI(tt.uri)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var buildErr *errtype.BuildError
			if !errors.As(err, &buildErr) {
				t.Fatalf("error type = %T, want *errtype.BuildError", err)
			}
			if buildErr.Phase != "source" {
				t.Errorf("Phase = %q, want %q", buildErr.Phase, "source")
			}
		})
	}
}
