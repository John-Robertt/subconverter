package render

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/John-Robertt/subconverter/internal/model"
)

// sectionHeaderRe matches INI-style section headers like [Proxy].
var sectionHeaderRe = regexp.MustCompile(`^\[.+\]\s*$`)

func renderINIKeyValue(key, value string) string {
	return key + "=" + quoteINIValueIfNeeded(value)
}

func quoteINIValueIfNeeded(value string) string {
	if !iniValueNeedsQuote(value) {
		return value
	}
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func iniValueNeedsQuote(value string) bool {
	return strings.TrimSpace(value) != value || strings.ContainsAny(value, `,"\`)
}

func renderSimpleObfsPlugin(plugin *model.Plugin, modeKey string) ([]string, error) {
	if plugin == nil {
		return nil, nil
	}

	if !isSimpleObfsPlugin(plugin.Name) {
		return nil, fmt.Errorf("不支持的 ss plugin %q", plugin.Name)
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
			return nil, fmt.Errorf("ss plugin %q 不支持选项 %q", plugin.Name, key)
		}
	}

	mode := plugin.Opts["obfs"]
	if mode != "http" && mode != "tls" {
		return nil, fmt.Errorf("ss plugin %q 要求 obfs=http 或 obfs=tls", plugin.Name)
	}

	parts := []string{modeKey + "=" + mode}
	if host := plugin.Opts["obfs-host"]; host != "" {
		parts = append(parts, renderINIKeyValue("obfs-host", host))
	}
	if uri := plugin.Opts["obfs-uri"]; uri != "" {
		parts = append(parts, renderINIKeyValue("obfs-uri", uri))
	}

	return parts, nil
}

func isSimpleObfsPlugin(name string) bool {
	switch name {
	case "simple-obfs", "obfs-local", "obfs":
		return true
	default:
		return false
	}
}

func buildINIGroups(nodeGroups, routeGroups []model.TargetGroup) string {
	var sb strings.Builder
	allGroups := append(routeGroups, nodeGroups...)
	for _, g := range allGroups {
		sb.WriteString(renderINIGroup(g))
		sb.WriteByte('\n')
	}
	return sb.String()
}

func renderINIGroup(g model.TargetGroup) string {
	parts := []string{g.Name + " = " + g.Strategy}
	parts = append(parts, g.Members...)

	if g.Strategy == "url-test" {
		parts = append(parts,
			"url="+urlTestURL,
			fmt.Sprintf("interval=%d", urlTestInterval),
			fmt.Sprintf("tolerance=%d", urlTestTolerance),
		)
	}

	return strings.Join(parts, ", ")
}

func mergeINISections(template []byte, replacements map[string]string, order []string) string {
	sections := parseINISections(string(template))

	// Strip any existing managed-config line from the preamble to avoid
	// duplicating a new Surge header or leaking it into a Loon profile.
	if len(sections) > 0 && sections[0].header == "" {
		sections[0].body = stripManagedConfigLine(sections[0].body)
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
	for _, header := range order {
		if !replaced[header] {
			sections = append(sections, iniSection{header: header, body: replacements[header]})
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

type iniSection struct {
	header string // e.g. "[Proxy]", or "" for preamble text before any header
	body   string
}

// parseINISections splits an INI profile into sections by [Header] lines.
func parseINISections(text string) []iniSection {
	lines := strings.Split(text, "\n")
	var sections []iniSection
	current := iniSection{}
	var bodyLines []string

	for _, line := range lines {
		if sectionHeaderRe.MatchString(line) {
			// Flush current section.
			current.body = strings.Join(bodyLines, "\n")
			sections = append(sections, current)
			current = iniSection{header: strings.TrimSpace(line)}
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
