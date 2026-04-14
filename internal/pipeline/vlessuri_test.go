package pipeline

import (
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// T-SRC-VLESS-001: ParseVLessURI_Valid
//
// Covers the sample URI shipped by the feature owner (TCP + Reality + Vision),
// plus common variants: tcp+tls with alpn list, tcp+none, non-none encryption
// passthrough, uppercase UUID, URL-escaped CJK fragment, IPv6 server, empty sid.
func TestParseVLessURI_Valid(t *testing.T) {
	cases := []struct {
		name       string
		uri        string
		wantName   string
		wantServer string
		wantPort   int
		wantParams map[string]string
	}{
		{
			name:       "sample reality tcp vision",
			uri:        "vless://b33a72bf-75ab-4be2-b182-223a727b37a5@11.11.11.11:443?encryption=none&flow=xtls-rprx-vision&security=reality&sni=www.cloudflare.com&fp=chrome&pbk=ajfmJbR3k4WVTJkfP4VnGMfj88G9J5xZg0c9VEz_C0k&type=tcp#vless-demo",
			wantName:   "vless-demo",
			wantServer: "11.11.11.11",
			wantPort:   443,
			wantParams: map[string]string{
				"uuid":               "b33a72bf-75ab-4be2-b182-223a727b37a5",
				"encryption":         "none",
				"flow":               "xtls-rprx-vision",
				"security":           "reality",
				"servername":         "www.cloudflare.com",
				"client-fingerprint": "chrome",
				"reality-public-key": "ajfmJbR3k4WVTJkfP4VnGMfj88G9J5xZg0c9VEz_C0k",
				"network":            "tcp",
			},
		},
		{
			name:       "tcp tls with alpn list",
			uri:        "vless://AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE@sg.example.com:443?security=tls&sni=sg.example.com&type=tcp&alpn=h2,http/1.1#SG-TLS",
			wantName:   "SG-TLS",
			wantServer: "sg.example.com",
			wantPort:   443,
			wantParams: map[string]string{
				"uuid":       "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE",
				"security":   "tls",
				"servername": "sg.example.com",
				"network":    "tcp",
				"alpn":       "h2,http/1.1",
			},
		},
		{
			name:       "tcp none no security",
			uri:        "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:8080?security=none&type=tcp#Plain",
			wantName:   "Plain",
			wantServer: "1.2.3.4",
			wantPort:   8080,
			wantParams: map[string]string{
				"uuid":     "11111111-2222-3333-4444-555555555555",
				"security": "none",
				"network":  "tcp",
			},
		},
		{
			name:       "non-none encryption passthrough",
			uri:        "vless://11111111-2222-3333-4444-555555555555@enc.example.com:443?security=tls&sni=enc.example.com&encryption=mlkem768x25519plus.native&type=tcp#Enc",
			wantName:   "Enc",
			wantServer: "enc.example.com",
			wantPort:   443,
			wantParams: map[string]string{
				"uuid":       "11111111-2222-3333-4444-555555555555",
				"security":   "tls",
				"servername": "enc.example.com",
				"encryption": "mlkem768x25519plus.native",
				"network":    "tcp",
			},
		},
		{
			name:       "url-escaped CJK fragment",
			uri:        "vless://11111111-2222-3333-4444-555555555555@hk.example.com:443?security=tls&type=tcp#%E9%A6%99%E6%B8%AF-01",
			wantName:   "香港-01",
			wantServer: "hk.example.com",
			wantPort:   443,
			wantParams: map[string]string{
				"uuid":     "11111111-2222-3333-4444-555555555555",
				"security": "tls",
				"network":  "tcp",
			},
		},
		{
			name:       "ipv6 server",
			uri:        "vless://11111111-2222-3333-4444-555555555555@[2001:db8::1]:443?security=tls&type=tcp#IPv6",
			wantName:   "IPv6",
			wantServer: "2001:db8::1",
			wantPort:   443,
			wantParams: map[string]string{
				"uuid":     "11111111-2222-3333-4444-555555555555",
				"security": "tls",
				"network":  "tcp",
			},
		},
		{
			name:       "reality with empty sid dropped",
			uri:        "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?security=reality&pbk=KEY&sid=&type=tcp#R",
			wantName:   "R",
			wantServer: "1.2.3.4",
			wantPort:   443,
			wantParams: map[string]string{
				"uuid":               "11111111-2222-3333-4444-555555555555",
				"security":           "reality",
				"reality-public-key": "KEY",
				"network":            "tcp",
				// reality-short-id intentionally absent — empty value not stored.
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVLessURI(tt.uri)
			if err != nil {
				t.Fatalf("ParseVLessURI: %v", err)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if got.Type != "vless" {
				t.Errorf("Type = %q, want vless", got.Type)
			}
			if got.Server != tt.wantServer {
				t.Errorf("Server = %q, want %q", got.Server, tt.wantServer)
			}
			if got.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", got.Port, tt.wantPort)
			}
			if got.Kind != model.KindVLess {
				t.Errorf("Kind = %q, want %q", got.Kind, model.KindVLess)
			}
			if len(got.Params) != len(tt.wantParams) {
				t.Errorf("Params len = %d, want %d; got = %v", len(got.Params), len(tt.wantParams), got.Params)
			}
			for k, want := range tt.wantParams {
				if gv := got.Params[k]; gv != want {
					t.Errorf("Params[%q] = %q, want %q", k, gv, want)
				}
			}
		})
	}
}

// T-SRC-VLESS-002: ParseVLessURI_Invalid
//
// Exercises rejection paths for malformed URIs and invalid specific fields.
// Unknown `type` values are not invalid: they normalize to tcp in
// T-SRC-VLESS-002b.
func TestParseVLessURI_Invalid(t *testing.T) {
	cases := []struct {
		name string
		uri  string
	}{
		{"missing prefix", "vmess://11111111-2222-3333-4444-555555555555@1.2.3.4:443#x"},
		{"missing at separator", "vless://11111111-2222-3333-4444-5555555555551.2.3.4:443#x"},
		{"empty uuid", "vless://@1.2.3.4:443#x"},
		{"malformed uuid short", "vless://ABC@1.2.3.4:443#x"},
		{"malformed uuid non-hex", "vless://ZZZZZZZZ-2222-3333-4444-555555555555@1.2.3.4:443#x"},
		{"missing fragment", "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443"},
		{"empty fragment", "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443#"},
		{"empty host", "vless://11111111-2222-3333-4444-555555555555@:443#x"},
		{"non-numeric port", "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:abc#x"},
		{"port zero", "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:0#x"},
		{"port above range", "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:70000#x"},
		{"unknown security", "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?security=xtls#x"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseVLessURI(tt.uri)
			if err == nil {
				t.Fatalf("ParseVLessURI(%q) = nil error, want failure", tt.uri)
			}
			var be *errtype.BuildError
			if !errors.As(err, &be) {
				t.Fatalf("error %T is not *BuildError", err)
			}
			if be.Code != errtype.CodeBuildVLessURIInvalid {
				t.Errorf("Code = %q, want %q", be.Code, errtype.CodeBuildVLessURIInvalid)
			}
			if be.Phase != "source" {
				t.Errorf("Phase = %q, want source", be.Phase)
			}
		})
	}
}

