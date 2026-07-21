# GORM Gen Database-First Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 以 Goose 迁移后的临时 MySQL 数据库为唯一来源反向生成 GORM Model 和 Query，并将运行期业务数据访问统一迁移到 GORM Gen。

**Architecture:** `kratos-admin-tools gorm-gen` 在配置指定的 MySQL 服务器上创建隔离临时库，复用 Goose 迁移内容建表，再将 Model 和 Query 生成到暂存目录并原子提升到正式目录。Repository 使用生成 DAO、字段表达式、`Preload`、`Join` 和 `query.Query.Transaction`；只有迁移、临时库管理和 MySQL 命名锁保留基础 SQL。

**Tech Stack:** Go 1.24、GORM v1.31、GORM Gen v0.3.28、Goose v3、MySQL 8、Cobra、标准库 `go/ast`/`go/parser`。

## Global Constraints

- Goose SQL 是唯一数据库结构真相源；禁止使用 `AutoMigrate`。
- Model 和 Query 均为生成文件，禁止手工修改 `*.gen.go`。
- 反向生成只能读取执行完仓库内全部迁移的临时数据库，不能直接读取业务数据库。
- 关联读取优先 `Preload`；关联过滤、聚合、排序或投影优先 `Join`/`LeftJoin`。
- 不给现有表新增数据库外键；关系通过生成配置显式声明。
- 业务 Repository 禁止 `.Table(`、`.Raw(`、`.Exec(` 和直接 `database/sql`。
- 允许基础 SQL 的文件仅限迁移、MySQL 命名锁和临时数据库生命周期实现。
- 不新增第三方依赖。
- 所有新增导出 Go 标识符必须有以名称开头的中文 GoDoc。
- 保留并不暂存用户现有的 `app/frontend/vite.config.ts` 修改。

---

### Task 1: 拆分可复用 Goose 迁移执行器

**Files:**
- Modify: `app/admin/internal/data/migration.go`
- Modify: `app/admin/internal/data/migration_test.go`

**Interfaces:**
- Produces: `func ApplyMigrations(ctx context.Context, db *sql.DB) error`
- Preserves: `func Migrate(ctx context.Context, dsn string) error`

- [ ] **Step 1: 写入失败测试，约束迁移内容可以脱离命名锁复用**

在 `migration_test.go` 增加一个最小接口测试，使用 `KRATOS_ADMIN_TEST_MYSQL_DSN` 时创建事务外连接并调用：

```go
func TestApplyMigrationsUsesProvidedDatabase(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: 运行测试确认 RED**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/internal/data -run TestApplyMigrationsUsesProvidedDatabase`

Expected: FAIL，提示 `undefined: ApplyMigrations`；未配置 DSN 时先增加纯单元测试，通过可注入的 `gooseUp` 函数验证调用路径。

- [ ] **Step 3: 实现可复用迁移函数并保持生产命名锁**

在 `migration.go` 中提取：

```go
// ApplyMigrations 在指定数据库中执行所有尚未应用的 Goose 迁移。
func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrations.Files)
	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("设置 Goose MySQL 方言失败: %w", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("执行 Goose 迁移失败: %w", err)
	}
	return nil
}
```

`Migrate` 继续获取 `GET_LOCK`，并在回调中调用 `ApplyMigrations(ctx, db)`；不得改变锁超时和释放逻辑。

