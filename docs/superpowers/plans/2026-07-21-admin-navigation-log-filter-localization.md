# 后台二级导航、日志筛选与系统设置中文化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为所有顶部导航分组补齐权限感知的二级菜单，为三个日志页面增加结构化筛选，并将系统设置的已知标识转换为中文展示。

**Architecture:** 保持后端 Protobuf、数据库和权限模型不变，使用前端编译期注册表把后端已授权菜单组织成两层导航。通用资源定义声明筛选项和值标签，通用列表页负责查询与导出参数同步；系统设置使用集中式中文标签字典，面板与编辑弹窗共同复用。

**Tech Stack:** Vue 3、TypeScript、Vue Router、Pinia、Element Plus、SCSS、Vitest、Vue Test Utils、Playwright。

## Global Constraints

- 不修改 `/api/v1` 接口、Protobuf、数据库表或 Casbin 权限语义。
- 只显示后端已下发并通过编译期组件注册表校验的菜单。
- 桌面端使用顶部二级下拉菜单，移动端在导航抽屉中展开二级入口。
- 日志保持登录日志、操作审计、API 日志三个独立页面。
- 日志列表与异步导出使用同一组非空筛选条件。
- 系统设置只转换展示文本，不翻译接口字段名和提交值。
- 保留当前工作区中其他会话对 `ResourceListView.vue` 的抽屉改造以及对应测试，不覆盖、不回退。
- 不新增第三方依赖。

---

### Task 1: 构建权限感知的两级导航数据

**Files:**
- Create: `app/frontend/src/features/navigation/types.ts`
- Modify: `app/frontend/src/features/navigation/registry.ts`
- Modify: `app/frontend/src/features/navigation/registry.test.ts`

**Interfaces:**
- Consumes: `resolveNavigation(items: AdminV1NavigationItem[]): ResolvedNavigationItem[]` 的安全路由映射结果。
- Produces: `NavigationItem`、`NavigationChildItem` 和 `resolveTopNavigation(items): NavigationItem[]`，供顶部导航与移动端共同使用。

- [ ] **Step 1: 写失败测试，证明日志和其他分组需要生成授权子项**

在 `registry.test.ts` 增加测试，输入登录日志、操作审计、API 日志、导出记录及系统设置菜单，断言：

```ts
const top = resolveTopNavigation(resolveNavigation(items))
const logs = top.find((item) => item.label === '日志中心')

expect(logs?.children?.map((item) => item.to)).toEqual([
  '/logs/login',
  '/logs/audit',
  '/logs/api',
  '/logs/exports'
])
expect(top.find((item) => item.label === '系统设置')?.children).toHaveLength(1)
```

再增加一个权限收缩用例，只下发操作审计时断言日志中心只含 `/logs/audit`，未知组件不进入子菜单。

- [ ] **Step 2: 运行定向测试并确认 RED**

Run:

```bash
cd app/frontend && pnpm vitest run src/features/navigation/registry.test.ts
```

Expected: FAIL，原因是当前 `resolveTopNavigation` 只返回扁平分组入口，没有 `children`。

- [ ] **Step 3: 实现共享导航类型和两级分组**

在 `types.ts` 定义：

```ts
export interface NavigationChildItem {
  label: string
  to: string
}

export interface NavigationItem extends NavigationChildItem {
  children?: NavigationChildItem[]
}
```

在 `registry.ts` 中保留 `resolveNavigation` 的现有白名单校验；将每个顶级分组匹配到的已授权路由按 `sortOrder` 放入 `children`。顶级分组 `to` 取配置首选路由，若无权限则取第一个授权子项；没有任何子项的分组不返回。工作台保持无子项。

- [ ] **Step 4: 运行导航注册表测试并确认 GREEN**

Run:

```bash
cd app/frontend && pnpm vitest run src/features/navigation/registry.test.ts
```

Expected: PASS，恶意路由仍被过滤，日志中心包含四个授权子项。

- [ ] **Step 5: 提交导航数据层**

```bash
git add app/frontend/src/features/navigation/types.ts app/frontend/src/features/navigation/registry.ts app/frontend/src/features/navigation/registry.test.ts
git commit -m "前端：构建权限感知二级导航"
```

---

### Task 2: 渲染桌面二级菜单和移动端分组

**Files:**
- Create: `app/frontend/src/layouts/components/TopNavigation.test.ts`
- Modify: `app/frontend/src/layouts/components/TopNavigation.vue`
- Modify: `app/frontend/src/layouts/AppShell.vue`
- Modify: `app/frontend/src/layouts/AppShell.test.ts`

