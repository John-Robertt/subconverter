package config

import (
	"fmt"
	"iter"
	"slices"

	"gopkg.in/yaml.v3"
)

// OrderedMap is a generic ordered map with string keys.
// It preserves YAML key declaration order for groups, routing, and rulesets.
// After unmarshaling, it is read-only.
type OrderedMap[V any] struct {
	keys   []string
	values map[string]V
}

// UnmarshalYAML implements yaml.Unmarshaler.
// It expects a MappingNode and walks Content pairs in order.
func (m *OrderedMap[V]) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("orderedmap: expected mapping node, got kind %d", node.Kind)
	}

	n := len(node.Content) / 2
	m.keys = make([]string, 0, n)
	m.values = make(map[string]V, n)

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		key := keyNode.Value

		if _, exists := m.values[key]; exists {
			return fmt.Errorf("orderedmap: duplicate key %q at line %d", key, keyNode.Line)
		}

		var val V
		if err := valNode.Decode(&val); err != nil {
			return fmt.Errorf("orderedmap: decoding value for key %q: %w", key, err)
		}

		m.keys = append(m.keys, key)
		m.values[key] = val
	}

	return nil
}

// Keys returns a copy of the keys in declaration order.
func (m *OrderedMap[V]) Keys() []string {
	return slices.Clone(m.keys)
}

// Get returns the value for a key and whether it exists.
func (m *OrderedMap[V]) Get(key string) (V, bool) {
	if m.values == nil {
		var zero V
		return zero, false
	}
	v, ok := m.values[key]
	return v, ok
}

// Len returns the number of entries.
func (m *OrderedMap[V]) Len() int {
	return len(m.keys)
}

// Entries returns an iterator over key-value pairs in declaration order.
func (m *OrderedMap[V]) Entries() iter.Seq2[string, V] {
	return func(yield func(string, V) bool) {
		for _, k := range m.keys {
			if !yield(k, m.values[k]) {
				return
			}
		}
	}
}
