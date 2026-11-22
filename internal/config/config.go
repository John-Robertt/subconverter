package config

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Config 是用户配置的顶层结构。
type Config struct {
	Sources   Sources              `yaml:"sources" json:"sources"`
	Filters   Filters              `yaml:"filters" json:"filters"`
	Groups    OrderedMap[Group]    `yaml:"groups" json:"groups"`
	Routing   OrderedMap[[]string] `yaml:"routing" json:"routing"`
	Rulesets  OrderedMap[[]string] `yaml:"rulesets" json:"rulesets"`
	Rules     []string             `yaml:"rules" json:"rules"`
	Fallback  string               `yaml:"fallback" json:"fallback"`
	BaseURL   string               `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	Templates Templates            `yaml:"templates,omitempty" json:"templates,omitempty"`
}

// Templates 按输出格式声明可选的底版配置模板。
// 位置可以是本地路径或 HTTP(S) URL；Inline 存放可编辑的模板正文。
type Templates struct {
	Clash  string            `yaml:"clash,omitempty" json:"clash,omitempty"`
	Surge  string            `yaml:"surge,omitempty" json:"surge,omitempty"`
	Loon   string            `yaml:"loon,omitempty" json:"loon,omitempty"`
	Inline map[string]string `yaml:"inline,omitempty" json:"inline,omitempty"`
}

// Sources 声明订阅与自定义代理输入。
//
// FetchOrder 记录拉取类键（subscriptions / snell / vless）在 YAML 中的声明顺序，
// 让流水线的 Source 阶段按用户声明的顺序遍历它们。它由自定义的 UnmarshalYAML 填充。
// custom_proxies 不属于拉取类，不计入 FetchOrder。
type Sources struct {
	Subscriptions []Source      `yaml:"subscriptions" json:"subscriptions"`
	Snell         []Source      `yaml:"snell" json:"snell"`
	VLess         []Source      `yaml:"vless" json:"vless"`
	CustomProxies []CustomProxy `yaml:"custom_proxies" json:"custom_proxies"`

	// FetchOrder 按 YAML 声明顺序存放拉取类键。
	// 由 UnmarshalYAML 设置；Sources 为零值时（测试中）为空。
	FetchOrder []string `yaml:"-" json:"fetch_order"`
}

// Source 声明一个 URL、可编辑正文或本地正文文件。Format 仅用于 subscriptions。
type Source struct {
	Name        string `yaml:"name,omitempty" json:"name,omitempty"`
	URL         string `yaml:"url,omitempty" json:"url,omitempty"`
	Content     string `yaml:"content,omitempty" json:"content,omitempty"`
	ContentFile string `yaml:"content_file,omitempty" json:"content_file,omitempty"`
	Format      string `yaml:"format,omitempty" json:"format,omitempty"`
}

// SourceList 标识一组来源，并保持其声明顺序不变。
type SourceList struct {
	Kind    string
	Entries []Source
}

// Lists 按固定顺序给出可编辑的来源，供校验、存储和归档使用。
// 运行时遍历改用 FetchOrder。
func (s Sources) Lists() []SourceList {
	return []SourceList{{"subscriptions", s.Subscriptions}, {"snell", s.Snell}, {"vless", s.VLess}}
}

// sourceFetchKeys 列出 `sources:` 下对应拉取类输入（远程来源的节点）的 YAML 键。
// custom_proxies 不在其中：它由用户直接内联声明，不参与 FetchOrder。
var sourceFetchKeys = map[string]bool{
	"subscriptions": true,
	"snell":         true,
	"vless":         true,
}

var defaultSourceFetchOrder = []string{"subscriptions", "snell", "vless"}

// UnmarshalYAML 为 Sources 实现 yaml.Unmarshaler。
//
// 它按 YAML 声明顺序遍历映射节点，把每个已知键解码到对应的结构体字段，
// 同时把拉取类键追加到 FetchOrder。流水线的 Source 阶段用 FetchOrder
// 按用户书写的顺序遍历拉取类输入（例如 snell → vless → subscriptions），
// 这决定了最终节点切片的顺序。
//
// 参考写法：orderedmap.go 中的 OrderedMap.UnmarshalYAML。
//
// 这里包含的设计决策：
//   - 未知的顶层键直接报错（防止 "vles" 这类拼写错误）。
//   - 同一个键出现两次时报错（与 OrderedMap 的做法一致）。
//   - 空的段落（例如 `vless:` 内容为 nil 或空列表）仍会把键记入 FetchOrder：
//     即使列表为空，声明意图也有意义——Source 循环只是对该类型无可拉取，
//     不产生节点。
//   - custom_proxies 会被解码，但不记入 FetchOrder。
func (s *Sources) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("sources: 应为映射节点，实际节点类型为 %d", node.Kind)
	}

	seen := make(map[string]bool, len(node.Content)/2)

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		key := keyNode.Value

		if seen[key] {
			return fmt.Errorf("sources: 键 %q 重复（第 %d 行）", key, keyNode.Line)
		}
		seen[key] = true

		switch key {
		case "subscriptions":
			if err := valNode.Decode(&s.Subscriptions); err != nil {
				return fmt.Errorf("sources.subscriptions: %w", err)
			}
		case "snell":
			if err := valNode.Decode(&s.Snell); err != nil {
				return fmt.Errorf("sources.snell: %w", err)
			}
		case "vless":
			if err := valNode.Decode(&s.VLess); err != nil {
				return fmt.Errorf("sources.vless: %w", err)
			}
		case "custom_proxies":
			if err := valNode.Decode(&s.CustomProxies); err != nil {
				return fmt.Errorf("sources.custom_proxies: %w", err)
			}
		default:
			return fmt.Errorf("sources: 未知的键 %q（第 %d 行）", key, keyNode.Line)
		}

		if sourceFetchKeys[key] {
			s.FetchOrder = append(s.FetchOrder, key)
		}
	}

	return nil
}

// MarshalYAML 实现 yaml.Marshaler，让 sources 的键顺序保持稳定。
func (s Sources) MarshalYAML() (any, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	order := completeSourceFetchOrder(s.FetchOrder)
	for _, key := range order {
		var value any
		switch key {
		case "subscriptions":
			value = s.Subscriptions
		case "snell":
			value = s.Snell
		case "vless":
			value = s.VLess
		default:
			return nil, fmt.Errorf("sources.fetch_order: 未知的键 %q", key)
		}
		if err := appendMappingNode(node, key, value); err != nil {
			return nil, err
		}
	}
	if err := appendMappingNode(node, "custom_proxies", s.CustomProxies); err != nil {
		return nil, err
	}
	return node, nil
}

// MarshalJSON 输出带完整 fetch_order 的 sources，列表字段用 [] 代替 null，
// 让经过 API 的往返结果保持稳定。
func (s Sources) MarshalJSON() ([]byte, error) {
	type sourcesJSON struct {
		Subscriptions []Source      `json:"subscriptions"`
		Snell         []Source      `json:"snell"`
		VLess         []Source      `json:"vless"`
		CustomProxies []CustomProxy `json:"custom_proxies"`
		FetchOrder    []string      `json:"fetch_order"`
	}
	return json.Marshal(sourcesJSON{
		Subscriptions: nonNilSlice(s.Subscriptions),
		Snell:         nonNilSlice(s.Snell),
		VLess:         nonNilSlice(s.VLess),
		CustomProxies: nonNilSlice(s.CustomProxies),
		FetchOrder:    completeSourceFetchOrder(s.FetchOrder),
	})
}

// UnmarshalJSON 接受 API 使用的对象结构。fetch_order 的校验由 app 层的 API 封装完成，
// 这样调用方能拿到结构化的诊断信息。
func (s *Sources) UnmarshalJSON(data []byte) error {
	type sourcesJSON struct {
		Subscriptions []Source      `json:"subscriptions"`
		Snell         []Source      `json:"snell"`
		VLess         []Source      `json:"vless"`
		CustomProxies []CustomProxy `json:"custom_proxies"`
		FetchOrder    []string      `json:"fetch_order"`
	}
	var decoded sourcesJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	s.Subscriptions = decoded.Subscriptions
	s.Snell = decoded.Snell
	s.VLess = decoded.VLess
	s.CustomProxies = decoded.CustomProxies
	s.FetchOrder = decoded.FetchOrder
	return nil
}

func appendMappingNode(node *yaml.Node, key string, value any) error {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	var valueNode yaml.Node
	if err := valueNode.Encode(value); err != nil {
		return fmt.Errorf("sources.%s: %w", key, err)
	}
	node.Content = append(node.Content, keyNode, &valueNode)
	return nil
}

func completeSourceFetchOrder(order []string) []string {
	if len(order) == 0 {
		return append([]string(nil), defaultSourceFetchOrder...)
	}
	result := append([]string(nil), order...)
	seen := make(map[string]bool, len(order))
	for _, key := range order {
		seen[key] = true
	}
	for _, key := range defaultSourceFetchOrder {
		if !seen[key] {
			result = append(result, key)
		}
	}
	return result
}

func nonNilSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// CustomProxy 是用户通过协议 URL 声明的自定义代理节点。
// 运行时的解析结果稍后由 pipeline.Source 产出；config 层只保留原始 YAML 字段。
type CustomProxy struct {
	URL          string        `yaml:"url" json:"url"`
	Name         string        `yaml:"name" json:"name"`
	RelayThrough *RelayThrough `yaml:"relay_through,omitempty" json:"relay_through,omitempty"`
}

// RelayThrough 定义自定义代理如何经由上游节点做链式连接。
type RelayThrough struct {
	Type     string `yaml:"type" json:"type"`                       // "group" | "select" | "all"
	Strategy string `yaml:"strategy" json:"strategy"`               // "select" | "url-test"
	Name     string `yaml:"name,omitempty" json:"name,omitempty"`   // Type=group 时必填
	Match    string `yaml:"match,omitempty" json:"match,omitempty"` // Type=select 时必填
}

// Group 定义一个区域节点组。
type Group struct {
	Match    string `yaml:"match" json:"match"`
	Strategy string `yaml:"strategy" json:"strategy"` // "select" | "url-test"
}

// Filters 定义订阅节点的过滤规则。
type Filters struct {
	Exclude string `yaml:"exclude,omitempty" json:"exclude,omitempty"`
}
