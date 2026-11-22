package admin

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// T-ADM-013：admin 包不直接引入 pipeline 或 model
func TestAdminDoesNotImportPipelineOrModel(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(".", entry.Name())
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("解析 %s 失败：%v", path, err)
		}
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if strings.HasSuffix(importPath, "/internal/pipeline") ||
				strings.HasSuffix(importPath, "/internal/model") ||
				strings.HasSuffix(importPath, "/internal/generate") {
				t.Fatalf("%s 引入了禁止引用的包 %s", path, importPath)
			}
		}
	}
}