**Interfaces:**
- Consumes: Task 1 的 `NavigationItem[]`，每个顶级项可携带 `children`。
- Produces: 桌面端可键盘聚焦的二级菜单、子路由激活态和移动端展开子项。

- [ ] **Step 1: 写失败测试，覆盖桌面和移动端可见入口**

`TopNavigation.test.ts` 使用内存路由挂载组件，传入含四个日志子项的“日志中心”，断言：

```ts
expect(wrapper.get('[data-testid="navigation-group-日志中心"]').text()).toContain('登录日志')
expect(wrapper.get('[data-testid="navigation-group-日志中心"]').text()).toContain('API 日志')
expect(wrapper.findAll('.navigation-submenu a')).toHaveLength(4)
```

在 `AppShell.test.ts` 的 Pinia 初始菜单中补齐三类日志和导出记录，打开移动端抽屉后断言四个链接均可见，并断言平台、组织、权限、系统设置分组同样展示其已授权子项。

- [ ] **Step 2: 运行组件测试并确认 RED**

Run:

```bash
cd app/frontend && pnpm vitest run src/layouts/components/TopNavigation.test.ts src/layouts/AppShell.test.ts
```

Expected: FAIL，当前顶部组件只循环渲染一级 `RouterLink`，移动抽屉也只渲染一级入口。

- [ ] **Step 3: 实现桌面二级菜单**

`TopNavigation.vue` 从 `types.ts` 导入 `NavigationItem`。无 `children` 时继续渲染原有链接；有子项时渲染带按钮的 `.navigation-group` 和绝对定位 `.navigation-submenu`。使用 `:focus-within` 与 `:hover` 打开菜单，按钮设置 `aria-haspopup="menu"`，子链接设置 `role="menuitem"`。通过 `useRoute()` 判断任一子路由是否为当前路径，并给顶级按钮添加红色激活线。

- [ ] **Step 4: 实现移动端展开分组**

`AppShell.vue` 按顶级项循环：显示分组名称，并对 `children` 循环渲染缩进链接；没有子项时保持单链接。点击任一子项关闭抽屉。桌面和平板断点沿用当前 `1040px`。

- [ ] **Step 5: 运行组件测试并确认 GREEN**

Run:

```bash
cd app/frontend && pnpm vitest run src/features/navigation/registry.test.ts src/layouts/components/TopNavigation.test.ts src/layouts/AppShell.test.ts
```

Expected: PASS，桌面和移动端都能看到授权二级入口，当前日志子路由激活日志中心。

- [ ] **Step 6: 提交导航界面**

```bash
git add app/frontend/src/layouts/components/TopNavigation.vue app/frontend/src/layouts/components/TopNavigation.test.ts app/frontend/src/layouts/AppShell.vue app/frontend/src/layouts/AppShell.test.ts
git commit -m "前端：实现顶部二级菜单"
```

---

### Task 3: 为日志列表和导出增加结构化筛选

**Files:**
- Modify: `app/frontend/src/features/management/resourceDefinitions.ts`
- Modify: `app/frontend/src/features/management/resourceDefinitions.test.ts`
- Modify: `app/frontend/src/views/management/ResourceListView.vue`
- Modify: `app/frontend/src/views/management/ResourceListView.test.ts`

**Interfaces:**
- Consumes: 后端现有 `ListResourcesRequest.filters: Record<string, string>` 与 `CreateExportRequest.filters`。
- Produces: `ResourceFilter` 声明、日志页面筛选控件、表格中文值标签以及列表/导出一致的筛选参数。

- [ ] **Step 1: 写失败测试，定义日志筛选和值标签**

在 `resourceDefinitions.test.ts` 断言：

```ts
expect(resourceDefinitions['login-logs'].filters?.map((item) => item.key)).toEqual([
  'result',
  'user_id'
])
expect(resourceDefinitions['audit-logs'].filters?.map((item) => item.key)).toEqual([
  'action',
  'resource_type',
  'user_id',
  'member_id'
])
expect(resourceDefinitions['api-logs'].filters?.map((item) => item.key)).toEqual([
  'method',
  'status_code',
  'user_id'
])
expect(resourceDefinitions['login-logs'].fields.find((item) => item.key === 'result')?.valueLabels?.['1']).toBe('成功')
```

在现有 `ResourceListView.test.ts` 保留抽屉测试，并新增日志挂载辅助方法，断言选择登录结果后 `listResources` 收到：