- [ ] **Step 4: 运行迁移测试确认 GREEN**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/internal/data -run 'Test(ApplyMigrations|RunWithNamedLock|Migration)'`

Expected: PASS；无 DSN 时 MySQL 集成用例显示 SKIP，命名锁单元测试全部通过。

- [ ] **Step 5: 提交**

```bash
git add app/admin/internal/data/migration.go app/admin/internal/data/migration_test.go
git commit -m "后端：复用Goose迁移执行器"
```

### Task 2: 实现安全的临时数据库生命周期

**Files:**
- Create: `app/admin/cmd/tools/gormgen_database.go`
- Create: `app/admin/cmd/tools/gormgen_database_test.go`
- Modify: `app/admin/cmd/tools/root.go`
- Modify: `app/admin/cmd/tools/root_test.go`
- Modify: `app/admin/cmd/tools/run.go`

**Interfaces:**
- Consumes: `data.ApplyMigrations(context.Context, *sql.DB) error`
- Produces: `func withTemporaryDatabase(ctx context.Context, sourceDSN string, run func(string) error) error`
- Produces: `func withTemporaryDatabaseUsing(ctx context.Context, sourceDSN string, open temporaryDatabaseOpener, run func(string) error) error`
- Produces: `genOptions{ConfPath, ModelOutPath, QueryOutPath string}`

- [ ] **Step 1: 写临时数据库命名和清理失败测试**

覆盖：名称只含小写字母、数字和下划线；回调失败仍删除；回调错误与删除错误使用 `errors.Join` 同时返回；原 DSN 的业务库名不会作为临时库名。

```go
func TestTemporaryDatabaseNameIsSafe(t *testing.T) {
	name := temporaryDatabaseName(time.Unix(1721520000, 0), []byte{0x01, 0xab, 0xff})
	if !regexp.MustCompile(`^kratos_admin_gen_[a-z0-9_]+$`).MatchString(name) {
		t.Fatalf("临时数据库名不安全: %q", name)
	}
}
```

- [ ] **Step 2: 运行测试确认 RED**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/cmd/tools -run TestTemporaryDatabase`

Expected: FAIL，提示临时数据库函数未定义。

- [ ] **Step 3: 实现临时库管理器**

使用 `github.com/go-sql-driver/mysql.ParseDSN` 解析 DSN；管理员连接清空 `DBName` 后创建临时库；生成 DSN 只替换 `DBName`。数据库名只能来自内部生成函数并再次通过正则校验。定义只包含 `ExecContext` 和 `Close` 的 `temporaryDatabaseAdmin` 接口，以及返回该接口的 `temporaryDatabaseOpener`，让单元测试可以注入 fake。

核心控制流固定为：

```go
func withTemporaryDatabase(ctx context.Context, sourceDSN string, run func(string) error) error {
	return withTemporaryDatabaseUsing(ctx, sourceDSN, openSQLTemporaryDatabaseAdmin, run)
}

func withTemporaryDatabaseUsing(ctx context.Context, sourceDSN string, open temporaryDatabaseOpener, run func(string) error) error {
	serverDSN, databaseDSN, name, err := temporaryDatabaseDSNs(sourceDSN)
	if err != nil {
		return err
	}
	admin, err := open(serverDSN)
	if err != nil {
		return fmt.Errorf("打开代码生成数据库连接失败: %w", err)
	}
	defer admin.Close()
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		return fmt.Errorf("创建代码生成临时数据库失败: %w", err)
	}
	runErr := run(databaseDSN)
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	_, cleanupErr := admin.ExecContext(cleanupCtx, "DROP DATABASE `"+name+"`")
	if cleanupErr != nil {
		cleanupErr = fmt.Errorf("删除代码生成临时数据库 %s 失败: %w", name, cleanupErr)
	}
	return errors.Join(runErr, cleanupErr)
}
```

`openSQLTemporaryDatabaseAdmin` 是唯一调用 `sql.Open("mysql", dsn)` 的默认实现。单元测试调用 `withTemporaryDatabaseUsing` 注入 fake，不依赖真实数据库。

- [ ] **Step 4: 更新 Cobra 参数**

`gorm-gen` 改为读取配置而不是只接收 Query 输出目录：

```go
type genOptions struct {
	ConfPath     string
	ModelOutPath string
	QueryOutPath string
}
```

默认值分别为 `./configs/config.yaml`、`app/admin/internal/data/model`、`app/admin/internal/data/query`；CLI 参数为 `--conf`、`--model-out-path`、`--query-out-path`。

- [ ] **Step 5: 运行命令测试确认 GREEN**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/cmd/tools -run 'Test(GORMGenCommand|TemporaryDatabase)'`

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add app/admin/cmd/tools/gormgen_database.go app/admin/cmd/tools/gormgen_database_test.go app/admin/cmd/tools/root.go app/admin/cmd/tools/root_test.go app/admin/cmd/tools/run.go
git commit -m "工具：建立GORM生成临时数据库"
```

