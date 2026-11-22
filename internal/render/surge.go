package render

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

const (
	surgeManagedInterval = 86400
)

// surgeSnellKeyOrder 固定 Snell 参数写入 Surge 节点行的顺序，让输出确定
// （golden 文件比对需要这一点），并与 jinqians/snell.sh 的输出风格一致。
// Proxy.Params 中的未知键有意不写出：Snell 行只来自 ParseSnellSurgeLine，
// 它是这种类型唯一的入口，所以合法键的集合是有限且已知的。
var surgeSnellKeyOrder = []string{
	"psk",
	"version",
	"obfs",
	"obfs-host",
	"obfs-uri",
	"reuse",
	"tfo",
	"udp-relay",
	"udp-port",
	"shadow-tls-password",
	"shadow-tls-sni",
	"shadow-tls-version",
}

var surgeSSKeyOrder = []string{
	"udp-relay",
	"tfo",
}

var surgeAnyTLSKeyOrder = []string{
	"sni",
	"skip-cert-verify",
	"reuse",
	"server-cert-fingerprint-sha256",
}

// Surge 把已按目标客户端投影的流水线结果渲染为 Surge conf 格式。
// managedURL 用于 #!MANAGED-CONFIG 头部（为空则不写头部）。
// baseTemplate 非 nil 时，生成的段落替换模板中对应的段落。
func Surge(p *model.TargetPipeline, managedURL string, baseTemplate []byte) ([]byte, error) {
	proxySection, err := buildSurgeProxies(p.Proxies)
	if err != nil {
		return nil, err
	}
	groupSection := buildINIGroups(p.NodeGroups, p.RouteGroups)
	ruleSection := buildSurgeRules(p.Rulesets, p.Rules, p.Fallback)

	var buf bytes.Buffer

	// managed config 头部。
	if managedURL != "" {
		fmt.Fprintf(&buf, "#!MANAGED-CONFIG %s interval=%d strict=false\n\n",
			managedURL, surgeManagedInterval)
	}

	if baseTemplate != nil {
		merged := mergeSurgeTemplate(baseTemplate, proxySection, groupSection, ruleSection)
		buf.WriteString(merged)
	} else {
		buf.WriteString("[Proxy]\n")
		buf.WriteString(proxySection)
		buf.WriteString("\n[Proxy Group]\n")
		buf.WriteString(groupSection)
		buf.WriteString("\n[Rule]\n")
		buf.WriteString(ruleSection)
	}

	return buf.Bytes(), nil
}

func buildSurgeProxies(proxies []model.TargetProxy) (string, error) {
	var sb strings.Builder
	for _, px := range proxies {
		line, err := renderSurgeProxy(px)
		if err != nil {
			return "", &errtype.RenderError{
				Code:    errtype.CodeRenderSurgeProxyInvalid,
				Format:  "surge",
				Message: fmt.Sprintf("代理 %q 渲染失败：%v", px.Name, err),
				Cause:   err,
			}
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

func renderSurgeProxy(px model.TargetProxy) (string, error) {
	if px.Native != nil {
		if px.Native.Format != model.NativeProxyFormatSurge {
			return "", fmt.Errorf("原生声明格式为 %q，不能写入 Surge", px.Native.Format)
		}
		return px.Name + " = " + px.Native.Payload, nil
	}

	var parts []string

	switch px.Type {
	case "ss":
		parts = append(parts, px.Name+" = ss", px.Server, fmt.Sprintf("%d", px.Port))
		if v := px.Params["cipher"]; v != "" {
			parts = append(parts, renderINIKeyValue("encrypt-method", v))
		}
		if v := px.Params["password"]; v != "" {
			parts = append(parts, renderINIKeyValue("password", v))
		}
		pluginParams, err := renderSimpleObfsPlugin(px.Plugin, "obfs")
		if err != nil {
			return "", err
		}
		parts = append(parts, pluginParams...)
		for _, k := range surgeSSKeyOrder {
			if v := px.Params[k]; v != "" {
				parts = append(parts, renderINIKeyValue(k, v))
			}
		}
	case "socks5", "http":
		parts = append(parts, px.Name+" = "+px.Type, px.Server, fmt.Sprintf("%d", px.Port))
		username := px.Params["username"]
		password := px.Params["password"]
		if username != "" || password != "" {
			parts = append(parts, username, password)
		}
	case "snell":
		parts = append(parts, px.Name+" = snell", px.Server, fmt.Sprintf("%d", px.Port))
		for _, k := range surgeSnellKeyOrder {
			if v := px.Params[k]; v != "" {
				parts = append(parts, renderINIKeyValue(k, v))
			}
		}
	case "anytls":
		parts = append(parts, px.Name+" = anytls", px.Server, fmt.Sprintf("%d", px.Port))
		if v := px.Params["password"]; v != "" {
			parts = append(parts, renderINIKeyValue("password", v))
		}
		for _, k := range surgeAnyTLSKeyOrder {
			if v := px.Params[k]; v != "" {
				parts = append(parts, renderINIKeyValue(k, v))
			}
		}
	default:
		parts = append(parts, px.Name+" = "+px.Type, px.Server, fmt.Sprintf("%d", px.Port))
	}

	if px.Dialer != "" {
		parts = append(parts, renderINIKeyValue("underlying-proxy", px.Dialer))
	}

	return strings.Join(parts, ", "), nil
}

func buildSurgeRules(rulesets []model.Ruleset, rules []model.Rule, fallback string) string {
	var sb strings.Builder

	// 1. RULE-SET 条目。
	for _, rs := range rulesets {
		for _, u := range rs.URLs {
			fmt.Fprintf(&sb, "RULE-SET,%s,%s\n", u, rs.Policy)
		}
	}

	// 2. 内联规则。
	for _, r := range rules {
		sb.WriteString(r.Raw)
		sb.WriteByte('\n')
	}

	// 3. fallback。
	fmt.Fprintf(&sb, "FINAL,%s\n", fallback)

	return sb.String()
}

// mergeSurgeTemplate 用生成的内容替换模板中的 [Proxy]、[Proxy Group] 和
// [Rule] 段落，其他段落全部保留。
func mergeSurgeTemplate(template []byte, proxySection, groupSection, ruleSection string) string {
	return mergeINISections(template, map[string]string{
		"[Proxy]":       proxySection,
		"[Proxy Group]": groupSection,
		"[Rule]":        ruleSection,
	}, []string{"[Proxy]", "[Proxy Group]", "[Rule]"})
}
