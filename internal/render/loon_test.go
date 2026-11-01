package render

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

func loonRenderFixture() *model.TargetPipeline {
	return &model.TargetPipeline{
		Proxies: []model.TargetProxy{
			{Name: "HK", Type: "ss", Server: "hk.example.com", Port: 443, Params: map[string]string{"cipher": "aes-128-gcm", "password": "pw,quoted", "udp-relay": "true", "tfo": "true"}},
			{Name: "ANY", Type: "anytls", Native: &model.NativeProxy{Format: model.NativeProxyFormatLoon, Payload: `anytls, any.example.com, 443, "native-pw", tls-name=edge.example.com, udp=true`}},
			{Name: "EXIT", Type: "socks5", Server: "exit.example.com", Port: 1080, Params: map[string]string{"username": "user", "password": "pass"}},
		},
		ProxyChains: []model.TargetChain{{Name: "CHAIN", Members: []string{"HK", "EXIT"}, UDP: true}},
		NodeGroups:  []model.TargetGroup{{Name: "HK_GROUP", Strategy: "url-test", Members: []string{"HK", "ANY"}}, {Name: "CHAIN_GROUP", Strategy: "select", Members: []string{"CHAIN"}}},
		RouteGroups: []model.TargetGroup{{Name: "OUT", Strategy: "select", Members: []string{"HK_GROUP", "CHAIN_GROUP", "DIRECT"}}},
		Rulesets:    []model.Ruleset{{Policy: "OUT", URLs: []string{"https://example.com/a.list", "https://example.com/b.list"}}},
		Rules:       []model.Rule{{Raw: "GEOIP,CN,DIRECT", Policy: "DIRECT"}}, Fallback: "OUT",
	}
}

// T-RND-LOON-001: the golden covers native and normalized nodes, chains, ordered groups and rulesets.
func TestLoon_GoldenNoTemplate(t *testing.T) {
	got, err := Loon(loonRenderFixture(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "render", "loon_golden.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("Loon golden mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// T-RND-LOON-002: all generated sections replace provider content while base settings survive.
func TestLoon_TemplateMerge(t *testing.T) {
	template := []byte(`#!MANAGED-CONFIG https://example.com/old-surge
[General]
dns-server = system
[Proxy]
OLD_NODE = invalid
[Proxy Chain]
OLD_CHAIN = invalid
[Proxy Group]
OLD_GROUP = select, MISSING
[Rule]
FINAL,OLD_GROUP
[Remote Rule]
https://example.com/old.list,OLD_GROUP
[Host]
node.example.com = alias.example.com
[Rewrite]
# base rewrite
[Script]
# base script
[MITM]
hostname = *.example.com
`)
	got, err := Loon(loonRenderFixture(), template)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"dns-server = system", "node.example.com = alias.example.com", "# base rewrite", "# base script", "hostname = *.example.com", "https://example.com/a.list,OUT"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"OLD_", "old.list", "MANAGED-CONFIG"} {
		if strings.Contains(string(got), unwanted) {
			t.Errorf("output retains %q:\n%s", unwanted, got)
		}
	}
}

// T-RND-LOON-003: normalized fields use Loon's positional credentials and parameter names.
func TestLoon_NormalizedProtocols(t *testing.T) {
	for _, tt := range []struct {
		name  string
		proxy model.TargetProxy
		want  []string
	}{
		{"anytls", model.TargetProxy{Name: "ANY", Type: "anytls", Server: "any.example.com", Port: 443, Params: map[string]string{"password": "p,=w", "sni": "edge.example.com", "skip-cert-verify": "true", "server-cert-fingerprint-sha256": "fingerprint", "idle-session-timeout": "30"}}, []string{`443, "p,=w"`, "sni=edge.example.com", "skip-cert-verify=true", "tls-cert-sha256=fingerprint", "idle-session-timeout=30"}},
		{"http", model.TargetProxy{Name: "HTTP", Type: "http", Server: "proxy.example.com", Port: 80, Params: map[string]string{"username": "a,b", "password": "pass"}}, []string{`80, "a,b", "pass"`}},
		{"vless reality", model.TargetProxy{Name: "VL", Type: "vless", Server: "vless.example.com", Port: 443, Params: map[string]string{"uuid": "uuid", "network": "tcp", "security": "reality", "flow": "xtls-rprx-vision", "servername": "edge.example.com", "reality-public-key": "pub", "reality-short-id": "ab"}}, []string{`443, "uuid"`, "transport=tcp", "flow=xtls-rprx-vision", "sni=edge.example.com", "public-key=pub", "short-id=ab", "over-tls=true"}},
		{"vless ws", model.TargetProxy{Name: "WS", Type: "vless", Server: "ws.example.com", Port: 80, Params: map[string]string{"uuid": "uuid", "network": "ws", "ws-path": "/ws", "ws-host": "edge.example.com"}}, []string{"transport=ws", "path=/ws", "host=edge.example.com"}},
		{"ss obfs", model.TargetProxy{Name: "SS", Type: "ss", Server: "ss.example.com", Port: 443, Params: map[string]string{"cipher": "aes-128-gcm", "password": "pw"}, Plugin: &model.Plugin{Name: "obfs-local", Opts: map[string]string{"obfs": "tls", "obfs-host": "edge.example.com"}}}, []string{`Shadowsocks, ss.example.com, 443, aes-128-gcm, "pw"`, "obfs-name=tls", "obfs-host=edge.example.com"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderLoonProxy(tt.proxy)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("line=%q missing %q", got, want)
				}
			}
		})
	}
}

// T-RND-LOON-004: incompatible native declarations, protocols and unprojected chains fail explicitly.
func TestLoon_InvalidProxy(t *testing.T) {
	for _, proxy := range []model.TargetProxy{
		{Name: "WRONG", Native: &model.NativeProxy{Format: model.NativeProxyFormatSurge, Payload: "ss,..."}},
		{Name: "SNELL", Type: "snell"},
		{Name: "CHAIN", Type: "socks5", Dialer: "ENTRY"},
		{Name: "PLUGIN", Type: "ss", Plugin: &model.Plugin{Name: "v2ray-plugin"}},
	} {
		t.Run(proxy.Name, func(t *testing.T) {
			_, err := Loon(&model.TargetPipeline{Proxies: []model.TargetProxy{proxy}}, nil)
			var renderErr *errtype.RenderError
			if !errors.As(err, &renderErr) || renderErr.Code != errtype.CodeRenderLoonProxyInvalid || !strings.Contains(err.Error(), proxy.Name) {
				t.Fatalf("error=%v, want named Loon RenderError", err)
			}
		})
	}
}
