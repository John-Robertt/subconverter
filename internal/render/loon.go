package render

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// Loon serializes projected nodes, chains, groups and rules into a Loon profile.
func Loon(p *model.TargetPipeline, baseTemplate []byte) ([]byte, error) {
	var proxies, chains, rules, remoteRules strings.Builder
	for _, proxy := range p.Proxies {
		line, err := renderLoonProxy(proxy)
		if err != nil {
			return nil, &errtype.RenderError{
				Code: errtype.CodeRenderLoonProxyInvalid, Format: "loon",
				Message: fmt.Sprintf("代理 %q 渲染失败：%v", proxy.Name, err), Cause: err,
			}
		}
		proxies.WriteString(line + "\n")
	}
	for _, chain := range p.ProxyChains {
		fmt.Fprintf(&chains, "%s = %s", chain.Name, strings.Join(chain.Members, ", "))
		if chain.UDP {
			chains.WriteString(", udp=true")
		}
		chains.WriteByte('\n')
	}
	for _, rule := range p.Rules {
		rules.WriteString(rule.Raw + "\n")
	}
	fmt.Fprintf(&rules, "FINAL,%s\n", p.Fallback)
	for _, ruleset := range p.Rulesets {
		for _, rawURL := range ruleset.URLs {
			fmt.Fprintf(&remoteRules, "%s,%s\n", rawURL, ruleset.Policy)
		}
	}
	sections := map[string]string{
		"[Proxy]":       proxies.String(),
		"[Proxy Chain]": chains.String(),
		"[Proxy Group]": buildINIGroups(p.NodeGroups, p.RouteGroups),
		"[Rule]":        rules.String(),
		"[Remote Rule]": remoteRules.String(),
	}
	return []byte(mergeINISections(baseTemplate, sections, []string{
		"[Proxy]", "[Proxy Chain]", "[Proxy Group]", "[Rule]", "[Remote Rule]",
	})), nil
}

func renderLoonProxy(proxy model.TargetProxy) (string, error) {
	if proxy.Dialer != "" {
		return "", fmt.Errorf("链式代理尚未转换为 Loon 链路")
	}
	if proxy.Native != nil {
		if proxy.Native.Format != model.NativeProxyFormatLoon {
			return "", fmt.Errorf("原生声明格式为 %q，不能写入 Loon", proxy.Native.Format)
		}
		return proxy.Name + " = " + proxy.Native.Payload, nil
	}
	parts := []string{proxy.Name + " = " + proxy.Type, proxy.Server, strconv.Itoa(proxy.Port)}
	switch proxy.Type {
	case "ss":
		parts[0] = proxy.Name + " = Shadowsocks"
		parts = append(parts, proxy.Params["cipher"], strconv.Quote(proxy.Params["password"]))
		plugin, err := renderSimpleObfsPlugin(proxy.Plugin, "obfs-name")
		if err != nil {
			return "", err
		}
		parts = append(parts, plugin...)
	case "anytls":
		parts = append(parts, strconv.Quote(proxy.Params["password"]))
		parts = appendLoonParams(parts, proxy.Params, [][2]string{
			{"sni", "sni"}, {"skip-cert-verify", "skip-cert-verify"},
			{"server-cert-fingerprint-sha256", "tls-cert-sha256"},
			{"idle-session-timeout", "idle-session-timeout"},
		})
	case "socks5", "http":
		if proxy.Params["username"] != "" || proxy.Params["password"] != "" {
			username := proxy.Params["username"]
			if username == "" || iniValueNeedsQuote(username) || !strconv.CanBackquote(username) {
				username = strconv.Quote(username)
			}
			parts = append(parts, username, strconv.Quote(proxy.Params["password"]))
		}
	case "vless":
		parts = append(parts, strconv.Quote(proxy.Params["uuid"]))
		parts = append(parts, renderINIKeyValue("transport", proxy.Params["network"]))
		parts = appendLoonParams(parts, proxy.Params, [][2]string{
			{"flow", "flow"}, {"servername", "sni"},
			{"skip-cert-verify", "skip-cert-verify"},
			{"reality-public-key", "public-key"}, {"reality-short-id", "short-id"},
		})
		if security := proxy.Params["security"]; security == "tls" || security == "reality" {
			parts = append(parts, "over-tls=true")
			parts = appendLoonParams(parts, proxy.Params, [][2]string{{"client-fingerprint", "tls-profile"}})
		}
		switch proxy.Params["network"] {
		case "ws":
			parts = appendLoonParams(parts, proxy.Params, [][2]string{{"ws-path", "path"}, {"ws-host", "host"}})
		case "http":
			parts = appendLoonParams(parts, proxy.Params, [][2]string{{"http-path", "path"}, {"http-host", "host"}})
		case "tcp":
		default:
			return "", fmt.Errorf("不支持 VLESS 传输 %q", proxy.Params["network"])
		}
	default:
		return "", fmt.Errorf("不支持协议 %q", proxy.Type)
	}
	if (proxy.Type == "anytls" || proxy.Type == "vless") && proxy.Params["udp-relay"] == "" {
		parts = append(parts, "udp=true")
	}
	parts = appendLoonParams(parts, proxy.Params, [][2]string{{"tfo", "fast-open"}, {"udp-relay", "udp"}})
	return strings.Join(parts, ", "), nil
}

func appendLoonParams(parts []string, params map[string]string, mappings [][2]string) []string {
	for _, mapping := range mappings {
		if value := params[mapping[0]]; value != "" {
			parts = append(parts, renderINIKeyValue(mapping[1], value))
		}
	}
	return parts
}
