package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jinzhu/inflection"
	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/sleep-go/kratos-admin/migrations"
)

const generatedModelImportPath = "github.com/sleep-go/kratos-admin/app/admin/internal/data/model"

var (
	booleanModelColumns = map[string]struct{}{
		"allow_tenant_override": {},
		"is_builtin":            {},
		"is_default":            {},
		"is_platform_admin":     {},
		"is_primary":            {},
		"is_secret":             {},
		"is_tenant_admin":       {},
		"mfa_enabled":           {},
		"visible":               {},
	}
	stagingModelImportPattern = regexp.MustCompile(`"[^"]*/gormgen-stage-[^"]*/model"`)
	generatedFieldNames       = map[string]string{
		"duration_ms":      "DurationMS",
		"etag":             "ETag",
		"mfa_channel":      "MFAChannel",
		"mfa_enabled":      "MFAEnabled",
		"refresh_jti_hash": "RefreshJTIHash",
		"sha256":           "SHA256",
	}
	jsonModelColumns = []string{
		"after_data",
		"before_data",
		"context_data",
		"filters",
		"payload",
		"request_data",
		"setting_value",
	}
)

type generatedManifest struct {
	Files []string `json:"files"`
}

type generatedDirectory struct {
	stagingPath string
	targetPath  string
	rewrite     func([]byte) []byte
}

type generatedFile struct {
	path    string
	content []byte
}

type fileSnapshot struct {
	path    string
	content []byte
	mode    os.FileMode
	exists  bool
}

type genOptions struct {
	ConfPath     string
	ModelOutPath string
	QueryOutPath string
}

func modelNameForTable(table string) string {
	name := schema.NamingStrategy{}.SchemaName(inflection.Singular(table))
	if name == "ApiAccessLog" {
		return "APIAccessLog"
	}
	return name
}

func modelTypeForColumn(column string) string {
	if _, ok := booleanModelColumns[column]; ok {
		return "bool"
	}
	return ""
}

func businessTableNames(tables []string) []string {
	unique := make(map[string]struct{}, len(tables))
	for _, table := range tables {
		if table == "" || table == "goose_db_version" {
			continue
		}
		unique[table] = struct{}{}
	}
	result := make([]string, 0, len(unique))
	for table := range unique {
		result = append(result, table)
	}
	sort.Strings(result)
	return result
}

func scalarModelOptionsForTable(_ string) []gen.ModelOpt {
	options := make([]gen.ModelOpt, 0, len(booleanModelColumns)+len(generatedFieldNames)+len(jsonModelColumns)+1)
	columns := make([]string, 0, len(booleanModelColumns))
	for column := range booleanModelColumns {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	for _, column := range columns {
		options = append(options, gen.FieldType(column, "bool"))
	}
	fieldColumns := make([]string, 0, len(generatedFieldNames))
	for column := range generatedFieldNames {
		fieldColumns = append(fieldColumns, column)
	}
	sort.Strings(fieldColumns)
	for _, column := range fieldColumns {
		options = append(options, gen.FieldRename(column, generatedFieldNames[column]))
	}
	for _, column := range jsonModelColumns {
		options = append(options, gen.FieldType(column, "datatypes.JSON"))
	}
	options = append(options, gen.FieldType("deleted_at", "gorm.DeletedAt"))
	return options
}

func relationConfig(foreignKey, references string, slice bool) *field.RelateConfig {
	return &field.RelateConfig{
		RelatePointer:      !slice,
		RelateSlicePointer: slice,
		GORMTag: field.GormTag{
			"foreignKey": {foreignKey},
			"references": {references},
		},
	}
}

func generateGORMArtifacts(ctx context.Context, dsn string, options genOptions) (resultErr error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			resultErr = fmt.Errorf("GORM Gen 反向生成失败: %v", recovered)
		}
	}()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接代码生成 MySQL 失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取代码生成数据库连接池失败: %w", err)
	}
	defer sqlDB.Close()
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("检查代码生成 MySQL 连接失败: %w", err)
	}
	if err := migrations.Apply(ctx, sqlDB); err != nil {
		return err
	}
	tables, err := db.Migrator().GetTables()
	if err != nil {
		return fmt.Errorf("读取代码生成数据库表失败: %w", err)
	}
	tables = businessTableNames(tables)
	if len(tables) == 0 {
		return errors.New("代码生成数据库中没有业务表")
	}
	return generateGORMArtifactsFromDatabase(db, tables, options)
}