```ts
expect(listResources).toHaveBeenLastCalledWith('login-logs', expect.objectContaining({
  filters: { result: '2' }
}))
```

同时断言 `createLogExport` 收到相同 `filters`，点击重置后筛选对象为空。

- [ ] **Step 2: 运行定向测试并确认 RED**

Run:

```bash
cd app/frontend && pnpm vitest run src/features/management/resourceDefinitions.test.ts src/views/management/ResourceListView.test.ts
```

Expected: FAIL，资源定义没有 `filters`/`valueLabels`，列表请求和导出未发送筛选对象。

- [ ] **Step 3: 增加声明式筛选定义**

在 `resourceDefinitions.ts` 增加：

```ts
export interface ResourceOption {
  label: string
  value: string
}

export interface ResourceFilter {
  key: string
  label: string
  type: 'text' | 'select'
  options?: ResourceOption[]
}
```

给 `ResourceField` 增加 `valueLabels?: Record<string, string>`，给 `ResourceDefinition` 增加 `filters?: ResourceFilter[]`。登录结果使用 `1成功、2失败、3锁定、4需要 MFA`；HTTP 方法使用 GET、POST、PUT、PATCH、DELETE；状态码提供 200、400、401、403、404、409、500；操作类型和资源类型使用文本输入，ID 使用文本输入。日志导出记录增加 `log_type` 与 `status` 下拉筛选及中文值标签。

- [ ] **Step 4: 在通用列表中接入筛选状态**

`ResourceListView.vue` 使用 `reactive<Record<string, string>>({})` 保存筛选值，并通过纯函数移除空值：

```ts
function activeFilters() {
  return Object.fromEntries(Object.entries(filters).filter(([, value]) => value !== ''))
}
```

`load()` 传递 `filters: activeFilters()`；`startExport()` 传递同一结果。查询与筛选变化时将 `page` 重置为 1；切换资源和点击重置时清空全部筛选。查询面板按定义渲染 `el-select` 或 `el-input`。`displayValue` 优先读取字段 `valueLabels[String(value)]`，再执行状态、布尔和原值兜底。表格与移动卡片都调用同一显示函数。

- [ ] **Step 5: 运行筛选测试并确认 GREEN**

Run:

```bash
cd app/frontend && pnpm vitest run src/features/management/resourceDefinitions.test.ts src/views/management/ResourceListView.test.ts
```

Expected: PASS，筛选查询、重置、中文标签和导出参数均符合定义，原有抽屉测试继续通过。

- [ ] **Step 6: 提交日志筛选**

```bash
git add app/frontend/src/features/management/resourceDefinitions.ts app/frontend/src/features/management/resourceDefinitions.test.ts app/frontend/src/views/management/ResourceListView.vue app/frontend/src/views/management/ResourceListView.test.ts
git commit -m "前端：增加日志分类筛选"
```

---

### Task 4: 统一系统设置中文展示字典

**Files:**
- Create: `app/frontend/src/features/settings/settingLabels.ts`
- Create: `app/frontend/src/features/settings/settingLabels.test.ts`
- Create: `app/frontend/src/features/settings/EffectiveSettingsPanel.test.ts`
- Modify: `app/frontend/src/features/settings/EffectiveSettingsPanel.vue`
- Modify: `app/frontend/src/features/settings/SettingEditorDialog.vue`

**Interfaces:**
- Consumes: 后端返回的 `category`、`setting_key`、`value_type`、`source` 原始值。
- Produces: `settingCategoryLabel`、`settingKeyLabel`、`settingValueTypeLabel`、`settingSourceLabel`，仅用于展示。

- [ ] **Step 1: 写失败测试，固定当前配置项的中文名称**

`settingLabels.test.ts` 覆盖当前九项代码默认配置：

```ts
expect(settingCategoryLabel('platform')).toBe('平台设置')
expect(settingCategoryLabel('security')).toBe('安全策略')
expect(settingCategoryLabel('log')).toBe('日志保留')
expect(settingCategoryLabel('file')).toBe('文件设置')
expect(settingKeyLabel('security', 'access_token_minutes')).toBe('访问令牌有效时间（分钟）')
expect(settingKeyLabel('log', 'audit_retention_days')).toBe('操作审计保留天数')
expect(settingValueTypeLabel('number')).toBe('数字')
expect(settingSourceLabel('tenant')).toBe('租户覆盖')
expect(settingKeyLabel('custom', 'new_key')).toBe('new_key')
```

