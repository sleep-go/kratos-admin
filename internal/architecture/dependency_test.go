package architecture_test

import (
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMonorepoEntrypoints(t *testing.T) {
	t.Parallel()
	assertPathExists(t, "../../app/frontend/package.json")
	assertPathExists(t, "../../docker-compose.yml")
	assertPathMissing(t, "../../frontend")
	assertPathMissing(t, "../../deploy/docker-compose.yml")
	assertFileContains(t, "../../docker-compose.yml", "${KRATOS_ADMIN_ENV_FILE:-.env.example}")
}

func assertFileContains(t *testing.T, path, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	if !strings.Contains(string(content), expected) {
		t.Fatalf("%s 未包含 %q", path, expected)
	}
}

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

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("大仓入口 %s 不存在: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("旧入口 %s 仍然存在", path)
	}
}
