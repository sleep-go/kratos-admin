package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestModelNameForTable(t *testing.T) {
	tests := []struct {
		table string
		model string
	}{
		{table: "api_access_logs", model: "APIAccessLog"},
		{table: "auth_sessions", model: "AuthSession"},
		{table: "casbin_rules", model: "CasbinRule"},
		{table: "tenant_admins", model: "TenantAdmin"},
	}
	for _, test := range tests {
		t.Run(test.table, func(t *testing.T) {
			if got := modelNameForTable(test.table); got != test.model {
				t.Fatalf("modelNameForTable(%q) = %q, want %q", test.table, got, test.model)
			}
		})
	}
}

func TestModelTypeForBooleanColumns(t *testing.T) {
	columns := []string{
		"mfa_enabled",
		"is_super_admin",
		"visible",
		"is_secret",
		"allow_tenant_override",
	}
	for _, column := range columns {
		if got := modelTypeForColumn(column); got != "bool" {
			t.Errorf("modelTypeForColumn(%q) = %q, want bool", column, got)
		}
	}
}

func TestModelFieldNamesPreserveInitialisms(t *testing.T) {
	want := map[string]string{
		"duration_ms":      "DurationMS",
		"etag":             "ETag",
		"mfa_enabled":      "MFAEnabled",
		"refresh_jti_hash": "RefreshJTIHash",
		"sha256":           "SHA256",
	}
	for column, fieldName := range want {
		if got := generatedFieldNames[column]; got != fieldName {
			t.Errorf("generatedFieldNames[%q] = %q, want %q", column, got, fieldName)
		}
	}
}

func TestBusinessTableNamesExcludeGooseAndSort(t *testing.T) {
	got := businessTableNames([]string{"users", "goose_db_version", "auth_sessions", "users"})
	want := []string{"auth_sessions", "users"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("businessTableNames() = %v, want %v", got, want)
	}
}

func TestGeneratedArtifactsPreserveHandwrittenFilesAndRemoveStaleGeneratedFiles(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, "staging")
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(staging, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "users.gen.go"), []byte("package model\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "extensions.go"), []byte("package model\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	manifest, err := json.Marshal(generatedManifest{Files: []string{"stale.gen.go"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, ".gormgen-manifest.json"), manifest, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "stale.gen.go"), []byte("package model\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := promoteGeneratedArtifacts([]generatedDirectory{{stagingPath: staging, targetPath: target}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "extensions.go")); err != nil {
		t.Fatalf("手写扩展文件未保留: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "stale.gen.go")); !os.IsNotExist(err) {
		t.Fatalf("过期生成文件仍存在: %v", err)
	}
}

func TestGeneratedArtifactsFromMigratedMySQLAreDeterministic(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_GORM_GEN_DSN")
	if dsn == "" {
		dsn = os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("未配置 GORM Gen MySQL DSN，跳过反向生成集成测试")
	}
	dataDirectory, err := filepath.Abs("../../internal/data")
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(dataDirectory, "gormgen-integration-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	options := genOptions{
		ModelOutPath: filepath.Join(root, "model"),
		QueryOutPath: filepath.Join(root, "query"),
	}
	run := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := withTemporaryDatabase(ctx, dsn, func(temporaryDSN string) error {
			return generateGORMArtifacts(ctx, temporaryDSN, options)
		}); err != nil {
			t.Fatal(err)
		}
	}
	run()
	if _, err := os.Stat(filepath.Join(options.ModelOutPath, "users.gen.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(options.QueryOutPath, "users.gen.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(options.ModelOutPath, "goose_db_version.gen.go")); !os.IsNotExist(err) {
		t.Fatalf("不应生成 goose_db_version: %v", err)
	}
	querySource, err := os.ReadFile(filepath.Join(options.QueryOutPath, "users.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(querySource), `"`+generatedModelImportPath+`"`) {
		t.Fatal("Query 未导入正式 Model 路径")
	}
	first := generatedDirectoryHash(t, root)
	run()
	second := generatedDirectoryHash(t, root)
	if first != second {
		t.Fatalf("连续生成结果不一致: %s != %s", first, second)
	}
}

func generatedDirectoryHash(t *testing.T, root string) string {
	t.Helper()
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fmt.Fprintf(hash, "%s\x00", strings.TrimPrefix(path, root))
		_, _ = hash.Write(content)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}
