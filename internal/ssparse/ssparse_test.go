package ssparse

import (
	"testing"

	"github.com/John-Robertt/subconverter/internal/model"
)

func TestParseBody_Valid(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		keepFragment bool
		wantName     string
		wantServer   string
		wantPort     int
		wantCipher   string
		wantPassword string
		wantPlugin   *model.Plugin
	}{
		{
			name:         "base64 userinfo",
			body:         "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388#HK-01",
			keepFragment: true,
			wantName:     "HK-01",
			wantServer:   "hk.example.com",
			wantPort:     8388,
			wantCipher:   "aes-256-cfb",
			wantPassword: "password",
		},
		{
			name:         "padded base64",
			body:         "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ=@hk.example.com:8388#HK-02",
			keepFragment: true,
			wantName:     "HK-02",
			wantServer:   "hk.example.com",
			wantPort:     8388,
			wantCipher:   "aes-256-cfb",
			wantPassword: "password",
		},
		{
			name:         "plain userinfo percent-encoded",
			body:         "aes-256-gcm:pass%3Aword@plain.example.com:8443#PLAIN-01",
			keepFragment: true,
			wantName:     "PLAIN-01",
			wantServer:   "plain.example.com",
			wantPort:     8443,
			wantCipher:   "aes-256-gcm",
			wantPassword: "pass:word",
		},
		{
			name:         "fragment stripped when keepFragment=false",
			body:         "YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388#SomeName",
			keepFragment: false,
			wantName:     "",
			wantServer:   "1.2.3.4",
			wantPort:     8388,
			wantCipher:   "aes-256-gcm",
			wantPassword: "mypass",
		},
		{
			name:         "no fragment at all",
			body:         "YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388",
			keepFragment: true,
			wantName:     "",
			wantServer:   "1.2.3.4",
			wantPort:     8388,
			wantCipher:   "aes-256-gcm",
			wantPassword: "mypass",
		},
		{
			name:         "URL-encoded CJK fragment",
			body:         "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@jp.example.com:8388#JP-%E4%B8%9C%E4%BA%AC-01",
			keepFragment: true,
			wantName:     "JP-东京-01",
			wantServer:   "jp.example.com",
			wantPort:     8388,
			wantCipher:   "aes-256-cfb",
			wantPassword: "password",
		},
		{
			name:         "password with colon in base64",
			body:         "YWVzLTI1Ni1jZmI6cGFzczp3b3Jk@sg.example.com:443#SG-01",
			keepFragment: true,
			wantName:     "SG-01",
			wantServer:   "sg.example.com",
			wantPort:     443,
			wantCipher:   "aes-256-cfb",
			wantPassword: "pass:word",
		},
		{
			name:         "query group ignored, plugin absent",
			body:         "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388/?group=Example#HK-03",
			keepFragment: true,
			wantName:     "HK-03",
			wantServer:   "hk.example.com",
			wantPort:     8388,
			wantCipher:   "aes-256-cfb",
			wantPassword: "password",
		},
		{
			name:         "simple obfs plugin",
			body:         "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388/?plugin=simple-obfs%3Bobfs%3Dhttp%3Bobfs-host%3Dcdn.example.com#HK-04",
			keepFragment: true,
			wantName:     "HK-04",
			wantServer:   "hk.example.com",
			wantPort:     8388,
			wantCipher:   "aes-256-cfb",
			wantPassword: "password",
			wantPlugin:   &model.Plugin{Name: "simple-obfs", Opts: map[string]string{"obfs": "http", "obfs-host": "cdn.example.com"}},
		},
		{
			name:         "v2ray-plugin with flag option",
			body:         "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388/?plugin=v2ray-plugin%3Bmode%3Dwebsocket%3Bserver#HK-05",
			keepFragment: true,
			wantName:     "HK-05",
			wantServer:   "hk.example.com",
			wantPort:     8388,
			wantCipher:   "aes-256-cfb",
			wantPassword: "password",
			wantPlugin:   &model.Plugin{Name: "v2ray-plugin", Opts: map[string]string{"mode": "websocket", "server": ""}},
		},
		{
			name:         "escaped plugin option separators",
			body:         "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388/?plugin=simple-obfs%3Bobfs%3Dhttp%3Bobfs-host%3Dcdn\\%3Ddemo\\%3Bedge#HK-06",
			keepFragment: true,
			wantName:     "HK-06",
			wantServer:   "hk.example.com",
			wantPort:     8388,
			wantCipher:   "aes-256-cfb",
			wantPassword: "password",
			wantPlugin:   &model.Plugin{Name: "simple-obfs", Opts: map[string]string{"obfs": "http", "obfs-host": "cdn=demo;edge"}},
		},
		{
			name:         "port boundary 65535",
			body:         "YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:65535",
			keepFragment: false,
			wantServer:   "1.2.3.4",
			wantPort:     65535,
			wantCipher:   "aes-256-gcm",
			wantPassword: "mypass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := ParseBody(tt.body, tt.keepFragment)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", r.Name, tt.wantName)
			}
			if r.Server != tt.wantServer {
				t.Errorf("Server = %q, want %q", r.Server, tt.wantServer)
			}
			if r.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", r.Port, tt.wantPort)
			}
			if r.Cipher != tt.wantCipher {
				t.Errorf("Cipher = %q, want %q", r.Cipher, tt.wantCipher)
			}
			if r.Password != tt.wantPassword {
				t.Errorf("Password = %q, want %q", r.Password, tt.wantPassword)
			}
			if tt.wantPlugin == nil {
				if r.Plugin != nil {
					t.Errorf("Plugin = %#v, want nil", r.Plugin)
				}
			} else {
				if r.Plugin == nil {
					t.Fatalf("Plugin = nil, want %#v", tt.wantPlugin)
				}
				if r.Plugin.Name != tt.wantPlugin.Name {
					t.Errorf("Plugin.Name = %q, want %q", r.Plugin.Name, tt.wantPlugin.Name)
				}
				if len(r.Plugin.Opts) != len(tt.wantPlugin.Opts) {
					t.Fatalf("Plugin.Opts len = %d, want %d", len(r.Plugin.Opts), len(tt.wantPlugin.Opts))
				}
				for key, want := range tt.wantPlugin.Opts {
					if got := r.Plugin.Opts[key]; got != want {
						t.Errorf("Plugin.Opts[%q] = %q, want %q", key, got, want)
					}
				}
			}
		})
	}
}

func TestParseBody_Invalid(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"missing @", "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ"},
		{"invalid base64 userinfo", "!!!invalid!!!@hk.example.com:8388"},
		{"empty host", "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@:8388"},
		{"non-numeric port", "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:abc"},
		{"missing port", "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com"},
		{"empty method", "OnBhc3N3b3Jk@hk.example.com:8388"},
		{"no colon in userinfo", "bm9jb2xvbg@hk.example.com:8388"},
		{"port zero", "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:0"},
		{"port too large", "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:65536"},
		{"invalid query", "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388?plugin=%zz"},
		{"trailing escape in plugin", "YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@hk.example.com:8388?plugin=simple-obfs%3Bobfs%3Dhttp%3Bobfs-host%3Dcdn\\"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseBody(tt.body, false)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}