### Task 3: 建立确定性的 Model 与 Query 反向生成器

**Files:**
- Create: `app/admin/cmd/tools/gormgen.go`
- Create: `app/admin/cmd/tools/gormgen_test.go`
- Modify: `app/admin/cmd/tools/run.go`
- Modify: `Makefile`

**Interfaces:**
- Consumes: `withTemporaryDatabase`、`data.ApplyMigrations`
- Produces: `func generateGORMArtifacts(ctx context.Context, dsn string, options genOptions) error`
- Produces: `func modelNameForTable(string) string`
- Produces: `func modelOptionsForTable(string, map[string]*generate.QueryStructMeta) []gen.ModelOpt`

- [ ] **Step 1: 写命名、类型和排除规则失败测试**

表驱动测试至少覆盖：

```go
tests := []struct{ table, model string }{
	{table: "api_access_logs", model: "APIAccessLog"},
	{table: "auth_sessions", model: "AuthSession"},
	{table: "casbin_rules", model: "CasbinRule"},
	{table: "tenant_members", model: "TenantMember"},
}
```

同时断言 `goose_db_version` 不进入业务表列表，`is_platform_admin`、`mfa_enabled`、`is_tenant_admin`、`visible`、`is_secret`、`allow_tenant_override` 映射为 `bool`。

- [ ] **Step 2: 运行测试确认 RED**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/cmd/tools -run 'Test(ModelName|ModelType|BusinessTable)'`

Expected: FAIL，提示生成规则函数未定义。

- [ ] **Step 3: 实现生成配置**

生成器使用真实临时库连接：

```go
generator := gen.NewGenerator(gen.Config{
	OutPath:      stagingQueryPath,
	ModelPkgPath: stagingModelPath,
	Mode:         gen.WithDefaultQuery | gen.WithQueryInterface,
})
generator.UseDB(db)
```

按排序后的迁移业务表执行两阶段生成，而不是无条件 `GenerateAllTable`。第一阶段不带关系生成全部基础元数据，保证关系目标一定存在；第二阶段只重新生成声明了关系的源表，并用新元数据覆盖第一阶段结果。每个最终 `QueryStructMeta.ModelPkg` 固定改为 `github.com/sleep-go/kratos-admin/app/admin/internal/data/model`，保证暂存目录提升后 Query import 不漂移。

```go
meta := generator.GenerateModelAs(table, modelNameForTable(table), scalarModelOptionsForTable(table)...)
meta.ModelPkg = modelImportPath
metas[table] = meta

for _, table := range tablesWithRelations(metas) {
	meta = generator.GenerateModelAs(table, modelNameForTable(table), relationModelOptionsForTable(table, metas)...)
	meta.ModelPkg = modelImportPath
	metas[table] = meta
}

models = orderedModelMetas(tables, metas)
```

最后执行 `generator.ApplyBasic(models...)` 和 `generator.Execute()`。

- [ ] **Step 4: 添加显式关系元数据**

只添加已被 Repository 使用的关系：用户到成员、租户到成员、成员到用户/部门/岗位、角色到租户、文件到引用、日志导出到文件。使用 `gen.FieldRelate` 明确 `foreignKey` 和 `references`，不得新增迁移外键。

关系配置采用固定注册表，例如：

```go
type relationSpec struct {
	SourceTable string
	TargetTable string
	FieldName   string
	Type        field.RelationshipType
	ForeignKey  string
	References  string
}
```

- [ ] **Step 5: 实现暂存目录和生成清单提升**

生成前创建与目标同文件系统的暂存目录；生成完成后只提升 `*.gen.go`。清单文件 `.gormgen-manifest.json` 记录上次生成的文件名；提升时删除清单中已不再生成的文件，但保留 `extensions.go` 等手写文件。任何生成失败都不得改动目标目录。

- [ ] **Step 6: 添加 MySQL 集成生成测试**

使用 `KRATOS_ADMIN_TEST_MYSQL_DSN` 创建临时库、执行迁移并生成到 `t.TempDir()`；断言：

- 存在 `model/users.gen.go` 和 `query/users.gen.go`。
- 不存在 `model/goose_db_version.gen.go`。
- Query import 为正式 Model import path。
- 连续生成两次得到相同文件哈希。

- [ ] **Step 7: 运行生成器测试确认 GREEN**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/cmd/tools -run 'Test(GORM|Model|Generated)'`