// T-SRC-VLESS-002b: Network values are normalized to Mihomo-compatible
// transport names. Known values in {tcp, ws, http, h2, grpc, xhttp} are kept;
// missing or unknown values fall back to tcp.
func TestParseVLessURI_NetworkNormalization(t *testing.T) {
	baseTemplate := "vless://11111111-2222-3333-4444-555555555555@example.com:443?security=tls&sni=a.b&type=%s#n"

	t.Run("known networks preserved", func(t *testing.T) {
		for _, network := range []string{"tcp", "ws", "http", "h2", "grpc", "xhttp"} {
			t.Run(network, func(t *testing.T) {
				uri := strings.Replace(baseTemplate, "%s", network, 1)
				got, err := ParseVLessURI(uri)
				if err != nil {
					t.Fatalf("ParseVLessURI(%q) errored: %v", uri, err)
				}
				if got.Params["network"] != network {
					t.Errorf("Params[network] = %q, want %q", got.Params["network"], network)
				}
			})
		}
	})

	t.Run("unknown networks fall back to tcp", func(t *testing.T) {
		for _, network := range []string{"kcp", "httpupgrade", "quic", "unknown-future-transport"} {
			t.Run(network, func(t *testing.T) {
				uri := strings.Replace(baseTemplate, "%s", network, 1)
				got, err := ParseVLessURI(uri)
				if err != nil {
					t.Fatalf("ParseVLessURI(%q) errored: %v", uri, err)
				}
				if got.Params["network"] != "tcp" {
					t.Errorf("Params[network] = %q, want tcp fallback", got.Params["network"])
				}
			})
		}
	})

	t.Run("missing type defaults to tcp", func(t *testing.T) {
		uri := "vless://11111111-2222-3333-4444-555555555555@example.com:443?security=tls&sni=a.b#n"
		got, err := ParseVLessURI(uri)
		if err != nil {
			t.Fatalf("missing type should default to tcp, got err: %v", err)
		}
		if got.Params["network"] != "tcp" {
			t.Errorf("Params[network] = %q, want tcp (default)", got.Params["network"])
		}
	})
}

