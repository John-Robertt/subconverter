package render

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
	"gopkg.in/yaml.v3"
)

const (
	urlTestURL           = "http://www.gstatic.com/generate_204"
	urlTestInterval      = 300
	urlTestTolerance     = 100
	ruleProviderInterval = 86400
	ruleProviderBehavior = "classical"
	ruleProviderFormat   = "text"
	ruleProviderPathExt  = ".txt"
)

// providerEntry holds a rule-provider definition for Clash Meta output.
type providerEntry struct {
	name   string
	policy string
	url    string
}

// Clash renders a Pipeline into Clash Meta YAML.
// If baseTemplate is non-nil, generated sections are injected into the template;
// otherwise only generated sections are output.
//
// Snell nodes are filtered out of the Clash view (see filterForClash): they
// only appear in Surge output because Clash Meta mainline does not support
// Snell v4/v5. Groups, rulesets, and rules that reference only filtered
// nodes are cascaded out automatically.
func Clash(p *model.Pipeline, baseTemplate []byte) ([]byte, error) {
	p, err := filterForClash(p)
	if err != nil {
		return nil, err
	}

	generated, providers, err := buildClashSections(p)
	if err != nil {
		return nil, err
	}

	var root yaml.Node
	if baseTemplate != nil {
		if err := yaml.Unmarshal(baseTemplate, &root); err != nil {
			return nil, &errtype.RenderError{
				Code:    errtype.CodeRenderTemplateParseFailed,
				Format:  "clash",
				Message: "解析底版模板失败：" + err.Error(),
				Cause:   err,
			}
		}
		if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
			return nil, &errtype.RenderError{
				Code:    errtype.CodeRenderTemplateInvalid,
				Format:  "clash",
				Message: "底版模板必须是 YAML 映射文档",
			}
		}
	} else {
		root = yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{
				{Kind: yaml.MappingNode},
			},
		}
	}

	mapping := root.Content[0]

	setMappingKey(mapping, "proxies", generated.proxies)
	setMappingKey(mapping, "proxy-groups", generated.proxyGroups)
	setMappingKey(mapping, "rule-providers", buildRuleProviderNode(providers))
	setMappingKey(mapping, "rules", buildClashRulesNode(providers, p.Rules, p.Fallback))

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		return nil, &errtype.RenderError{
			Code:    errtype.CodeRenderYAMLEncodeFailed,
			Format:  "clash",
			Message: "编码 YAML 失败：" + err.Error(),
			Cause:   err,
		}
	}
	if err := enc.Close(); err != nil {
		return nil, &errtype.RenderError{
			Code:    errtype.CodeRenderYAMLFinalizeFailed,
			Format:  "clash",
			Message: "完成 YAML 编码失败：" + err.Error(),
			Cause:   err,
		}
	}

	return buf.Bytes(), nil
}

// clashSections holds the generated yaml.Node sections.
type clashSections struct {
	proxies     *yaml.Node
	proxyGroups *yaml.Node
}

func buildClashSections(p *model.Pipeline) (*clashSections, []providerEntry, error) {
	proxies := &yaml.Node{Kind: yaml.SequenceNode}
	for _, px := range p.Proxies {
		proxies.Content = append(proxies.Content, buildClashProxy(px))
	}

	groups := &yaml.Node{Kind: yaml.SequenceNode}
	allGroups := append(p.RouteGroups, p.NodeGroups...)
	for _, g := range allGroups {
		groups.Content = append(groups.Content, buildClashGroup(g))
	}

	providers := assignProviderNames(p.Rulesets)

	return &clashSections{
		proxies:     proxies,
		proxyGroups: groups,
	}, providers, nil
}

