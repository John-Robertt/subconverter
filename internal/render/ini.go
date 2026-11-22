package render

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/John-Robertt/subconverter/internal/model"
)

// sectionHeaderRe 匹配 INI 风格的段落标题，例如 [Proxy]。
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

	// 从前言中去掉已有的 managed-config 行，避免与新的 Surge 头部重复，
	// 或被带进 Loon 配置。
	if len(sections) > 0 && sections[0].header == "" {
		sections[0].body = stripManagedConfigLine(sections[0].body)
	}

	// 替换已有的段落。
	replaced := make(map[string]bool)
	for i, sec := range sections {
		if content, ok := replacements[sec.header]; ok {
			sections[i].body = content
			replaced[sec.header] = true
		}
	}

	// 追加模板中没有的段落。
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
	header string // 例如 "[Proxy]"；第一个标题之前的前言文本为 ""
	body   string
}

// parseINISections 按 [Header] 行把 INI 配置拆分成段落。
func parseINISections(text string) []iniSection {
	lines := strings.Split(text, "\n")
	var sections []iniSection
	current := iniSection{}
	var bodyLines []string

	for _, line := range lines {
		if sectionHeaderRe.MatchString(line) {
			// 收尾当前段落。
			current.body = strings.Join(bodyLines, "\n")
			sections = append(sections, current)
			current = iniSection{header: strings.TrimSpace(line)}
			bodyLines = nil
		} else {
			bodyLines = append(bodyLines, line)
		}
	}
	// 收尾最后一个段落。
	current.body = strings.Join(bodyLines, "\n")
	sections = append(sections, current)

	// 标题和正文都为空时，去掉开头的空前言。
	if len(sections) > 0 && sections[0].header == "" && strings.TrimSpace(sections[0].body) == "" {
		sections = sections[1:]
	}

	return sections
}

// stripManagedConfigLine 从前言中去掉所有 #!MANAGED-CONFIG 行，
// 让 mergeSurgeTemplate 不会产生重复的头部。
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