Expected: PASS；未配置 MySQL DSN 时集成测试 SKIP，纯单元测试全部 PASS。

- [ ] **Step 8: 更新 Makefile 并提交**

`make gorm-gen` 显式传递 `--conf ./configs/config.yaml`；帮助文本改为“从 Goose 临时数据库反向生成 GORM Model 与 Query”。

```bash
git add app/admin/cmd/tools/gormgen.go app/admin/cmd/tools/gormgen_test.go app/admin/cmd/tools/run.go Makefile
git commit -m "工具：反向生成GORM模型与查询"
```

### Task 4: 用反向生成 Model 替换手写 Schema

**Files:**
- Delete: `app/admin/internal/data/model/schema.go`
- Modify: `app/admin/internal/data/model/schema_test.go`
- Generate: `app/admin/internal/data/model/*.gen.go`
- Generate: `app/admin/internal/data/query/*.gen.go`
- Create if required: `app/admin/internal/data/model/extensions.go`

**Interfaces:**
- Consumes: Task 3 的 `gorm-gen` 命令。
- Produces: 与现有业务编译契约兼容的生成 Model 类型。

- [ ] **Step 1: 将现有 Model 契约测试改为生成结果契约**

测试必须断言核心字段类型和表名：

```go
func TestGeneratedModelCoreTypes(t *testing.T) {
	var user model.User
	var tenant model.Tenant
	if reflect.TypeOf(user.ID).Kind() != reflect.Uint64 {
		t.Fatal("User.ID 必须为 uint64")
	}
	if reflect.TypeOf(user.IsPlatformAdmin).Kind() != reflect.Bool {
		t.Fatal("User.IsPlatformAdmin 必须为 bool")
	}
	if user.TableName() != "users" || tenant.TableName() != "tenants" {
		t.Fatal("生成 Model 表名不正确")
	}
}
```

- [ ] **Step 2: 运行现有 Model 测试建立基线**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/internal/data/model`

Expected: 当前手写 Model 下 PASS。

- [ ] **Step 3: 启动本地 MySQL 并执行反向生成**

Run: `docker compose up -d mysql`

Run: `GOCACHE=/tmp/go-build-kratos-gen go run ./app/admin/cmd/tools gorm-gen --conf ./configs/config.yaml`

Expected: Model 和 Query 均生成成功，临时数据库被删除。

- [ ] **Step 4: 删除手写 Schema 并补充非结构扩展**

删除 `schema.go`。如果生成器不能输出必要的 `TableName` 或纯业务方法，只在 `extensions.go` 中保留方法；禁止在扩展文件重新声明字段或结构体。

- [ ] **Step 5: 编译定位并修正类型映射，不修改生成文件**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/internal/data/model ./app/admin/internal/data/query ./app/admin/internal/data`

Expected: 首次可能因类型映射失败；只修改 Task 3 的命名/类型配置后重新生成，直到 PASS。

- [ ] **Step 6: 验证生成幂等并提交**

Run: `make gorm-gen && git diff --exit-code -- app/admin/internal/data/model app/admin/internal/data/query`

Expected: PASS，无差异。

```bash
git add app/admin/internal/data/model app/admin/internal/data/query app/admin/cmd/tools/gormgen.go app/admin/cmd/tools/gormgen_test.go
git commit -m "后端：切换为反向生成GORM模型"
```

### Task 5: 统一认证、初始化和验证码 Repository

**Files:**
- Modify: `app/admin/internal/data/data.go`
- Modify: `app/admin/internal/data/auth_repository.go`
- Modify: `app/admin/internal/data/setup_repository.go`
- Modify: `app/admin/internal/data/verification_repository.go`
- Modify: `app/admin/internal/data/auth_repository_integration_test.go`
- Modify: `app/admin/internal/data/verification_repository_integration_test.go`