// T-SRC-VLESS-002c: Per-network transport query dispatch — path/host/
// serviceName/mode land in transport-specific Params keys so the renderer
// can emit the matching *-opts block without runtime dispatch.
func TestParseVLessURI_TransportQueryDispatch(t *testing.T) {
	cases := []struct {
		name       string
		uri        string
		wantParams map[string]string
		forbidKeys []string
	}{
		{
			name: "ws path+host",
			uri:  "vless://11111111-2222-3333-4444-555555555555@x.com:443?security=tls&type=ws&path=/ws-path&host=ws.example.com#n",
			wantParams: map[string]string{
				"network": "ws",
				"ws-path": "/ws-path",
				"ws-host": "ws.example.com",
			},
			forbidKeys: []string{"path", "host", "http-path", "h2-path", "xhttp-path"},
		},
		{
			name: "http path+host",
			uri:  "vless://11111111-2222-3333-4444-555555555555@x.com:443?security=tls&type=http&path=/api&host=api.example.com#n",
			wantParams: map[string]string{
				"network":   "http",
				"http-path": "/api",
				"http-host": "api.example.com",
			},
			forbidKeys: []string{"ws-path", "h2-path", "xhttp-path"},
		},
		{
			name: "h2 path+host",
			uri:  "vless://11111111-2222-3333-4444-555555555555@x.com:443?security=tls&type=h2&path=/grpc&host=h2.example.com#n",
			wantParams: map[string]string{
				"network": "h2",
				"h2-path": "/grpc",
				"h2-host": "h2.example.com",
			},
			forbidKeys: []string{"ws-path", "http-path", "xhttp-path"},
		},
		{
			name: "grpc serviceName",
			uri:  "vless://11111111-2222-3333-4444-555555555555@x.com:443?security=tls&type=grpc&serviceName=GunService#n",
			wantParams: map[string]string{
				"network":           "grpc",
				"grpc-service-name": "GunService",
			},
			forbidKeys: []string{"ws-path", "http-path", "h2-path", "xhttp-path"},
		},
		{
			name: "xhttp mode+path+host",
			uri:  "vless://11111111-2222-3333-4444-555555555555@x.com:443?security=tls&type=xhttp&mode=packet-up&path=/xh&host=xh.example.com#n",
			wantParams: map[string]string{
				"network":    "xhttp",
				"xhttp-mode": "packet-up",
				"xhttp-path": "/xh",
				"xhttp-host": "xh.example.com",
			},
			forbidKeys: []string{"ws-path", "http-path", "h2-path"},
		},
		{
			name: "tcp network: path/host in URI are ignored",
			uri:  "vless://11111111-2222-3333-4444-555555555555@x.com:443?security=tls&type=tcp&path=/ignored&host=ignored.example.com#n",
			wantParams: map[string]string{
				"network": "tcp",
			},
			forbidKeys: []string{"ws-path", "http-path", "h2-path", "xhttp-path", "ws-host"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseVLessURI(tc.uri)
			if err != nil {
				t.Fatalf("ParseVLessURI: %v", err)
			}
			for k, want := range tc.wantParams {
				if got.Params[k] != want {
					t.Errorf("Params[%q] = %q, want %q", k, got.Params[k], want)
				}
			}
			for _, k := range tc.forbidKeys {
				if _, exists := got.Params[k]; exists {
					t.Errorf("Params should NOT contain %q for network=%q (dispatcher should route elsewhere)", k, tc.wantParams["network"])
				}
			}
		})
	}
}

// T-SRC-VLESS-003: Key naming in Params follows Clash target names.
//
// Guards the domain-model convention that parsers rename URI query keys
// to Clash YAML keys (e.g. sni→servername, fp→client-fingerprint), so the
// renderer reads Params directly without a rename lookup.
func TestParseVLessURI_KeyNamingMatchesClashTarget(t *testing.T) {
	uri := "vless://b33a72bf-75ab-4be2-b182-223a727b37a5@11.11.11.11:443?security=reality&sni=www.cloudflare.com&fp=chrome&pbk=KEY&sid=SHORT&type=tcp#x"

	got, err := ParseVLessURI(uri)
	if err != nil {
		t.Fatalf("ParseVLessURI: %v", err)
	}

	mustHave := map[string]string{
		"servername":         "www.cloudflare.com",
		"client-fingerprint": "chrome",
		"reality-public-key": "KEY",
		"reality-short-id":   "SHORT",
		"network":            "tcp",
	}
	mustNotHave := []string{"sni", "fp", "pbk", "sid", "type"}

	for k, want := range mustHave {
		if got.Params[k] != want {
			t.Errorf("Params[%q] = %q, want %q", k, got.Params[k], want)
		}
	}
	for _, k := range mustNotHave {
		if _, exists := got.Params[k]; exists {
			t.Errorf("Params should NOT carry URI-native key %q", k)
		}
	}
}

// T-SRC-VLESS-004: Kind assertion — returned Proxy carries KindVLess.
//
// Sibling of TestParseSnellSurgeLine's Kind check.
func TestParseVLessURI_KindIsKindVLess(t *testing.T) {
	uri := "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?security=tls&type=tcp#n"
	got, err := ParseVLessURI(uri)
	if err != nil {
		t.Fatalf("ParseVLessURI: %v", err)
	}
	if got.Kind != model.KindVLess {
		t.Errorf("Kind = %q, want %q", got.Kind, model.KindVLess)
	}
	if got.Type != "vless" {
		t.Errorf("Type = %q, want vless", got.Type)
	}
}
