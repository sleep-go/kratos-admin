package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var forbiddenGORMCalls = map[string]struct{}{
	"Table": {},
	"Raw":   {},
	"Exec":  {},
	"Model": {},
	"Joins": {},
}

var gormGenGuardAllowedFiles = map[string]struct{}{
	"migration.go": {},
}

func TestRepositoriesUseGORMGen(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	err := filepath.WalkDir("../data", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path == "../data/model" || path == "../data/query" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if _, allowed := gormGenGuardAllowedFiles[filepath.Base(path)]; allowed {
			return nil
		}
		parsed, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, violation := range gormGenViolations(fset, parsed) {
			t.Errorf("%s:%d 禁止业务仓储绕过 GORM Gen：%s", path, violation.line, violation.reason)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫描数据仓储失败: %v", err)
	}
}

type gormGenViolation struct {
	line   int
	reason string
}

func gormGenViolations(fset *token.FileSet, parsed *ast.File) []gormGenViolation {
	violations := make([]gormGenViolation, 0)
	for _, imported := range parsed.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err == nil && path == "database/sql" {
			violations = append(violations, gormGenViolation{
				line: fset.Position(imported.Pos()).Line, reason: "直接导入 database/sql",
			})
		}
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if _, forbidden := forbiddenGORMCalls[selector.Sel.Name]; forbidden {
			violations = append(violations, gormGenViolation{
				line: fset.Position(selector.Sel.Pos()).Line, reason: "调用 ." + selector.Sel.Name + "()",
			})
		}
		return true
	})
	return violations
}

func TestGORMGenGuardDetectsForbiddenAccess(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "fixture.go", `package fixture
import "database/sql"
func query(db any) { db.Table("users"); db.Raw("SELECT 1"); _ = sql.ErrNoRows }
`, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	violations := gormGenViolations(fset, parsed)
	if len(violations) != 3 {
		t.Fatalf("违规数 = %d，期望 3：%+v", len(violations), violations)
	}
}