**Interfaces:**
- Consumes: `*query.Query`、生成 Model 关系字段。
- Preserves: 所有现有 biz Repository 接口。

- [ ] **Step 1: 增加 SQL 记录器行为测试**

在 MySQL 集成测试中覆盖登录标识查询、成员身份、权限列表、会话轮换、验证码行锁和密码更新；测试断言保持现有业务结果，不断言内部 GORM 调用顺序。

- [ ] **Step 2: 运行定向测试确认当前行为基线**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/internal/data -run 'Test(Auth|Verification|Admin)'`

Expected: PASS 或在无 DSN 时 SKIP 集成测试。

- [ ] **Step 3: 将普通 CRUD 转为生成 DAO**

使用 `q := r.q.WithContext(ctx)`，例如用户更新固定写为：

```go
u := q.User
_, err := u.Where(u.ID.Eq(userID), u.DeletedAt.IsNull()).UpdateSimple(
	u.DisplayName.Value(displayName),
	u.AvatarURL.Value(nullableString(avatarURL)),
	u.Email.Value(nullableString(email)),
	u.Phone.Value(nullableString(phone)),
)
```

事务内使用：

```go
return r.q.Transaction(func(tx *query.Query) error {
	u := tx.User
	s := tx.AuthSession
	// 使用 tx 对应 DAO 完成原子更新。
	return nil
})
```

- [ ] **Step 4: 将关联读取转为 Preload 或 Join**

- 完整成员和租户资料使用显式关系 `Preload`。
- 权限列表和导航需要关联过滤、去重和投影，使用生成表的 `As`、`Join`、字段 `EqCol`、`Concat`/投影表达式。
- 验证码和会话的 `FOR UPDATE` 使用 DAO `.Clauses(clause.Locking{Strength: "UPDATE"})`。

- [ ] **Step 5: 验证不再出现业务原生表访问**

Run: `rg -n '\.Table\(|\.Raw\(|\.Exec\(' app/admin/internal/data/{auth_repository.go,setup_repository.go,verification_repository.go}`

Expected: 无输出。

- [ ] **Step 6: 运行测试并提交**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/internal/data -run 'Test(Auth|Verification|Admin)'`

Expected: PASS。

```bash
git add app/admin/internal/data/data.go app/admin/internal/data/auth_repository.go app/admin/internal/data/setup_repository.go app/admin/internal/data/verification_repository.go app/admin/internal/data/auth_repository_integration_test.go app/admin/internal/data/verification_repository_integration_test.go
git commit -m "后端：认证数据访问统一到GORM Gen"
```

### Task 6: 统一审计、文件、任务和日志 Repository

**Files:**
- Modify: `app/admin/internal/data/audit_repository.go`
- Modify: `app/admin/internal/data/file_repository.go`
- Modify: `app/admin/internal/data/task_repository.go`
- Modify: `app/admin/internal/data/log_export_repository.go`
- Modify: `app/admin/internal/data/log_maintenance.go`
- Create: `app/admin/internal/data/audit_repository_integration_test.go`
- Modify: `app/admin/internal/data/file_repository_integration_test.go`
- Modify: `app/admin/internal/data/task_repository_test.go`
- Modify: `app/admin/internal/data/task_repository_integration_test.go`
- Modify: `app/admin/internal/data/log_export_repository_integration_test.go`
- Modify: `app/admin/internal/data/log_maintenance_test.go`

**Interfaces:**
- Consumes: `*query.Query`。
- Preserves: audit、file、task、logexport 的 biz 接口和幂等语义。

- [ ] **Step 1: 补齐锁、Upsert 和清理行为测试**

覆盖：审计 Outbox 幂等发布、文件引用重复添加不报错、任务认领行锁、失败任务幂等键、日志导出认领、日志保留期批量清理。