func generateGORMArtifactsFromDatabase(db *gorm.DB, tables []string, options genOptions) error {
	modelTarget, err := filepath.Abs(options.ModelOutPath)
	if err != nil {
		return fmt.Errorf("解析 Model 输出目录失败: %w", err)
	}
	queryTarget, err := filepath.Abs(options.QueryOutPath)
	if err != nil {
		return fmt.Errorf("解析 Query 输出目录失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(modelTarget), 0o750); err != nil {
		return fmt.Errorf("创建代码生成暂存目录父目录失败: %w", err)
	}
	stagingRoot, err := os.MkdirTemp(filepath.Dir(modelTarget), "gormgen-stage-")
	if err != nil {
		return fmt.Errorf("创建代码生成暂存目录失败: %w", err)
	}
	defer os.RemoveAll(stagingRoot)
	stagingModel := filepath.Join(stagingRoot, "model")
	stagingQuery := filepath.Join(stagingRoot, "query")

	generator := gen.NewGenerator(gen.Config{
		OutPath:             stagingQuery,
		ModelPkgPath:        stagingModel,
		Mode:                gen.WithDefaultQuery | gen.WithQueryInterface,
		FieldNullable:       true,
		FieldSignable:       true,
		FieldWithIndexTag:   true,
		FieldWithTypeTag:    true,
		FieldWithDefaultTag: true,
	})
	generator.UseDB(db)
	generator.WithImportPkgPath("gorm.io/datatypes", "gorm.io/gorm")
	generator.WithDataTypeMap(map[string]func(gorm.ColumnType) string{
		"bigint":   func(gorm.ColumnType) string { return "int64" },
		"int":      func(gorm.ColumnType) string { return "int32" },
		"json":     func(gorm.ColumnType) string { return "datatypes.JSON" },
		"smallint": func(gorm.ColumnType) string { return "int16" },
		"tinyint":  func(gorm.ColumnType) string { return "int8" },
	})

	models := make([]interface{}, len(tables))
	indexes := make(map[string]int, len(tables))
	for index, table := range tables {
		models[index] = generator.GenerateModelAs(table, modelNameForTable(table), scalarModelOptionsForTable(table)...)
		indexes[table] = index
	}
	applyGeneratedRelations(generator, models, indexes)
	generator.ApplyBasic(models...)
	generator.Execute()

	return promoteGeneratedArtifacts([]generatedDirectory{
		{stagingPath: stagingModel, targetPath: modelTarget},
		{stagingPath: stagingQuery, targetPath: queryTarget, rewrite: rewriteGeneratedModelImport},
	})
}

func applyGeneratedRelations(generator *gen.Generator, models []interface{}, indexes map[string]int) {
	generate := func(table string, options ...gen.ModelOpt) interface{} {
		return generator.GenerateModelAs(table, modelNameForTable(table), append(scalarModelOptionsForTable(table), options...)...)
	}
	replace := func(table string, meta interface{}) {
		if index, ok := indexes[table]; ok {
			models[index] = meta
		}
	}

	tenantMemberForUser := generator.GenerateModelAs("tenant_members", modelNameForTable("tenant_members"), scalarModelOptionsForTable("tenant_members")...)
	replace("users", generate("users", gen.FieldRelate(field.HasMany, "Members", tenantMemberForUser, relationConfig("UserID", "ID", true))))
	tenantMemberForTenant := generator.GenerateModelAs("tenant_members", modelNameForTable("tenant_members"), scalarModelOptionsForTable("tenant_members")...)
	replace("tenants", generate("tenants", gen.FieldRelate(field.HasMany, "Members", tenantMemberForTenant, relationConfig("TenantID", "ID", true))))

	user := generator.GenerateModelAs("users", modelNameForTable("users"), scalarModelOptionsForTable("users")...)
	department := generator.GenerateModelAs("departments", modelNameForTable("departments"), scalarModelOptionsForTable("departments")...)
	position := generator.GenerateModelAs("positions", modelNameForTable("positions"), scalarModelOptionsForTable("positions")...)
	replace("tenant_members", generate("tenant_members",
		gen.FieldRelate(field.BelongsTo, "User", user, relationConfig("UserID", "ID", false)),
		gen.FieldRelate(field.BelongsTo, "PrimaryDepartment", department, relationConfig("PrimaryDepartmentID", "ID", false)),
		gen.FieldRelate(field.BelongsTo, "Position", position, relationConfig("PositionID", "ID", false)),
	))

	tenant := generator.GenerateModelAs("tenants", modelNameForTable("tenants"), scalarModelOptionsForTable("tenants")...)
	replace("roles", generate("roles", gen.FieldRelate(field.BelongsTo, "Tenant", tenant, relationConfig("TenantID", "ID", false))))
	fileReference := generator.GenerateModelAs("file_references", modelNameForTable("file_references"), scalarModelOptionsForTable("file_references")...)
	replace("files", generate("files", gen.FieldRelate(field.HasMany, "References", fileReference, relationConfig("FileID", "ID", true))))
	file := generator.GenerateModelAs("files", modelNameForTable("files"), scalarModelOptionsForTable("files")...)
	replace("log_exports", generate("log_exports", gen.FieldRelate(field.BelongsTo, "File", file, relationConfig("FileID", "ID", false))))
}

func rewriteGeneratedModelImport(content []byte) []byte {
	return stagingModelImportPattern.ReplaceAll(content, []byte(`"`+generatedModelImportPath+`"`))
}

func promoteGeneratedArtifacts(directories []generatedDirectory) error {
	files, removals, manifests, err := prepareGeneratedArtifacts(directories)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(files)+len(removals)+len(manifests))
	for _, file := range files {
		paths = append(paths, file.path)
	}
	paths = append(paths, removals...)
	for _, manifest := range manifests {
		paths = append(paths, manifest.path)
	}
	snapshots, err := snapshotFiles(paths)
	if err != nil {
		return err
	}
	apply := func() error {
		for _, directory := range directories {
			if err := os.MkdirAll(directory.targetPath, 0o750); err != nil {
				return err
			}
		}
		for _, file := range files {
			if err := writeFileAtomically(file.path, file.content, 0o640); err != nil {
				return err
			}
		}
		for _, path := range removals {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		for _, manifest := range manifests {
			if err := writeFileAtomically(manifest.path, manifest.content, 0o640); err != nil {
				return err
			}
		}
		return nil
	}
	if err := apply(); err != nil {
		return errors.Join(fmt.Errorf("提升 GORM Gen 生成文件失败: %w", err), restoreSnapshots(snapshots))
	}
	return nil
}

func prepareGeneratedArtifacts(directories []generatedDirectory) (files []generatedFile, removals []string, manifests []generatedFile, err error) {
	for _, directory := range directories {
		matches, globErr := filepath.Glob(filepath.Join(directory.stagingPath, "*.gen.go"))
		if globErr != nil {
			return nil, nil, nil, globErr
		}
		sort.Strings(matches)
		names := make([]string, 0, len(matches))
		for _, match := range matches {
			content, readErr := os.ReadFile(match)
			if readErr != nil {
				return nil, nil, nil, readErr
			}
			if directory.rewrite != nil {
				content = directory.rewrite(content)
			}
			name := filepath.Base(match)
			names = append(names, name)
			files = append(files, generatedFile{path: filepath.Join(directory.targetPath, name), content: content})
		}
		manifestPath := filepath.Join(directory.targetPath, ".gormgen-manifest.json")
		previous, readErr := readGeneratedManifest(manifestPath)
		if readErr != nil {
			return nil, nil, nil, readErr
		}
		current := make(map[string]struct{}, len(names))
		for _, name := range names {
			current[name] = struct{}{}
		}
		for _, name := range previous.Files {
			if _, ok := current[name]; !ok && strings.HasSuffix(name, ".gen.go") {
				removals = append(removals, filepath.Join(directory.targetPath, name))
			}
		}
		content, marshalErr := json.MarshalIndent(generatedManifest{Files: names}, "", "  ")
		if marshalErr != nil {
			return nil, nil, nil, marshalErr
		}
		manifests = append(manifests, generatedFile{path: manifestPath, content: append(content, '\n')})
	}
	return files, removals, manifests, nil
}

func readGeneratedManifest(path string) (generatedManifest, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return generatedManifest{}, nil
	}
	if err != nil {
		return generatedManifest{}, err
	}
	var manifest generatedManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return generatedManifest{}, fmt.Errorf("解析 GORM Gen 清单失败: %w", err)
	}
	return manifest, nil
}

func snapshotFiles(paths []string) ([]fileSnapshot, error) {
	unique := make(map[string]struct{}, len(paths))
	result := make([]fileSnapshot, 0, len(paths))
	for _, path := range paths {
		if _, ok := unique[path]; ok {
			continue
		}
		unique[path] = struct{}{}
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			result = append(result, fileSnapshot{path: path})
			continue
		}
		if err != nil {
			return nil, err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		result = append(result, fileSnapshot{path: path, content: content, mode: info.Mode().Perm(), exists: true})
	}
	return result, nil
}

func restoreSnapshots(snapshots []fileSnapshot) error {
	var restoreErrors []error
	for _, snapshot := range snapshots {
		if snapshot.exists {
			restoreErrors = append(restoreErrors, writeFileAtomically(snapshot.path, snapshot.content, snapshot.mode))
		} else if err := os.Remove(snapshot.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			restoreErrors = append(restoreErrors, err)
		}
	}
	return errors.Join(restoreErrors...)
}

func writeFileAtomically(path string, content []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".gormgen-write-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