func buildClashProxy(px model.Proxy) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}
	addPair(m, "name", scalarNode(px.Name))
	addPair(m, "type", scalarNode(px.Type))
	addPair(m, "server", scalarNode(px.Server))
	addPair(m, "port", intNode(px.Port))

	switch px.Type {
	case "ss":
		if v := px.Params["cipher"]; v != "" {
			addPair(m, "cipher", scalarNode(v))
		}
		if v := px.Params["password"]; v != "" {
			addPair(m, "password", scalarNode(v))
		}
		if px.Plugin != nil {
			addPair(m, "plugin", scalarNode(normalizeClashSSPluginName(px.Plugin.Name)))
			if pluginOpts := buildClashPluginOpts(px.Plugin); len(pluginOpts.Content) > 0 {
				addPair(m, "plugin-opts", pluginOpts)
			}
		}
	case "socks5", "http":
		if v := px.Params["username"]; v != "" {
			addPair(m, "username", scalarNode(v))
		}
		if v := px.Params["password"]; v != "" {
			addPair(m, "password", scalarNode(v))
		}
	case "vless":
		emitClashVLessFields(m, px)
	}

	if px.Dialer != "" {
		addPair(m, "dialer-proxy", scalarNode(px.Dialer))
	}

	return m
}

// emitClashVLessFields writes VLESS-specific fields onto the proxy mapping
// node in a fixed order so golden files and substring assertions stay stable.
//
// Order: uuid → network → udp → flow → alpn → encryption → tls (when
// applicable) → servername → client-fingerprint → reality-opts →
// transport-specific *-opts.
//
// Security branches:
//   - security=none: no tls block, no servername, no reality-opts.
//   - security=tls:  tls: true + servername + client-fingerprint.
//   - security=reality: tls: true + servername + client-fingerprint + reality-opts block.
//
// Network branches emit the matching transport opts block (ws-opts /
// http-opts / h2-opts / grpc-opts / xhttp-opts). Missing or unknown URI
// `type` values are normalized to tcp by the parser before render.
func emitClashVLessFields(m *yaml.Node, px model.Proxy) {
	if v := px.Params["uuid"]; v != "" {
		addPair(m, "uuid", scalarNode(v))
	}
	if v := px.Params["network"]; v != "" {
		addPair(m, "network", scalarNode(v))
	}
	// VLESS supports UDP natively; emit unconditionally.
	addPair(m, "udp", trueNode())

	if v := px.Params["flow"]; v != "" {
		addPair(m, "flow", scalarNode(v))
	}

	if v := px.Params["alpn"]; v != "" {
		if parts := splitClashALPN(v); len(parts) > 0 {
			addPair(m, "alpn", sequenceOfStrings(parts))
		}
	}
	if v := px.Params["encryption"]; v != "" {
		addPair(m, "encryption", scalarNode(v))
	}

	security := px.Params["security"]
	if security == "tls" || security == "reality" {
		addPair(m, "tls", trueNode())
		if v := px.Params["servername"]; v != "" {
			addPair(m, "servername", scalarNode(v))
		}
		if v := px.Params["client-fingerprint"]; v != "" {
			addPair(m, "client-fingerprint", scalarNode(v))
		}
	}

	if security == "reality" {
		realityOpts := &yaml.Node{Kind: yaml.MappingNode}
		if v := px.Params["reality-public-key"]; v != "" {
			addPair(realityOpts, "public-key", scalarNode(v))
		}
		// short-id is always emitted (may be empty) — Reality servers may
		// configure no short-id list, in which case clients send "".
		addPair(realityOpts, "short-id", scalarNode(px.Params["reality-short-id"]))
		addPair(m, "reality-opts", realityOpts)
	}

	emitClashVLessTransportOpts(m, px)
}

// emitClashVLessTransportOpts emits the transport-specific opts block for
// VLESS when the network requires one. tcp has no opts block; the remaining
// five explicitly handled networks (ws/http/h2/grpc/xhttp) each get a
// dedicated builder.
//
// An opts block with zero populated sub-fields is omitted entirely — Clash
// treats "network: ws" without `ws-opts` as default-empty ws-opts, which is
// the same outcome.
func emitClashVLessTransportOpts(m *yaml.Node, px model.Proxy) {
	switch px.Params["network"] {
	case "ws":
		if opts := buildClashVLessWsOpts(px); len(opts.Content) > 0 {
			addPair(m, "ws-opts", opts)
		}
	case "http":
		if opts := buildClashVLessHttpOpts(px); len(opts.Content) > 0 {
			addPair(m, "http-opts", opts)
		}
	case "h2":
		if opts := buildClashVLessH2Opts(px); len(opts.Content) > 0 {
			addPair(m, "h2-opts", opts)
		}
	case "grpc":
		if opts := buildClashVLessGrpcOpts(px); len(opts.Content) > 0 {
			addPair(m, "grpc-opts", opts)
		}
	case "xhttp":
		if opts := buildClashVLessXhttpOpts(px); len(opts.Content) > 0 {
			addPair(m, "xhttp-opts", opts)
		}
	}
}

