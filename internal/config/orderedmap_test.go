package config

import (
	"encoding/json"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOrderedMap_PreservesOrder(t *testing.T) {
	input := `
c: 3
a: 1
b: 2
`
	var m OrderedMap[int]
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatal(err)
	}

	wantKeys := []string{"c", "a", "b"}
	if !slices.Equal(m.Keys(), wantKeys) {
		t.Errorf("Keys() = %v, want %v", m.Keys(), wantKeys)
	}
}

func TestOrderedMap_StructValue(t *testing.T) {
	type item struct {
		Match    string `yaml:"match"`
		Strategy string `yaml:"strategy"`
	}
	input := `
HK: { match: "(港|HK)", strategy: select }
SG: { match: "(SG)", strategy: url-test }
`
	var m OrderedMap[item]
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatal(err)
	}

	if m.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", m.Len())
	}

	hk, ok := m.Get("HK")
	if !ok {
		t.Fatal("Get(HK) not found")
	}
	if hk.Match != "(港|HK)" || hk.Strategy != "select" {
		t.Errorf("HK = %+v", hk)
	}
}

func TestOrderedMap_SliceValue(t *testing.T) {
	input := `
fast: [HK, SG, DIRECT]
manual: ["@all"]
`
	var m OrderedMap[[]string]
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatal(err)
	}

	wantKeys := []string{"fast", "manual"}
	if !slices.Equal(m.Keys(), wantKeys) {
		t.Errorf("Keys() = %v, want %v", m.Keys(), wantKeys)
	}

	fast, _ := m.Get("fast")
	if !slices.Equal(fast, []string{"HK", "SG", "DIRECT"}) {
		t.Errorf("fast = %v", fast)
	}
}

func TestOrderedMap_Get_Missing(t *testing.T) {
	input := `a: 1`
	var m OrderedMap[int]
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatal(err)
	}

	_, ok := m.Get("missing")
	if ok {
		t.Error("Get(missing) should return false")
	}
}

func TestOrderedMap_Empty(t *testing.T) {
	input := `{}`
	var m OrderedMap[int]
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatal(err)
	}

	if m.Len() != 0 {
		t.Errorf("Len() = %d, want 0", m.Len())
	}
}

func TestOrderedMap_NonMapping_Error(t *testing.T) {
	input := `[1, 2, 3]`
	var m OrderedMap[int]
	if err := yaml.Unmarshal([]byte(input), &m); err == nil {
		t.Error("expected error for non-mapping input")
	}
}

func TestOrderedMap_DuplicateKey_Error(t *testing.T) {
	input := `
a: 1
b: 2
a: 3
`
	var m OrderedMap[int]
	if err := yaml.Unmarshal([]byte(input), &m); err == nil {
		t.Error("expected error for duplicate key")
	}
}

func TestOrderedMap_ZeroValue_Safe(t *testing.T) {
	var m OrderedMap[int]

	if m.Len() != 0 {
		t.Errorf("Len() = %d, want 0", m.Len())
	}
	if m.Keys() != nil {
		t.Errorf("Keys() = %v, want nil", m.Keys())
	}
	_, ok := m.Get("any")
	if ok {
		t.Error("Get on zero value should return false")
	}

	// Entries on zero value should not panic
	count := 0
	for range m.Entries() {
		count++
	}
	if count != 0 {
		t.Errorf("Entries() yielded %d items, want 0", count)
	}
}

func TestOrderedMap_Entries(t *testing.T) {
	input := `
z: 26
a: 1
m: 13
`
	var m OrderedMap[int]
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatal(err)
	}

	var keys []string
	var vals []int
	for k, v := range m.Entries() {
		keys = append(keys, k)
		vals = append(vals, v)
	}

	if !slices.Equal(keys, []string{"z", "a", "m"}) {
		t.Errorf("keys = %v", keys)
	}
	if !slices.Equal(vals, []int{26, 1, 13}) {
		t.Errorf("vals = %v", vals)
	}
}

func TestOrderedMap_JSONRoundTripPreservesOrder(t *testing.T) {
	input := []byte(`[
		{"key":"HK","value":{"match":"(HK)","strategy":"select"}},
		{"key":"SG","value":{"match":"(SG)","strategy":"url-test"}}
	]`)
	var m OrderedMap[Group]
	if err := json.Unmarshal(input, &m); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if !slices.Equal(m.Keys(), []string{"HK", "SG"}) {
		t.Fatalf("Keys() = %v", m.Keys())
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if string(out) != `[{"key":"HK","value":{"match":"(HK)","strategy":"select"}},{"key":"SG","value":{"match":"(SG)","strategy":"url-test"}}]` {
		t.Fatalf("JSON = %s", out)
	}
}

func TestOrderedMap_MarshalYAMLPreservesOrder(t *testing.T) {
	var m OrderedMap[int]
	if err := json.Unmarshal([]byte(`[{"key":"b","value":2},{"key":"a","value":1}]`), &m); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	if string(out) != "b: 2\na: 1\n" {
		t.Fatalf("YAML = %q", out)
	}
}
