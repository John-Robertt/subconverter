package pipeline

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/model"
	"gopkg.in/yaml.v3"
)

type subscriptionDocument struct {
	Format  model.NativeProxyFormat
	Proxies []model.Proxy
}

// DetectSubscriptionFormat uses the same precedence as Source before a document is copied.
// An empty result denotes the legacy text/Base64 subscription formats.
func DetectSubscriptionFormat(location string, body []byte) string {
	document, _ := parseSubscriptionDocument(location, body, "auto")
	if document == nil {
		return ""
	}
	return string(document.Format)
}

func parseSubscriptionDocument(rawURL string, body []byte, format string) (*subscriptionDocument, error) {
	selected := model.NativeProxyFormat(format)
	if format == "" || format == "auto" {
		selected = subscriptionDocumentFormatFromURL(rawURL)
	}
	switch selected {
	case "text":
		return nil, nil
	case model.NativeProxyFormatClash:
		return parseClashSubscriptionDocument(body, true)
	case model.NativeProxyFormatSurge, model.NativeProxyFormatLoon:
		return parseINIProxySubscription(body, true, selected)
	case "":
	default:
		return &subscriptionDocument{Format: selected}, fmt.Errorf("不支持的订阅格式 %q", format)
	}

	if document, err := parseClashSubscriptionDocument(body, false); document != nil {
		return document, err
	}
	for _, rawLine := range strings.Split(string(body), "\n") {
		switch strings.ToLower(strings.TrimSpace(rawLine)) {
		case "[remote proxy]", "[remote filter]", "[remote rule]", "[proxy chain]":
			return parseINIProxySubscription(body, true, model.NativeProxyFormatLoon)
		}
	}
	return parseINIProxySubscription(body, false, model.NativeProxyFormatSurge)
}

func subscriptionDocumentFormatFromURL(rawURL string) model.NativeProxyFormat {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".yaml", ".yml":
		return model.NativeProxyFormatClash
	case ".lcf":
		return model.NativeProxyFormatLoon
	case ".conf":
		return model.NativeProxyFormatSurge
	default:
		return ""
	}
}

func parseClashSubscriptionDocument(body []byte, required bool) (*subscriptionDocument, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(body, &root); err != nil {
		if required || looksLikeClashSubscriptionDocument(body) {
			return &subscriptionDocument{Format: model.NativeProxyFormatClash}, fmt.Errorf("解析 YAML 失败：%w", err)
		}
		return nil, nil
	}

	mapping := yamlDocumentMapping(&root)
	proxiesNode, found := yamlMappingValue(mapping, "proxies")
	if !found {
		if required {
			return &subscriptionDocument{Format: model.NativeProxyFormatClash}, fmt.Errorf("缺少顶层 proxies 字段")
		}
		return nil, nil
	}

	document := &subscriptionDocument{Format: model.NativeProxyFormatClash}
	proxiesNode = dereferenceYAMLAlias(proxiesNode)
	if proxiesNode == nil || proxiesNode.Kind != yaml.SequenceNode {
		return document, fmt.Errorf("顶层 proxies 字段必须是序列")
	}

	document.Proxies = make([]model.Proxy, 0, len(proxiesNode.Content))
	for index, proxyNode := range proxiesNode.Content {
		proxy, err := parseClashNativeProxy(proxyNode)
		if err != nil {
			return document, fmt.Errorf("proxies[%d] 无效：%w", index, err)
		}
		document.Proxies = append(document.Proxies, proxy)
	}
	return document, nil
}

func looksLikeClashSubscriptionDocument(body []byte) bool {
	for _, rawLine := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "proxies:") {
			return true
		}
	}
	return false
}

func parseClashNativeProxy(node *yaml.Node) (model.Proxy, error) {
	node = dereferenceYAMLAlias(node)
	if node == nil || node.Kind != yaml.MappingNode {
		return model.Proxy{}, fmt.Errorf("节点必须是 YAML 映射")
	}

	name, ok := yamlMappingScalar(node, "name")
	if !ok || strings.TrimSpace(name) == "" {
		return model.Proxy{}, fmt.Errorf("name 必须是非空标量")
	}
	proxyType, ok := yamlMappingScalar(node, "type")
	if !ok || strings.TrimSpace(proxyType) == "" {
		return model.Proxy{}, fmt.Errorf("type 必须是非空标量")
	}

	payload, err := encodeYAMLNode(node)
	if err != nil {
		return model.Proxy{}, fmt.Errorf("复制节点映射失败：%w", err)
	}

	server, _ := yamlMappingScalar(node, "server")
	port := yamlMappingInt(node, "port")
	return model.Proxy{
		Name:   strings.TrimSpace(name),
		Type:   strings.TrimSpace(proxyType),
		Server: strings.TrimSpace(server),
		Port:   port,
		Kind:   model.KindSubscription,
		Native: &model.NativeProxy{
			Format:  model.NativeProxyFormatClash,
			Payload: payload,
		},
	}, nil
}