// buildClashVLessWsOpts → ws-opts: { path, headers: { Host } }.
func buildClashVLessWsOpts(px model.Proxy) *yaml.Node {
	opts := &yaml.Node{Kind: yaml.MappingNode}
	if v := px.Params["ws-path"]; v != "" {
		addPair(opts, "path", scalarNode(v))
	}
	if v := px.Params["ws-host"]; v != "" {
		headers := &yaml.Node{Kind: yaml.MappingNode}
		addPair(headers, "Host", scalarNode(v))
		addPair(opts, "headers", headers)
	}
	return opts
}

// buildClashVLessHttpOpts → http-opts: { path:[...], headers:{Host:[...]} }.
// URI can only carry one path / one host; Clash requires list values.
func buildClashVLessHttpOpts(px model.Proxy) *yaml.Node {
	opts := &yaml.Node{Kind: yaml.MappingNode}
	if v := px.Params["http-path"]; v != "" {
		addPair(opts, "path", sequenceOfStrings([]string{v}))
	}
	if v := px.Params["http-host"]; v != "" {
		headers := &yaml.Node{Kind: yaml.MappingNode}
		addPair(headers, "Host", sequenceOfStrings([]string{v}))
		addPair(opts, "headers", headers)
	}
	return opts
}

// buildClashVLessH2Opts → h2-opts: { host:[...], path }. host is a list.
func buildClashVLessH2Opts(px model.Proxy) *yaml.Node {
	opts := &yaml.Node{Kind: yaml.MappingNode}
	if v := px.Params["h2-host"]; v != "" {
		addPair(opts, "host", sequenceOfStrings([]string{v}))
	}
	if v := px.Params["h2-path"]; v != "" {
		addPair(opts, "path", scalarNode(v))
	}
	return opts
}

// buildClashVLessGrpcOpts → grpc-opts: { grpc-service-name }.
func buildClashVLessGrpcOpts(px model.Proxy) *yaml.Node {
	opts := &yaml.Node{Kind: yaml.MappingNode}
	if v := px.Params["grpc-service-name"]; v != "" {
		addPair(opts, "grpc-service-name", scalarNode(v))
	}
	return opts
}

// buildClashVLessXhttpOpts → xhttp-opts: { mode, path, host }.
func buildClashVLessXhttpOpts(px model.Proxy) *yaml.Node {
	opts := &yaml.Node{Kind: yaml.MappingNode}
	if v := px.Params["xhttp-mode"]; v != "" {
		addPair(opts, "mode", scalarNode(v))
	}
	if v := px.Params["xhttp-path"]; v != "" {
		addPair(opts, "path", scalarNode(v))
	}
	if v := px.Params["xhttp-host"]; v != "" {
		addPair(opts, "host", scalarNode(v))
	}
	return opts
}

// splitClashALPN splits a comma-separated ALPN string into trimmed, non-empty
// entries suitable for YAML list emission.
func splitClashALPN(csv string) []string {
	raw := strings.Split(csv, ",")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// trueNode returns a YAML scalar node holding the boolean `true`. VLESS
// emits `udp: true` and `tls: true` unconditionally; `false` has no caller,
// so the helper is narrowed to avoid suggesting a generic bool abstraction.
func trueNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}
}

func normalizeClashSSPluginName(name string) string {
	switch name {
	case "simple-obfs", "obfs-local", "obfs":
		return "obfs"
	default:
		return name
	}
}

func buildClashPluginOpts(plugin *model.Plugin) *yaml.Node {
	opts := &yaml.Node{Kind: yaml.MappingNode}
	if plugin == nil || len(plugin.Opts) == 0 {
		return opts
	}

	mapped := make(map[string]string, len(plugin.Opts))
	for key, value := range plugin.Opts {
		mapped[mapClashPluginOpt(plugin.Name, key)] = value
	}

	keys := make([]string, 0, len(mapped))
	for key := range mapped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		addPair(opts, key, scalarNode(mapped[key]))
	}

	return opts
}

