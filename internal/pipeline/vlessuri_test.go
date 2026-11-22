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
// 覆盖功能负责人提供的样例 URI（TCP + Reality + Vision），以及常见变体：
// 带 alpn 列表的 tcp+tls、tcp+none、非 none 的 encryption 原样透传、
// 大写 UUID、URL 转义的 CJK 片段、IPv6 服务器、空 sid。
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
				// 有意不含 reality-short-id：空值不会被存储。
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
				t.Errorf("Name = %q，期望 %q", got.Name, tt.wantName)
			}
			if got.Type != "vless" {
				t.Errorf("Type = %q，期望 vless", got.Type)
			}
			if got.Server != tt.wantServer {
				t.Errorf("Server = %q，期望 %q", got.Server, tt.wantServer)
			}
			if got.Port != tt.wantPort {
				t.Errorf("Port = %d，期望 %d", got.Port, tt.wantPort)
			}
			if got.Kind != model.KindVLess {
				t.Errorf("Kind = %q，期望 %q", got.Kind, model.KindVLess)
			}
			if len(got.Params) != len(tt.wantParams) {
				t.Errorf("Params 长度 = %d，期望 %d；实际 = %v", len(got.Params), len(tt.wantParams), got.Params)
			}
			for k, want := range tt.wantParams {
				if gv := got.Params[k]; gv != want {
					t.Errorf("Params[%q] = %q，期望 %q", k, gv, want)
				}
			}
		})
	}
}

// T-SRC-VLESS-002: ParseVLessURI_Invalid
//
// 验证格式错误的 URI 和取值非法的特定字段会被拒绝。
// 未知的 `type` 值不算非法：它们会被归一化为 tcp，见
// T-SRC-VLESS-002b。
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
				t.Fatalf("ParseVLessURI(%q) 未返回错误，期望失败", tt.uri)
			}
			var be *errtype.BuildError
			if !errors.As(err, &be) {
				t.Fatalf("错误 %T 不是 *BuildError", err)
			}
			if be.Code != errtype.CodeBuildVLessURIInvalid {
				t.Errorf("Code = %q，期望 %q", be.Code, errtype.CodeBuildVLessURIInvalid)
			}
			if be.Phase != "source" {
				t.Errorf("Phase = %q，期望 source", be.Phase)
			}
		})
	}
}

// T-SRC-VLESS-002b: network 值被归一化为 Mihomo 兼容的传输名称。
// {tcp, ws, http, h2, grpc, xhttp} 中的已知值保持不变；
// 缺失或未知的值回退为 tcp。
func TestParseVLessURI_NetworkNormalization(t *testing.T) {
	baseTemplate := "vless://11111111-2222-3333-4444-555555555555@example.com:443?security=tls&sni=a.b&type=%s#n"

	t.Run("known networks preserved", func(t *testing.T) {
		for _, network := range []string{"tcp", "ws", "http", "h2", "grpc", "xhttp"} {
			t.Run(network, func(t *testing.T) {
				uri := strings.Replace(baseTemplate, "%s", network, 1)
				got, err := ParseVLessURI(uri)
				if err != nil {
					t.Fatalf("ParseVLessURI(%q) 出错：%v", uri, err)
				}
				if got.Params["network"] != network {
					t.Errorf("Params[network] = %q，期望 %q", got.Params["network"], network)
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
					t.Fatalf("ParseVLessURI(%q) 出错：%v", uri, err)
				}
				if got.Params["network"] != "tcp" {
					t.Errorf("Params[network] = %q，期望回退为 tcp", got.Params["network"])
				}
			})
		}
	})

	t.Run("missing type defaults to tcp", func(t *testing.T) {
		uri := "vless://11111111-2222-3333-4444-555555555555@example.com:443?security=tls&sni=a.b#n"
		got, err := ParseVLessURI(uri)
		if err != nil {
			t.Fatalf("缺少 type 时应默认为 tcp，实际出错：%v", err)
		}
		if got.Params["network"] != "tcp" {
			t.Errorf("Params[network] = %q，期望 tcp（默认值）", got.Params["network"])
		}
	})

	t.Run("no query defaults to tcp and satisfies proxy invariant", func(t *testing.T) {
		for _, uri := range []string{
			"vless://11111111-2222-3333-4444-555555555555@example.com:443#bare",
			"vless://11111111-2222-3333-4444-555555555555@example.com:443?#empty-query",
		} {
			got, err := ParseVLessURI(uri)
			if err != nil {
				t.Fatalf("ParseVLessURI(%q) 出错：%v", uri, err)
			}
			if got.Params["network"] != "tcp" {
				t.Errorf("ParseVLessURI(%q) Params[network] = %q，期望 tcp", uri, got.Params["network"])
			}
			got.ID = 1
			if err := model.ValidateProxyInvariant(got); err != nil {
				t.Errorf("ParseVLessURI(%q) 不满足节点不变量：%v", uri, err)
			}
		}
	})
}

// T-SRC-VLESS-002c: 按 network 分派传输相关的查询参数：path/host/
// serviceName/mode 落到各传输专属的 Params 键中，渲染器无需在运行时
// 再分派，就能输出对应的 *-opts 块。
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
					t.Errorf("Params[%q] = %q，期望 %q", k, got.Params[k], want)
				}
			}
			for _, k := range tc.forbidKeys {
				if _, exists := got.Params[k]; exists {
					t.Errorf("Params 不应包含 %q（network=%q，分派逻辑应把它放到别处）", k, tc.wantParams["network"])
				}
			}
		})
	}
}

// T-SRC-VLESS-003: Params 中的键名沿用 Clash 目标格式的名称。
//
// 守护领域模型的约定：解析器把 URI 查询键改名为 Clash YAML 键
// （例如 sni→servername、fp→client-fingerprint），渲染器因此直接读取
// Params，无需再查改名表。
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
			t.Errorf("Params[%q] = %q，期望 %q", k, got.Params[k], want)
		}
	}
	for _, k := range mustNotHave {
		if _, exists := got.Params[k]; exists {
			t.Errorf("Params 不应携带 URI 原生键 %q", k)
		}
	}
}

// T-SRC-VLESS-004: Kind 断言，返回的 Proxy 带有 KindVLess。
//
// 与 TestParseSnellSurgeLine 中的 Kind 检查相对应。
func TestParseVLessURI_KindIsKindVLess(t *testing.T) {
	uri := "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?security=tls&type=tcp#n"
	got, err := ParseVLessURI(uri)
	if err != nil {
		t.Fatalf("ParseVLessURI: %v", err)
	}
	if got.Kind != model.KindVLess {
		t.Errorf("Kind = %q，期望 %q", got.Kind, model.KindVLess)
	}
	if got.Type != "vless" {
		t.Errorf("Type = %q，期望 vless", got.Type)
	}
}