- [ ] **Step 2: 运行测试建立基线**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/internal/data -run 'Test(Audit|File|Task|LogExport|LogMaintenance)'`

Expected: PASS 或集成测试 SKIP。

- [ ] **Step 3: 转换事务、锁和 Upsert**

统一模式：

```go
return r.q.Transaction(func(tx *query.Query) error {
	f := tx.File
	row, err := f.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(f.ID.Eq(fileID), f.TenantID.Eq(tenantID), f.DeletedAt.IsNull()).
		Take()
	if err != nil {
		return err
	}
	_, err = f.Where(f.ID.Eq(row.ID)).UpdateSimple(f.Status.Value(nextStatus))
	return err
})
```

Upsert 使用生成 DAO 的 `.Clauses(clause.OnConflict{...}).Create(model)`；原子计数使用字段表达式 `field.Add` 或 `UpdateColumnSimple`。

- [ ] **Step 4: 转换日志导出动态读取**

保留日志类型白名单，但将每种日志注册为明确的 DAO、字段表达式和结果扫描函数。禁止继续通过 `definition.table` 和 `strings.Join(columns, ",")` 构造查询。

- [ ] **Step 5: 转换日志维护批量删除**

分别通过 `q.LoginLog`、`q.AuditLog`、`q.APIAccessLog`、`q.LogExport` 的字段条件删除；删除数量累加 `gen.ResultInfo.RowsAffected`，不使用拼接 `DELETE FROM`。

- [ ] **Step 6: 验证和提交**

Run: `rg -n '\.Table\(|\.Raw\(|\.Exec\(' app/admin/internal/data/{audit_repository.go,file_repository.go,task_repository.go,log_export_repository.go,log_maintenance.go}`

Expected: 无输出。

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/internal/data -run 'Test(Audit|File|Task|LogExport|LogMaintenance)'`

Expected: PASS。

```bash
git add app/admin/internal/data/audit_repository.go app/admin/internal/data/audit_repository_integration_test.go app/admin/internal/data/file_repository.go app/admin/internal/data/file_repository_integration_test.go app/admin/internal/data/task_repository.go app/admin/internal/data/task_repository_test.go app/admin/internal/data/task_repository_integration_test.go app/admin/internal/data/log_export_repository.go app/admin/internal/data/log_export_repository_integration_test.go app/admin/internal/data/log_maintenance.go app/admin/internal/data/log_maintenance_test.go
git commit -m "后端：任务与日志数据访问统一到GORM Gen"
```

### Task 7: 建立通用管理 Gen 资源注册表

**Files:**
- Create: `app/admin/internal/data/management_gen_registry.go`
- Create: `app/admin/internal/data/management_gen_registry_test.go`
- Modify: `app/admin/internal/data/management_repository.go`

**Interfaces:**
- Produces: `type managementResourceAdapter struct { List, Create, Update, Delete and Validate operation closures }`
- Produces: `func managementAdapterFor(string) (managementResourceAdapter, bool)`

- [ ] **Step 1: 写注册表覆盖失败测试**

遍历 `managementResources`，要求每个资源都有 DAO 注册、全部 `columns`/`writeFields`/`filterFields` 都能解析为生成字段；未知资源返回 false。

```go
func TestManagementGenRegistryCoversResources(t *testing.T) {
	for resource, definition := range managementResources {
		registered, ok := managementAdapterFor(resource)
		if !ok {
			t.Fatalf("资源 %s 未注册 Gen 适配器", resource)
		}
		for _, column := range definition.columns {
			if !registered.SupportsField(column) {
				t.Fatalf("资源 %s 缺少字段 %s", resource, column)
			}
		}
	}
}
```

- [ ] **Step 2: 运行测试确认 RED**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/internal/data -run TestManagementGenRegistry`

Expected: FAIL，提示注册表函数未定义。

- [ ] **Step 3: 实现白名单注册表**

GORM Gen 的生成接口（如 `IUserDo`、`ITenantDo`）具有不同的链式返回类型，不能强制转换为统一 `gen.Dao`。每个资源改为注册高阶操作闭包，闭包内部使用对应的类型安全 DAO，例如：

```go
"users": {
	Fields: fieldSet("id", "username", "email", "phone", "display_name", "status", "deleted_at"),
	List: func(ctx context.Context, q *query.Query, request managementListRequest) ([]map[string]any, uint64, error) {
		u := q.User.WithContext(ctx)
		return listUsers(u, request)
	},
	Create: func(ctx context.Context, q *query.Query, values map[string]any) (uint64, error) {
		row, err := userFromManagementValues(values)
		if err != nil {
			return 0, err
		}
		if err := q.User.WithContext(ctx).Create(row); err != nil {
			return 0, err
		}
		return row.ID, nil
	},
},
```

各闭包内部的条件、排序和赋值字段必须来自对应生成 Query。外部字段字符串只能先通过适配器的固定白名单映射为闭包分支，不能传入 `field.NewField` 或原生 GORM。

- [ ] **Step 4: 运行注册表测试确认 GREEN**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/internal/data -run TestManagementGenRegistry`

