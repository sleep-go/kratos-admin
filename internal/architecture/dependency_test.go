package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestLayerDependencies(t *testing.T) {
	t.Parallel()
	assertNoImports(t, "../data", "/service", "/server", "/app/admin/internal", "/app/worker/internal")
	assertNoImports(t, "../biz", "/internal/data", "/internal/service", "/internal/server", "/app/admin/internal", "/app/worker/internal")
	assertNoImports(t, "../../app/admin", "/app/worker/internal")
	assertNoImports(t, "../../app/worker", "/app/admin/internal")
}

func assertNoImports(t *testing.T, root string, forbidden ...string) {
	t.Helper()
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range parsed.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			for _, fragment := range forbidden {
				if strings.Contains(importPath, fragment) {
					t.Errorf("%s 禁止导入 %s", path, importPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫描 %s 失败: %v", root, err)
	}
}
