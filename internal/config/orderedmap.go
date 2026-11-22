package config

import (
	"encoding/json"
	"fmt"
	"iter"
	"slices"

	"gopkg.in/yaml.v3"
)

// OrderedMap 是以字符串为键的泛型有序映射。
// 它为 groups、routing 和 rulesets 保留 YAML 键的声明顺序。
// 反序列化完成后只读。
type OrderedMap[V any] struct {
	keys   []string
	values map[string]V
}

type orderedMapEntry[V any] struct {
	Key   string `json:"key"`
	Value V      `json:"value"`
}

// UnmarshalYAML 实现 yaml.Unmarshaler。
// 它要求传入 MappingNode，并按顺序遍历 Content 中的键值对。
func (m *OrderedMap[V]) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("orderedmap: 应为映射节点，实际节点类型为 %d", node.Kind)
	}

	n := len(node.Content) / 2
	m.keys = make([]string, 0, n)
	m.values = make(map[string]V, n)

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		key := keyNode.Value

		if _, exists := m.values[key]; exists {
			return fmt.Errorf("orderedmap: 键 %q 重复（第 %d 行）", key, keyNode.Line)
		}

		var val V
		if err := valNode.Decode(&val); err != nil {
			return fmt.Errorf("orderedmap: 解码键 %q 的值失败：%w", key, err)
		}

		m.keys = append(m.keys, key)
		m.values[key] = val
	}

	return nil
}

// MarshalYAML 实现 yaml.Marshaler。
// 它输出映射节点，让 YAML 回写时保持已存储的键顺序。
func (m OrderedMap[V]) MarshalYAML() (any, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, key := range m.keys {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
		var valueNode yaml.Node
		if err := valueNode.Encode(m.values[key]); err != nil {
			return nil, fmt.Errorf("orderedmap: 编码键 %q 的值失败：%w", key, err)
		}
		node.Content = append(node.Content, keyNode, &valueNode)
	}
	return node, nil
}

// MarshalJSON 实现 json.Marshaler。
// JSON 对象的键顺序不稳定，所以有序映射使用 [{key,value}] 结构。
func (m OrderedMap[V]) MarshalJSON() ([]byte, error) {
	entries := make([]orderedMapEntry[V], 0, len(m.keys))
	for _, key := range m.keys {
		entries = append(entries, orderedMapEntry[V]{
			Key:   key,
			Value: m.values[key],
		})
	}
	return json.Marshal(entries)
}

// UnmarshalJSON 为 API 使用的 [{key,value}] 结构实现 json.Unmarshaler。
func (m *OrderedMap[V]) UnmarshalJSON(data []byte) error {
	var entries []orderedMapEntry[V]
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("orderedmap: 应为 {key,value} 数组：%w", err)
	}

	m.keys = make([]string, 0, len(entries))
	m.values = make(map[string]V, len(entries))
	for i, entry := range entries {
		if entry.Key == "" {
			return fmt.Errorf("orderedmap: 第 %d 项的键为空", i)
		}
		if _, exists := m.values[entry.Key]; exists {
			return fmt.Errorf("orderedmap: 键 %q 重复", entry.Key)
		}
		m.keys = append(m.keys, entry.Key)
		m.values[entry.Key] = entry.Value
	}
	return nil
}

// Keys 按声明顺序返回键的副本。
func (m *OrderedMap[V]) Keys() []string {
	return slices.Clone(m.keys)
}

// Get 返回键对应的值，以及该键是否存在。
func (m *OrderedMap[V]) Get(key string) (V, bool) {
	if m.values == nil {
		var zero V
		return zero, false
	}
	v, ok := m.values[key]
	return v, ok
}

// Len 返回条目数量。
func (m *OrderedMap[V]) Len() int {
	return len(m.keys)
}

// Entries 返回一个按声明顺序遍历键值对的迭代器。
func (m *OrderedMap[V]) Entries() iter.Seq2[string, V] {
	return func(yield func(string, V) bool) {
		for _, k := range m.keys {
			if !yield(k, m.values[k]) {
				return
			}
		}
	}
}
