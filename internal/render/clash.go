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
	}

	if px.Dialer != "" {
		addPair(m, "dialer-proxy", scalarNode(px.Dialer))
	}

	return m
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
