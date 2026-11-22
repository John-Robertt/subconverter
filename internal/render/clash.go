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

// providerEntry 保存 Clash Meta 输出中的一条 rule-provider 定义。
type providerEntry struct {
	name   string
	policy string
	url    string
}

// Clash 把已按目标客户端投影的流水线结果渲染为 Clash Meta YAML。
// baseTemplate 非 nil 时，生成的段落注入模板；否则只输出生成的段落。
func Clash(p *model.TargetPipeline, baseTemplate []byte) ([]byte, error) {
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

// clashSections 保存生成的各个 yaml.Node 段落。
type clashSections struct {
	proxies     *yaml.Node
	proxyGroups *yaml.Node
}

func buildClashSections(p *model.TargetPipeline) (*clashSections, []providerEntry, error) {
	proxies := &yaml.Node{Kind: yaml.SequenceNode}
	for _, px := range p.Proxies {
		node, err := buildClashProxy(px)
		if err != nil {
			return nil, nil, &errtype.RenderError{
				Code:    errtype.CodeRenderClashProxyInvalid,
				Format:  "clash",
				Message: fmt.Sprintf("代理 %q 渲染失败：%v", px.Name, err),
				Cause:   err,
			}
		}
		proxies.Content = append(proxies.Content, node)
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

func buildClashProxy(px model.TargetProxy) (*yaml.Node, error) {
	if px.Native != nil {
		return buildNativeClashProxy(px)
	}

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
		if paramBoolTrue(px.Params["udp-relay"]) {
			addPair(m, "udp", trueNode())
		}
		if paramBoolTrue(px.Params["tfo"]) {
			addPair(m, "tfo", trueNode())
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
	case "anytls":
		emitClashAnyTLSFields(m, px)
	case "vless":
		emitClashVLessFields(m, px)
	}

	if px.Dialer != "" {
		addPair(m, "dialer-proxy", scalarNode(px.Dialer))
	}

	return m, nil
}

func buildNativeClashProxy(px model.TargetProxy) (*yaml.Node, error) {
	if px.Native.Format != model.NativeProxyFormatClash {
		return nil, fmt.Errorf("原生声明格式为 %q，不能写入 Clash", px.Native.Format)
	}

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(px.Native.Payload), &root); err != nil {
		return nil, fmt.Errorf("解析原生 Clash 节点失败：%w", err)
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("原生 Clash 节点必须是 YAML 映射")
	}

	mapping := root.Content[0]
	setMappingKey(mapping, "name", scalarNode(px.Name))
	return mapping, nil
}

func paramBoolTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

// emitClashAnyTLSFields 按固定顺序写入 AnyTLS 专有字段。
func emitClashAnyTLSFields(m *yaml.Node, px model.TargetProxy) {
	if v := px.Params["password"]; v != "" {
		addPair(m, "password", scalarNode(v))
	}
	// AnyTLS 支持 UDP；默认写出，与订阅客户端的行为保持一致。
	addPair(m, "udp", trueNode())
	if v := px.Params["sni"]; v != "" {
		addPair(m, "sni", scalarNode(v))
	}
	if paramBoolTrue(px.Params["skip-cert-verify"]) {
		addPair(m, "skip-cert-verify", trueNode())
	}
	if v := px.Params["client-fingerprint"]; v != "" {
		addPair(m, "client-fingerprint", scalarNode(v))
	}
	if v := px.Params["alpn"]; v != "" {
		if parts := splitClashALPN(v); len(parts) > 0 {
			addPair(m, "alpn", sequenceOfStrings(parts))
		}
	}
	if v := px.Params["idle-session-check-interval"]; v != "" {
		addPair(m, "idle-session-check-interval", scalarNode(v))
	}
	if v := px.Params["idle-session-timeout"]; v != "" {
		addPair(m, "idle-session-timeout", scalarNode(v))
	}
	if v := px.Params["min-idle-session"]; v != "" {
		addPair(m, "min-idle-session", scalarNode(v))
	}
}

// emitClashVLessFields 按固定顺序把 VLESS 专有字段写入节点映射，
// 让 golden 文件和子串断言保持稳定。
//
// 顺序：uuid → network → udp → flow → alpn → encryption → tls（适用时）
// → servername → client-fingerprint → reality-opts → 传输层专属的 *-opts。
//
// 按 security 分支：
//   - security=none：不写 tls、servername 和 reality-opts。
//   - security=tls：tls: true + servername + client-fingerprint。
//   - security=reality：tls: true + servername + client-fingerprint + reality-opts 块。
//
// 按 network 分支写出对应的传输层 opts 块（ws-opts / http-opts / h2-opts /
// grpc-opts / xhttp-opts）。URI 中缺失或未知的 `type` 值在渲染前已由解析器
// 归一化为 tcp。
func emitClashVLessFields(m *yaml.Node, px model.TargetProxy) {
	if v := px.Params["uuid"]; v != "" {
		addPair(m, "uuid", scalarNode(v))
	}
	if v := px.Params["network"]; v != "" {
		addPair(m, "network", scalarNode(v))
	}
	// VLESS 原生支持 UDP；无条件写出。
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
		// short-id 总是写出（可以为空）：Reality 服务端可能没有配置 short-id
		// 列表，这时客户端发送 ""。
		addPair(realityOpts, "short-id", scalarNode(px.Params["reality-short-id"]))
		addPair(m, "reality-opts", realityOpts)
	}

	emitClashVLessTransportOpts(m, px)
}

// emitClashVLessTransportOpts 在 network 需要时为 VLESS 写出传输层专属的
// opts 块。tcp 没有 opts 块；其余五种显式处理的 network（ws/http/h2/grpc/xhttp）
// 各有专门的构建函数。
//
// 没有任何子字段的 opts 块整个省略：Clash 把不带 `ws-opts` 的 "network: ws"
// 当作默认的空 ws-opts，结果相同。
func emitClashVLessTransportOpts(m *yaml.Node, px model.TargetProxy) {
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
func buildClashVLessWsOpts(px model.TargetProxy) *yaml.Node {
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
// URI 只能携带一个 path 和一个 host；Clash 要求写成列表。
func buildClashVLessHttpOpts(px model.TargetProxy) *yaml.Node {
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

// buildClashVLessH2Opts → h2-opts: { host:[...], path }。host 是列表。
func buildClashVLessH2Opts(px model.TargetProxy) *yaml.Node {
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
func buildClashVLessGrpcOpts(px model.TargetProxy) *yaml.Node {
	opts := &yaml.Node{Kind: yaml.MappingNode}
	if v := px.Params["grpc-service-name"]; v != "" {
		addPair(opts, "grpc-service-name", scalarNode(v))
	}
	return opts
}

// buildClashVLessXhttpOpts → xhttp-opts: { mode, path, host }.
func buildClashVLessXhttpOpts(px model.TargetProxy) *yaml.Node {
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

// splitClashALPN 把逗号分隔的 ALPN 字符串拆成去掉首尾空白的非空条目，
// 用于写成 YAML 列表。
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

// trueNode 返回值为布尔 `true` 的 YAML 标量节点。VLESS 无条件写出
// `udp: true` 和 `tls: true`；没有调用方需要 `false`，所以这个辅助函数
// 只做这一件事，避免让人误以为它是通用的布尔抽象。
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

func buildClashGroup(g model.TargetGroup) *yaml.Node {
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

	// 1. 来自 provider 的 RULE-SET 条目。
	for _, pe := range providers {
		seq.Content = append(seq.Content, scalarNode(
			fmt.Sprintf("RULE-SET,%s,%s", pe.name, pe.policy),
		))
	}

	// 2. 内联规则。
	for _, r := range rules {
		seq.Content = append(seq.Content, scalarNode(r.Raw))
	}

	// 3. fallback。
	seq.Content = append(seq.Content, scalarNode(
		fmt.Sprintf("MATCH,%s", fallback),
	))

	return seq
}

// --- 提取 provider 名称 ---

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
	// 第 1 步：收集所有基础名称。
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

	// 第 2 步：统计每个基础名称出现的次数。
	count := make(map[string]int)
	for _, e := range entries {
		count[e.base]++
	}

	// 第 3 步：分配名称。不重复的基础名称直接使用。
	// 重复的基础名称加上递增后缀，并跳过已被占用的名称
	// （避免与本来就叫这个名字的条目冲突）。
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

// --- yaml.Node 辅助函数 ---

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

// setMappingKey 替换 MappingNode 中已有键的值；键不存在时追加这个键值对。
func setMappingKey(mapping *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}
	addPair(mapping, key, value)
}