func mapClashPluginOpt(pluginName, key string) string {
	switch normalizeClashSSPluginName(pluginName) {
	case "obfs":
		switch key {
		case "obfs":
			return "mode"
		case "obfs-host":
			return "host"
		}
	}
	return key
}

func buildClashGroup(g model.ProxyGroup) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}
	addPair(m, "name", scalarNode(g.Name))
	addPair(m, "type", scalarNode(g.Strategy))
	addPair(m, "proxies", sequenceOfStrings(g.Members))

	if g.Strategy == "url-test" {
		addPair(m, "url", scalarNode(urlTestURL))
		addPair(m, "interval", intNode(urlTestInterval))
		addPair(m, "tolerance", intNode(urlTestTolerance))
	}

	return m
}

func buildRuleProviderNode(providers []providerEntry) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}
	for _, pe := range providers {
		entry := &yaml.Node{Kind: yaml.MappingNode}
		addPair(entry, "type", scalarNode("http"))
		addPair(entry, "behavior", scalarNode(ruleProviderBehavior))
		addPair(entry, "format", scalarNode(ruleProviderFormat))
		addPair(entry, "url", scalarNode(pe.url))
		addPair(entry, "path", scalarNode("./rule-providers/"+pe.name+ruleProviderPathExt))
		addPair(entry, "interval", intNode(ruleProviderInterval))
		addPair(m, pe.name, entry)
	}
	return m
}

func buildClashRulesNode(providers []providerEntry, rules []model.Rule, fallback string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode}

	// 1. RULE-SET entries from providers.
	for _, pe := range providers {
		seq.Content = append(seq.Content, scalarNode(
			fmt.Sprintf("RULE-SET,%s,%s", pe.name, pe.policy),
		))
	}

	// 2. Inline rules.
	for _, r := range rules {
		seq.Content = append(seq.Content, scalarNode(r.Raw))
	}

	// 3. Fallback.
	seq.Content = append(seq.Content, scalarNode(
		fmt.Sprintf("MATCH,%s", fallback),
	))

	return seq
}

// --- provider name extraction ---

func extractProviderName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Path == "" {
		return "provider"
	}
	base := path.Base(u.Path)
	ext := path.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		return "provider"
	}
	return name
}

func assignProviderNames(rulesets []model.Ruleset) []providerEntry {
	// Phase 1: collect all base names.
	type rawEntry struct {
		base, policy, url string
	}
	var entries []rawEntry
	for _, rs := range rulesets {
		for _, u := range rs.URLs {
			entries = append(entries, rawEntry{
				base:   extractProviderName(u),
				policy: rs.Policy,
				url:    u,
			})
		}
	}

	// Phase 2: count occurrences of each base name.
	count := make(map[string]int)
	for _, e := range entries {
		count[e.base]++
	}

	// Phase 3: assign names. Unique base names are used as-is.
	// Duplicate base names get incrementing suffixes, skipping any
	// name already occupied (prevents collision with natural names).
	used := make(map[string]bool)
	for name, c := range count {
		if c == 1 {
			used[name] = true
		}
	}

	seenIdx := make(map[string]int)
	result := make([]providerEntry, 0, len(entries))
	for _, e := range entries {
		var name string
		if count[e.base] == 1 {
			name = e.base
		} else {
			seenIdx[e.base]++
			if seenIdx[e.base] == 1 {
				name = e.base
			} else {
				suffix := seenIdx[e.base]
				candidate := fmt.Sprintf("%s-%d", e.base, suffix)
				for used[candidate] {
					suffix++
					candidate = fmt.Sprintf("%s-%d", e.base, suffix)
				}
				name = candidate
			}
			used[name] = true
		}
		result = append(result, providerEntry{name: name, policy: e.policy, url: e.url})
	}
	return result
}

// --- yaml.Node helpers ---

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func intNode(value int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(value)}
}

func addPair(mapping *yaml.Node, key string, value *yaml.Node) {
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value,
	)
}

func sequenceOfStrings(items []string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, s := range items {
		seq.Content = append(seq.Content, scalarNode(s))
	}
	return seq
}

// setMappingKey replaces an existing key's value in a MappingNode, or appends
// the key-value pair if the key does not exist.
func setMappingKey(mapping *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}
	addPair(mapping, key, value)
}
