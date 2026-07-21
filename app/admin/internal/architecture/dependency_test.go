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
	assertPathExists(t, "../../cmd/server/main.go")
	assertPathExists(t, "../../cmd/tools/main.go")
	assertPathExists(t, "../biz")
	assertPathExists(t, "../data")
	assertPathMissing(t, "../../../../internal")
	assertPathExists(t, "../../../frontend/package.json")
	assertPathExists(t, "../../../../docker-compose.yml")
	assertPathExists(t, "../../../../configs/config.yaml")
	assertPathExists(t, "../../../../configs/config.docker.yaml")
	assertPathMissing(t, "../../../../frontend")
	assertPathMissing(t, "../../../../deploy/docker-compose.yml")
	assertPathMissing(t, "../../../../configs/admin.yaml")
	assertPathMissing(t, "../../../../configs/worker.yaml")
	assertPathMissing(t, "../../../../.env.example")
	assertFileNotContains(t, "../conf/config.go", "configenv.NewSource")
	assertFileNotContains(t, "../../../../Makefile", "--env-file")
	assertFileNotContains(t, "../../../../Makefile", "COMPOSE_ENV_FILE")
	assertFileNotContains(t, "../../../../Makefile", "KRATOS_ADMIN_")
	assertFileNotContains(t, "../../../../docker-compose.yml", "env_file:")
	assertFileNotContains(t, "../../../../docker-compose.yml", "KRATOS_ADMIN_")
	assertFileNotContains(t, "../../../../docker-compose.yml", "configs/admin.yaml")
	assertFileContains(t, "../../../../docker-compose.yml", "configs/config.docker.yaml")
}

func TestLocalMySQLGrantsTemporaryGORMGenDatabaseAccess(t *testing.T) {
	t.Parallel()
	const initSQL = "../../../../deploy/mysql/init/01-gorm-gen.sql"
	assertPathExists(t, initSQL)
	assertFileContains(t, "../../../../docker-compose.yml", "./deploy/mysql/init/01-gorm-gen.sql:/docker-entrypoint-initdb.d/01-gorm-gen.sql:ro")
	assertFileContains(t, initSQL, "GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, INDEX ON `kratos\\_admin\\_gen\\_%`.* TO 'kratos'@'%'")
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

func assertFileNotContains(t *testing.T, path, unexpected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	if strings.Contains(string(content), unexpected) {
		t.Fatalf("%s 不应包含 %q", path, unexpected)
	}
}

func TestLayerDependencies(t *testing.T) {
	t.Parallel()
	assertNoImports(t, "../data", "/internal/service", "/internal/server")
	assertNoImports(t, "../biz", "/internal/data", "/internal/service", "/internal/server")
	assertNoImports(t, "../service", "/internal/data", "/internal/server")
}

func TestSingleAdminProcessHasNoLegacyWorkerDependency(t *testing.T) {
	t.Parallel()
	assertPathMissing(t, "../../../worker")
	assertNoImports(t, "../../..", "github.com/hibiken/asynq", "/app/worker")
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