Expected: PASS。

- [ ] **Step 5: 提交注册表**

```bash
git add app/admin/internal/data/management_gen_registry.go app/admin/internal/data/management_gen_registry_test.go
git commit -m "后端：建立通用管理Gen资源注册表"
```

### Task 8: 将通用管理 Repository 全部迁移到 Gen

**Files:**
- Modify: `app/admin/internal/data/management_repository.go`
- Modify: `app/admin/internal/data/management_repository_test.go`
- Modify: `app/admin/internal/data/management_gen_registry.go`

**Interfaces:**
- Consumes: Task 7 的 `managementDAOFor`。
- Preserves: 管理 API 的分页、过滤、租户隔离、数据范围、加密、审计和权限版本行为。

- [ ] **Step 1: 补齐通用管理端到端行为测试**

在现有 MySQL 集成测试中覆盖每类操作的代表资源：用户（全局软删除）、成员（租户隔离）、部门（路径更新）、角色授权（事务替换）、租户功能（事务替换）、设置（加密和版本）、Provider（加密配置）、资源（父子循环）、字典项（跨租户关联拒绝）。

- [ ] **Step 2: 运行测试建立基线**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/internal/data -run 'TestManagement'`

Expected: PASS 或 MySQL 集成测试 SKIP。

- [ ] **Step 3: 转换 List、Create、Update、Delete**

- `List` 从注册表取得对应资源的操作闭包；闭包使用具体生成 DAO 组合 `Where`、`Order`、`Offset`、`Limit`、`Find` 和 `Count`。
- `Create` 使用 `Dao.Create(model)`；自增 ID 从创建后的生成 Model 读取，不再执行 `SELECT LAST_INSERT_ID()`。
- `Update` 使用字段赋值表达式或经过白名单清洗的生成 Model；不得把未知 map key 交给 GORM。
- `Delete` 使用生成 DAO 的软删除或明确更新 `deleted_at`。

- [ ] **Step 4: 转换权限、数据范围和关联校验**

成员、角色、资源、部门和 Casbin 查询使用生成 DAO；完整对象读取优先 `Preload`，只需校验或投影时使用 `Join`、`Count`、`Pluck`。部门子树和资源父链使用类型安全字段递归读取。

- [ ] **Step 5: 转换授权与配置事务**

`UpdateRoleAuthorization`、`UpdateTenantFeatures`、设置/Provider 加密更新、权限版本递增和审计 Outbox 全部在 `r.q.Transaction` 内完成；每个写入使用事务参数 `tx` 的 DAO。

- [ ] **Step 6: 删除原生 GORM 查询辅助参数**

将接收 `*gorm.DB` 的业务辅助函数改成接收 `*query.Query` 或具体 DAO；保留 `gorm.Expr` 的地方改成 Gen 字段赋值表达式。`ManagementRepository` 最终只保存 `q *query.Query` 和业务 Codec，不保存 `db *gorm.DB`。

- [ ] **Step 7: 验证与提交**

Run: `rg -n '\.Table\(|\.Raw\(|\.Exec\(|gorm\.Expr' app/admin/internal/data/management_repository.go app/admin/internal/data/management_gen_registry.go`

Expected: 无输出。

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/internal/data -run 'TestManagement'`

Expected: PASS。

```bash
git add app/admin/internal/data/management_repository.go app/admin/internal/data/management_repository_test.go app/admin/internal/data/management_gen_registry.go
git commit -m "后端：通用管理数据访问统一到GORM Gen"
```

### Task 9: 增加业务 SQL 架构守卫