func yamlDocumentMapping(root *yaml.Node) *yaml.Node {
	if root == nil || root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil
	}
	mapping := dereferenceYAMLAlias(root.Content[0])
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	return mapping
}

func yamlMappingValue(mapping *yaml.Node, key string) (*yaml.Node, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1], true
		}
	}
	return nil, false
}

func yamlMappingScalar(mapping *yaml.Node, key string) (string, bool) {
	value, found := yamlMappingValue(mapping, key)
	value = dereferenceYAMLAlias(value)
	if !found || value == nil || value.Kind != yaml.ScalarNode {
		return "", false
	}
	return value.Value, true
}

func yamlMappingInt(mapping *yaml.Node, key string) int {
	value, found := yamlMappingScalar(mapping, key)
	if !found {
		return 0
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}

func dereferenceYAMLAlias(node *yaml.Node) *yaml.Node {
	for node != nil && node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	return node
}

func parseINIProxySubscription(body []byte, required bool, format model.NativeProxyFormat) (*subscriptionDocument, error) {
	document := &subscriptionDocument{Format: format}
	inProxySection := false
	foundProxySection := false

	for lineIndex, rawLine := range strings.Split(string(body), "\n") {
		line := strings.TrimPrefix(strings.TrimSpace(strings.TrimSuffix(rawLine, "\r")), "\ufeff")
		if isINISectionHeader(line) {
			if strings.EqualFold(line, "[Proxy]") {
				inProxySection = true
				foundProxySection = true
				continue
			}
			if inProxySection {
				break
			}
			continue
		}
		if !inProxySection || isSubscriptionDocumentComment(line) {
			continue
		}

		proxy, err := parseININativeProxy(line, format)
		if err != nil {
			return document, fmt.Errorf("[Proxy] 第 %d 行无效：%w", lineIndex+1, err)
		}
		if config.IsReservedPolicyName(proxy.Name) {
			continue
		}
		document.Proxies = append(document.Proxies, proxy)
	}

	if !foundProxySection {
		if required {
			return document, fmt.Errorf("缺少 [Proxy] 段")
		}
		return nil, nil
	}
	return document, nil
}

func isINISectionHeader(line string) bool {
	return len(line) >= 3 && strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")
}

func isSubscriptionDocumentComment(line string) bool {
	return line == "" ||
		strings.HasPrefix(line, "#") ||
		strings.HasPrefix(line, ";") ||
		strings.HasPrefix(line, "//")
}

func parseININativeProxy(line string, format model.NativeProxyFormat) (model.Proxy, error) {
	name, payload, found := strings.Cut(line, "=")
	name = strings.TrimSpace(name)
	payload = strings.TrimSpace(payload)
	if !found || name == "" {
		return model.Proxy{}, fmt.Errorf("声明必须使用非空 NAME = VALUE 格式")
	}
	if payload == "" {
		return model.Proxy{}, fmt.Errorf("代理 %q 的声明内容为空", name)
	}

	proxyType, server, port := iniNativeProxyIdentity(payload)
	if proxyType == "" {
		return model.Proxy{}, fmt.Errorf("代理 %q 缺少类型", name)
	}
	return model.Proxy{
		Name:   name,
		Type:   proxyType,
		Server: server,
		Port:   port,
		Kind:   model.KindSubscription,
		Native: &model.NativeProxy{
			Format:  format,
			Payload: payload,
		},
	}, nil
}

func iniNativeProxyIdentity(payload string) (proxyType, server string, port int) {
	parts := strings.SplitN(payload, ",", 4)
	proxyType = trimININativeAtom(parts[0])
	if len(parts) > 1 {
		server = trimININativeAtom(parts[1])
	}
	if len(parts) > 2 {
		parsed, err := strconv.Atoi(trimININativeAtom(parts[2]))
		if err == nil {
			port = parsed
		}
	}
	return proxyType, server, port
}

func trimININativeAtom(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		unquoted, err := strconv.Unquote(value)
		if err == nil {
			return unquoted
		}
	}
	return value
}

func encodeYAMLNode(node *yaml.Node) (string, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(node); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}
