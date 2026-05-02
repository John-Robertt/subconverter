package config

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Config is the top-level user configuration.
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

// Templates declares optional base config templates per output format.
// Each value may be a local file path or an HTTP(S) URL.
type Templates struct {
	Clash string `yaml:"clash,omitempty" json:"clash,omitempty"`
	Surge string `yaml:"surge,omitempty" json:"surge,omitempty"`
}

// Sources declares subscription and custom proxy inputs.
//
// FetchOrder records the YAML declaration order of the fetch-kind keys
// (subscriptions / snell / vless) so the Source pipeline stage can traverse
// them in the user-declared order. It is populated by the custom UnmarshalYAML.
// custom_proxies is not a fetch-kind and is excluded from FetchOrder.
type Sources struct {
	Subscriptions []Subscription `yaml:"subscriptions" json:"subscriptions"`
	Snell         []SnellSource  `yaml:"snell" json:"snell"`
	VLess         []VLessSource  `yaml:"vless" json:"vless"`
	CustomProxies []CustomProxy  `yaml:"custom_proxies" json:"custom_proxies"`

	// FetchOrder holds fetch-kind keys in YAML declaration order.
	// Set by UnmarshalYAML; empty when Sources is zero-valued (tests).
	FetchOrder []string `yaml:"-" json:"fetch_order"`
}

// Subscription is a single subscription source.
type Subscription struct {
	URL string `yaml:"url" json:"url"`
}

// SnellSource is a single Snell node source. The URL is fetched as plain text;
// each non-empty line is parsed as a Surge-style Snell proxy declaration
// (e.g. `HK = snell, 1.2.3.4, 57891, psk=xxx, version=4`).
//
// Snell nodes are Surge-only: they appear in Surge output and are filtered
// out of Clash output (Clash Meta does not support Snell v4/v5).
type SnellSource struct {
	URL string `yaml:"url" json:"url"`
}

// VLessSource is a single VLESS node source. The URL is fetched as plain text;
// each non-empty line is parsed as a standard VLESS URI
// (e.g. `vless://UUID@server:port?security=reality&...#name`).
//
// VLESS nodes are Clash-only: they appear in Clash output and are filtered
// out of Surge output (Surge does not natively support VLESS).
type VLessSource struct {
	URL string `yaml:"url" json:"url"`
}

// sourceFetchKeys enumerates the YAML keys under `sources:` that correspond
// to fetch-kind inputs (remote-sourced proxies). custom_proxies is excluded:
// it is user-declared inline and does not participate in FetchOrder.
var sourceFetchKeys = map[string]bool{
	"subscriptions": true,
	"snell":         true,
	"vless":         true,
}

var defaultSourceFetchOrder = []string{"subscriptions", "snell", "vless"}

// UnmarshalYAML implements yaml.Unmarshaler for Sources.
//
// It walks the mapping node in YAML declaration order, decoding each known key
// into the corresponding struct field AND appending fetch-kind keys to
// FetchOrder. The Source pipeline stage uses FetchOrder to traverse fetch
// inputs in the exact order the user wrote them (e.g. snell → vless →
// subscriptions), which determines the final proxy slice ordering.
//
// Reference pattern: OrderedMap.UnmarshalYAML in orderedmap.go.
//
// Design decisions embedded here:
//   - Unknown top-level keys error out (protects against typos like "vles").
//   - A key appearing twice errors out (matches OrderedMap's approach).
//   - An empty section (e.g. `vless:` with nil body or empty list) still
//     records the key in FetchOrder: declaration intent matters even if the
//     list is empty — the Source loop simply has nothing to fetch for that
//     kind, producing no proxies.
//   - custom_proxies is decoded but NOT recorded in FetchOrder.
func (s *Sources) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("sources: expected mapping node, got kind %d", node.Kind)
	}

	seen := make(map[string]bool, len(node.Content)/2)

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		key := keyNode.Value

		if seen[key] {
			return fmt.Errorf("sources: duplicate key %q at line %d", key, keyNode.Line)
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
			return fmt.Errorf("sources: unknown key %q at line %d", key, keyNode.Line)
		}

		if sourceFetchKeys[key] {
			s.FetchOrder = append(s.FetchOrder, key)
		}
	}

	return nil
}

// MarshalYAML implements yaml.Marshaler for stable sources key order.
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
			return nil, fmt.Errorf("sources.fetch_order: unknown key %q", key)
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

// MarshalJSON emits sources with complete fetch_order and [] instead of null
// for list fields so API round trips are stable.
func (s Sources) MarshalJSON() ([]byte, error) {
	type sourcesJSON struct {
		Subscriptions []Subscription `json:"subscriptions"`
		Snell         []SnellSource  `json:"snell"`
		VLess         []VLessSource  `json:"vless"`
		CustomProxies []CustomProxy  `json:"custom_proxies"`
		FetchOrder    []string       `json:"fetch_order"`
	}
	return json.Marshal(sourcesJSON{
		Subscriptions: nonNilSlice(s.Subscriptions),
		Snell:         nonNilSlice(s.Snell),
		VLess:         nonNilSlice(s.VLess),
		CustomProxies: nonNilSlice(s.CustomProxies),
		FetchOrder:    completeSourceFetchOrder(s.FetchOrder),
	})
}

// UnmarshalJSON accepts the API object shape. fetch_order validation is done
// in Prepare so callers can receive structured diagnostics.
func (s *Sources) UnmarshalJSON(data []byte) error {
	type sourcesJSON struct {
		Subscriptions []Subscription `json:"subscriptions"`
		Snell         []SnellSource  `json:"snell"`
		VLess         []VLessSource  `json:"vless"`
		CustomProxies []CustomProxy  `json:"custom_proxies"`
		FetchOrder    []string       `json:"fetch_order"`
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

// CustomProxy is a user-defined proxy node declared via a protocol URL.
// The runtime parse result is produced later by pipeline.Source; the config
// layer only preserves raw YAML fields.
type CustomProxy struct {
	URL          string        `yaml:"url" json:"url"`
	Name         string        `yaml:"name" json:"name"`
	RelayThrough *RelayThrough `yaml:"relay_through,omitempty" json:"relay_through,omitempty"`
}

// RelayThrough defines how a custom proxy chains through upstream nodes.
type RelayThrough struct {
	Type     string `yaml:"type" json:"type"`                       // "group" | "select" | "all"
	Strategy string `yaml:"strategy" json:"strategy"`               // "select" | "url-test"
	Name     string `yaml:"name,omitempty" json:"name,omitempty"`   // required when Type=group
	Match    string `yaml:"match,omitempty" json:"match,omitempty"` // required when Type=select
}

// Group defines a region node group.
type Group struct {
	Match    string `yaml:"match" json:"match"`
	Strategy string `yaml:"strategy" json:"strategy"` // "select" | "url-test"
}

// Filters defines subscription node filtering rules.
type Filters struct {
	Exclude string `yaml:"exclude,omitempty" json:"exclude,omitempty"`
}