**Files:**
- Create: `app/admin/internal/architecture/gorm_gen_test.go`
- Modify: `app/admin/internal/architecture/dependency_test.go` only if helper reuse is needed

**Interfaces:**
- Produces: AST-based repository access policy test.

- [ ] **Step 1: 写入架构测试并观察 RED**

测试扫描 `../data/*.go`，跳过 `_test.go`、`migration.go` 和生成工具目录；拒绝业务文件导入 `database/sql`，并拒绝选择器调用 `Table`、`Raw`、`Exec`。错误必须包含文件和行号。

```go
var forbiddenCalls = map[string]struct{}{
	"Table": {},
	"Raw":   {},
	"Exec":  {},
}
```

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/internal/architecture -run TestRepositoriesUseGORMGen`

Expected: RED，并列出尚未迁移的准确调用；若前面任务已全部完成，先临时在测试夹具文件加入一个 `.Table` 调用验证 RED，再删除夹具。

- [ ] **Step 2: 完成 AST 守卫实现**

使用 `parser.ParseFile(..., parser.SkipObjectResolution)` 解析完整 AST，再分别检查 imports 和 `ast.CallExpr`；允许名单写成固定相对路径集合，禁止字符串前缀模糊放行。

- [ ] **Step 3: 运行架构测试确认 GREEN**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./app/admin/internal/architecture`

Expected: PASS。

- [ ] **Step 4: 提交**

```bash
git add app/admin/internal/architecture/gorm_gen_test.go app/admin/internal/architecture/dependency_test.go
git commit -m "测试：禁止业务仓储绕过GORM Gen"
```

### Task 10: 文档、幂等生成和完整验收

**Files:**
- Modify: `README.md`
- Modify: `docs/deployment.md`
- Modify: `Makefile`
- Modify: `docs/superpowers/specs/2026-07-21-gorm-gen-database-first-design.md` only if implementation revealed a confirmed deviation

**Interfaces:**
- Consumes: all previous tasks.
- Produces: reproducible developer and CI commands.

- [ ] **Step 1: 更新开发文档**

文档明确：MySQL 用户需要临时 `CREATE DATABASE`/`DROP DATABASE` 权限；`make gorm-gen` 不访问配置业务库；Model/Query 禁止手工修改；修改迁移后必须重新生成；复杂关联优先级为 `Preload`、`Join`、评审后的自定义 Gen 查询。

- [ ] **Step 2: 验证生成幂等**

Run: `make gorm-gen`

Run: `git diff --exit-code -- app/admin/internal/data/model app/admin/internal/data/query`

Expected: 两条命令退出码 0，第二条无输出。

- [ ] **Step 3: 运行后端完整测试与竞态检查**

Run: `GOCACHE=/tmp/go-build-kratos-gen go test ./api/... ./app/admin/... ./migrations`

Run: `GOCACHE=/tmp/go-build-kratos-gen go test -race ./app/admin/internal/data/...`

Expected: PASS；依赖真实 MySQL 的用例在配置 DSN 时 PASS，未配置时明确 SKIP。

- [ ] **Step 4: 运行静态检查**

Run: `GOCACHE=/tmp/go-build-kratos-gen go vet ./api/... ./app/admin/... ./migrations`

Run: `GOLANGCI_LINT_CACHE=/tmp/golangci-kratos-gen golangci-lint run ./app/admin/...`

Expected: PASS，无新增告警。

- [ ] **Step 5: 检查改动边界**

Run: `git status --short`

Expected: 只包含本计划相关后端、生成代码和文档；用户的 `app/frontend/vite.config.ts` 保持原有未提交状态且不在暂存区。

- [ ] **Step 6: 提交文档和最终收敛**

```bash
git add README.md docs/deployment.md Makefile docs/superpowers/specs/2026-07-21-gorm-gen-database-first-design.md
git commit -m "文档：说明GORM Gen数据库生成流程"
```

- [ ] **Step 7: 最终审计**

Run: `git log --oneline --decorate -12`

Run: `git diff --check HEAD~10..HEAD`

Expected: 每个任务有独立中文提交，diff 无空白错误；没有提交 `app/frontend/vite.config.ts`。