`EffectiveSettingsPanel.test.ts` 模拟接口返回英文标识，断言页面出现“安全策略”“访问令牌有效时间（分钟）”“数字”“代码安全默认”，且不把 `access_token_minutes` 作为配置标题显示。

- [ ] **Step 2: 运行设置测试并确认 RED**

Run:

```bash
cd app/frontend && pnpm vitest run src/features/settings/settingLabels.test.ts src/features/settings/EffectiveSettingsPanel.test.ts
```

Expected: FAIL，中文标签函数尚不存在，页面仍直接显示英文分类、配置键和值类型。

- [ ] **Step 3: 实现集中中文标签字典**

在 `settingLabels.ts` 定义四个只读映射。配置键覆盖：`site_name`、`access_token_minutes`、`refresh_token_days`、`login_failure_limit`、`login_lock_minutes`、`audit_retention_days`、`login_retention_days`、`api_retention_days`、`max_upload_mb`。未知值返回原始字符串，防止新配置项无法展示。

- [ ] **Step 4: 设置面板与编辑弹窗复用中文标签**

`EffectiveSettingsPanel.vue` 的分组标题使用 `settingCategoryLabel`，配置标题使用 `settingKeyLabel`，页脚类型使用 `settingValueTypeLabel`，来源使用 `settingSourceLabel`。`SettingEditorDialog.vue` 的标题改为：

```ts
const title = computed(() =>
  `配置 ${settingKeyLabel(String(props.item?.category ?? ''), String(props.item?.setting_key ?? ''))}`
)
```

接口提交继续使用原始 `category`、`setting_key` 和 `value_type`，不改变数据契约。

- [ ] **Step 5: 运行设置测试并确认 GREEN**

Run:

```bash
cd app/frontend && pnpm vitest run src/features/settings/settingLabels.test.ts src/features/settings/EffectiveSettingsPanel.test.ts
```

Expected: PASS，当前所有代码默认配置均显示中文，未知键仍安全显示。

- [ ] **Step 6: 提交系统设置中文化**

```bash
git add app/frontend/src/features/settings/settingLabels.ts app/frontend/src/features/settings/settingLabels.test.ts app/frontend/src/features/settings/EffectiveSettingsPanel.vue app/frontend/src/features/settings/EffectiveSettingsPanel.test.ts app/frontend/src/features/settings/SettingEditorDialog.vue
git commit -m "前端：完善系统设置中文展示"
```

---

### Task 5: 集成验证和交互回归

**Files:**
- Modify: `app/frontend/e2e/admin.spec.ts`

**Interfaces:**
- Consumes: Task 1 至 Task 4 的最终前端行为。
- Produces: 覆盖二级菜单、日志筛选和系统设置中文展示的端到端回归测试。

- [ ] **Step 1: 写失败的端到端断言**

在已登录流程中增加：打开“日志中心”二级菜单，依次确认登录日志、操作审计、API 日志入口；进入登录日志选择“失败”并断言请求 URL 包含 `filters[result]=2`；进入系统设置断言“平台设置”“安全策略”“日志保留”“文件设置”可见。

- [ ] **Step 2: 启动本地依赖与应用并运行 E2E**

Run:

```bash
make compose-deps-up
make run-admin
cd app/frontend && pnpm dev
```

在另一个终端运行：

```bash
cd app/frontend && pnpm playwright test e2e/admin.spec.ts
```

Expected: PASS；若测试环境缺少已初始化管理员，先在项目根目录执行 `make init-admin` 后重跑。

- [ ] **Step 3: 执行前端定向测试、类型检查和 lint**

Run:

```bash
cd app/frontend && pnpm vitest run src/features/navigation/registry.test.ts src/layouts/components/TopNavigation.test.ts src/layouts/AppShell.test.ts src/features/management/resourceDefinitions.test.ts src/views/management/ResourceListView.test.ts src/features/settings/settingLabels.test.ts src/features/settings/EffectiveSettingsPanel.test.ts
cd app/frontend && pnpm typecheck
cd app/frontend && pnpm lint
```

Expected: 所有命令退出码为 0，无 TypeScript 或 ESLint 错误。

- [ ] **Step 4: 提交端到端测试**

```bash
git add app/frontend/e2e/admin.spec.ts
git commit -m "测试：覆盖后台二级菜单与日志筛选"
```

- [ ] **Step 5: 检查最终改动边界**

Run:

```bash
git status --short
git log -5 --oneline
```

Expected: 本计划文件均已提交；其他会话的无关改动保持原状且未被误提交。
