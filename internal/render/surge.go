package render

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

const (
	surgeURLTestTolerance = 100
	surgeManagedInterval  = 86400
)

// sectionHeaderRe matches INI-style section headers like [Proxy].
var sectionHeaderRe = regexp.MustCompile(`^\[.+\]\s*$`)

// Surge renders a Pipeline into Surge conf format.
// managedURL is used for the #!MANAGED-CONFIG header (empty = omit header).
// If baseTemplate is non-nil, generated sections replace corresponding sections in the template.
func Surge(p *model.Pipeline, managedURL string, baseTemplate []byte) ([]byte, error) {
	proxySection, err := buildSurgeProxies(p.Proxies)
	if err != nil {
		return nil, err
	}
	groupSection := buildSurgeGroups(p.NodeGroups, p.RouteGroups)
	ruleSection := buildSurgeRules(p.Rulesets, p.Rules, p.Fallback)

	var buf bytes.Buffer

	// Managed config header.
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

func buildSurgeProxies(proxies []model.Proxy) (string, error) {
	var sb strings.Builder
	for _, px := range proxies {
		line, err := renderSurgeProxy(px)
		if err != nil {
			return "", &errtype.RenderError{
				Format:  "surge",
				Message: fmt.Sprintf("proxy %q: %v", px.Name, err),
				Cause:   err,
			}
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

func renderSurgeProxy(px model.Proxy) (string, error) {
	var parts []string

	switch px.Type {
	case "ss":
		parts = append(parts, px.Name+" = ss", px.Server, fmt.Sprintf("%d", px.Port))
		if v := px.Params["cipher"]; v != "" {
			parts = append(parts, "encrypt-method="+v)
		}
		if v := px.Params["password"]; v != "" {
			parts = append(parts, "password="+v)
		}
		pluginParams, err := renderSurgeSSPlugin(px.Plugin)
		if err != nil {
			return "", err
		}
		parts = append(parts, pluginParams...)
	case "socks5", "http":
		parts = append(parts, px.Name+" = "+px.Type, px.Server, fmt.Sprintf("%d", px.Port))
		username := px.Params["username"]
		password := px.Params["password"]
		if username != "" || password != "" {
			parts = append(parts, username, password)
		}
	default:
		parts = append(parts, px.Name+" = "+px.Type, px.Server, fmt.Sprintf("%d", px.Port))
	}

	if px.Dialer != "" {
		parts = append(parts, "underlying-proxy="+px.Dialer)
	}

	return strings.Join(parts, ", "), nil
}

func renderSurgeSSPlugin(plugin *model.Plugin) ([]string, error) {
	if plugin == nil {
		return nil, nil
	}

	if !isSurgeSSObfsPlugin(plugin.Name) {
		return nil, fmt.Errorf("unsupported ss plugin %q", plugin.Name)
	}

	allowed := map[string]bool{
		"obfs":      true,
		"obfs-host": true,
		"obfs-uri":  true,
	}
	keys := make([]string, 0, len(plugin.Opts))
	for key := range plugin.Opts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !allowed[key] {
			return nil, fmt.Errorf("unsupported ss plugin option %q for %q", key, plugin.Name)
		}
	}

	mode := plugin.Opts["obfs"]
	if mode != "http" && mode != "tls" {
		return nil, fmt.Errorf("ss plugin %q requires obfs=http or obfs=tls", plugin.Name)
	}

	parts := []string{"obfs=" + mode}
	if host := plugin.Opts["obfs-host"]; host != "" {
		parts = append(parts, "obfs-host="+host)
	}
	if uri := plugin.Opts["obfs-uri"]; uri != "" {
		parts = append(parts, "obfs-uri="+uri)
	}

	return parts, nil
}

func isSurgeSSObfsPlugin(name string) bool {
	switch name {
	case "simple-obfs", "obfs-local", "obfs":
		return true
	default:
		return false
	}
}

func buildSurgeGroups(nodeGroups, routeGroups []model.ProxyGroup) string {
	var sb strings.Builder
	allGroups := append(routeGroups, nodeGroups...)
	for _, g := range allGroups {
		sb.WriteString(renderSurgeGroup(g))
		sb.WriteByte('\n')
	}
	return sb.String()
}

func renderSurgeGroup(g model.ProxyGroup) string {
	parts := []string{g.Name + " = " + g.Strategy}
	parts = append(parts, g.Members...)

	if g.Strategy == "url-test" {
		parts = append(parts,
			"url="+urlTestURL,
			fmt.Sprintf("interval=%d", urlTestInterval),
			fmt.Sprintf("tolerance=%d", surgeURLTestTolerance),
		)
	}

	return strings.Join(parts, ", ")
}

func buildSurgeRules(rulesets []model.Ruleset, rules []model.Rule, fallback string) string {
	var sb strings.Builder

	// 1. RULE-SET entries.
	for _, rs := range rulesets {
		for _, u := range rs.URLs {
			fmt.Fprintf(&sb, "RULE-SET,%s,%s\n", u, rs.Policy)
		}
	}

	// 2. Inline rules.
	for _, r := range rules {
		sb.WriteString(r.Raw)
		sb.WriteByte('\n')
	}

	// 3. Fallback.
	fmt.Fprintf(&sb, "FINAL,%s\n", fallback)

	return sb.String()
}

// mergeSurgeTemplate replaces [Proxy], [Proxy Group], and [Rule] sections
// in the template with generated content, preserving all other sections.
func mergeSurgeTemplate(template []byte, proxySection, groupSection, ruleSection string) string {
	sections := parseSurgeSections(string(template))

	// Strip any existing managed-config line from the preamble to avoid
	// duplicating the header that Surge() generates.
	if len(sections) > 0 && sections[0].header == "" {
		sections[0].body = stripManagedConfigLine(sections[0].body)
	}

	replacements := map[string]string{
		"[Proxy]":       proxySection,
		"[Proxy Group]": groupSection,
		"[Rule]":        ruleSection,
	}

	// Replace existing sections.
	replaced := make(map[string]bool)
	for i, sec := range sections {
		if content, ok := replacements[sec.header]; ok {
			sections[i].body = content
			replaced[sec.header] = true
		}
	}

	// Append sections that were not found in the template.
	order := []string{"[Proxy]", "[Proxy Group]", "[Rule]"}
	for _, header := range order {
		if !replaced[header] {
			sections = append(sections, surgeSection{header: header, body: replacements[header]})
		}
	}

	var sb strings.Builder
	for i, sec := range sections {
		if sec.header != "" {
			if i > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(sec.header)
			sb.WriteByte('\n')
		}
		sb.WriteString(sec.body)
	}

	return sb.String()
}

type surgeSection struct {
	header string // e.g. "[Proxy]", or "" for preamble text before any header
	body   string
}

// parseSurgeSections splits a Surge config into sections by [Header] lines.
func parseSurgeSections(text string) []surgeSection {
	lines := strings.Split(text, "\n")
	var sections []surgeSection
	current := surgeSection{}
	var bodyLines []string

	for _, line := range lines {
		if sectionHeaderRe.MatchString(line) {
			// Flush current section.
			current.body = strings.Join(bodyLines, "\n")
			sections = append(sections, current)
			current = surgeSection{header: strings.TrimSpace(line)}
			bodyLines = nil
		} else {
			bodyLines = append(bodyLines, line)
		}
	}
	// Flush last section.
	current.body = strings.Join(bodyLines, "\n")
	sections = append(sections, current)

	// Remove leading empty preamble if header and body are both empty.
	if len(sections) > 0 && sections[0].header == "" && strings.TrimSpace(sections[0].body) == "" {
		sections = sections[1:]
	}

	return sections
}

// stripManagedConfigLine removes any #!MANAGED-CONFIG line from the preamble
// so that mergeSurgeTemplate does not produce a duplicate header.
func stripManagedConfigLine(body string) string {
	lines := strings.Split(body, "\n")
	filtered := lines[:0]
	for _, line := range lines {
		if !strings.HasPrefix(line, "#!MANAGED-CONFIG") {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}
