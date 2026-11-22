package config

import (
	"encoding/json"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

// T-CFG-005：有序映射保留插入顺序
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
		t.Errorf("Keys() = %v，期望 %v", m.Keys(), wantKeys)
	}
}

// T-CFG-006：值为结构体的有序映射
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
		t.Fatalf("Len() = %d，期望 2", m.Len())
	}

	hk, ok := m.Get("HK")
	if !ok {
		t.Fatal("Get(HK) 未找到")
	}
	if hk.Match != "(港|HK)" || hk.Strategy != "select" {
		t.Errorf("HK = %+v", hk)
	}
}

// T-CFG-007：值为切片的有序映射
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
		t.Errorf("Keys() = %v，期望 %v", m.Keys(), wantKeys)
	}

	fast, _ := m.Get("fast")
	if !slices.Equal(fast, []string{"HK", "SG", "DIRECT"}) {
		t.Errorf("fast = %v", fast)
	}
}

// T-CFG-008：键不存在时有序映射的 Get 返回零值
func TestOrderedMap_Get_Missing(t *testing.T) {
	input := `a: 1`
	var m OrderedMap[int]
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatal(err)
	}

	_, ok := m.Get("missing")
	if ok {
		t.Error("Get(missing) 应返回 false")
	}
}

// T-CFG-009：有序映射能处理空 YAML
func TestOrderedMap_Empty(t *testing.T) {
	input := `{}`
	var m OrderedMap[int]
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatal(err)
	}

	if m.Len() != 0 {
		t.Errorf("Len() = %d，期望 0", m.Len())
	}
}

// T-CFG-010：YAML 不是映射时有序映射返回错误
func TestOrderedMap_NonMapping_Error(t *testing.T) {
	input := `[1, 2, 3]`
	var m OrderedMap[int]
	if err := yaml.Unmarshal([]byte(input), &m); err == nil {
		t.Error("输入不是映射时应返回错误")
	}
}

// T-CFG-011：键重复时有序映射返回错误
func TestOrderedMap_DuplicateKey_Error(t *testing.T) {
	input := `
a: 1
b: 2
a: 3
`
	var m OrderedMap[int]
	if err := yaml.Unmarshal([]byte(input), &m); err == nil {
		t.Error("键重复时应返回错误")
	}
}

// T-CFG-012：有序映射的零值可以安全使用
func TestOrderedMap_ZeroValue_Safe(t *testing.T) {
	var m OrderedMap[int]

	if m.Len() != 0 {
		t.Errorf("Len() = %d，期望 0", m.Len())
	}
	if m.Keys() != nil {
		t.Errorf("Keys() = %v，期望 nil", m.Keys())
	}
	_, ok := m.Get("any")
	if ok {
		t.Error("在零值上调用 Get 应返回 false")
	}

	// 在零值上调用 Entries 不应 panic
	count := 0
	for range m.Entries() {
		count++
	}
	if count != 0 {
		t.Errorf("Entries() 产出 %d 项，期望 0", count)
	}
}

// T-CFG-013：有序映射的 Entries 按顺序返回键值对
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

// T-CFG-014：有序映射经 JSON 往返后保持顺序
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

// T-CFG-015：有序映射序列化为 YAML 时保持顺序
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
