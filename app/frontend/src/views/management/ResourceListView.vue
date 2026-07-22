<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, shallowRef, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'

import * as managementApi from '@/api/management'
import * as logApi from '@/api/logs'
import ResourceFormField from '@/features/management/ResourceFormField.vue'
import {
  Check,
  Delete,
  Download,
  Edit,
  Plus,
  RefreshLeft,
  Search
} from '@/components/icons/actions'
import {
  resourceDefinitions,
  type ResourceField,
  type ResourceOption,
  type ResourceScopeTab
} from '@/features/management/resourceDefinitions'
import { buildLookupOptions, resolveFieldLookup } from '@/features/management/resourceFormOptions'
import {
  buildResourceTree,
  canAddResourceChild,
  filterResourceRows,
  filterResourceRowsByScopeSide,
  flattenResourceTree,
  type ResourceTreeRow
} from '@/features/management/resourceTree'
import type { ResourceRow } from '@/api/management'
import type { AdminV1LogExport } from '@/api/generated'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{ resourceKey: string; targetTenantId?: string; embedded?: boolean }>()
const router = useRouter()
const authStore = useAuthStore()
const definition = computed(() => resourceDefinitions[props.resourceKey])
const scopeTabs = computed(() => definition.value?.scopeTabs ?? [])
const activeScopeTab = ref('')
const activeScopeTabDefinition = computed<ResourceScopeTab | undefined>(() =>
  scopeTabs.value.find((tab) => tab.key === activeScopeTab.value)
)
const pageDescription = computed(
  () => activeScopeTabDefinition.value?.description ?? definition.value?.description ?? ''
)
const isTreeList = computed(() => definition.value?.listMode === 'tree')
const treePageSize = 500
const tableFields = computed(() => definition.value?.fields.filter((field) => field.table) ?? [])
const treeTableFields = computed(() => {
  const fields = tableFields.value
  const nameField = fields.find((field) => field.key === 'name')
  const rest = fields.filter((field) => field.key !== 'name')
  return nameField ? [nameField, ...rest] : fields
})
const visibleItems = computed(() => {
  if (!isTreeList.value) return items.value
  const scopeSide = activeScopeTabDefinition.value?.scopeSide
  const scoped = filterResourceRowsByScopeSide(
    items.value,
    scopeSide === 'platform' || scopeSide === 'tenant' ? scopeSide : undefined
  )
  return filterResourceRows(scoped, keyword.value)
})
const treeItems = computed(() => (isTreeList.value ? buildResourceTree(visibleItems.value) : []))
const flattenedTreeItems = computed(() => flattenResourceTree(treeItems.value))
const tableRows = computed(() => (isTreeList.value ? treeItems.value : items.value))
const formFields = computed(() => {
  if (!definition.value) return []
  return definition.value.fields.filter(
    (field) =>
      field.form !== false &&
      ![
        'created_at',
        'updated_at',
        'permission_version',
        'version',
        'created_by',
        'is_builtin'
      ].includes(field.key) &&
      !(editingID.value && field.createOnly)
  )
})
const resolvedFormFields = computed(() => {
  const tab = activeScopeTabDefinition.value
  if (!tab) return formFields.value
  return formFields.value.map((field) =>
    field.key === 'scope_mask'
      ? { ...field, options: tab.scopeMaskOptions, default: tab.defaultScopeMask }
      : field
  )
})
const loading = ref(false)
const items = ref<ResourceRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const filters = reactive<Record<string, string>>({})
const drawerOpen = ref(false)
const editingID = ref('')
const form = reactive<ResourceRow>({})
const fieldOptions = shallowRef<Record<string, ResourceOption[]>>({})
const optionsLoading = shallowRef(false)
const exportTask = ref<AdminV1LogExport>()
const exporting = ref(false)
let active = true

async function enterTenant(tenantId: string) {
  await authStore.impersonateTenant(tenantId)
  ElMessage.success('已进入租户代维视角')
  await router.push('/console')
}

function scopeOptions(resource: string) {
  if (!props.targetTenantId || ['app-users', 'tenants', 'resources'].includes(resource))
    return undefined
  return { targetTenantId: props.targetTenantId }
}

function listScopeFilters(resource: string) {
  if (resource !== 'resources' || !activeScopeTabDefinition.value || isTreeList.value) return {}
  if (activeScopeTabDefinition.value.scopeSide === 'all') return {}
  return { scope_side: activeScopeTabDefinition.value.scopeSide }
}

async function load() {
  if (!definition.value) return
  loading.value = true
  try {
    const response = await managementApi.listResources(
      definition.value.resource,
      isTreeList.value
        ? {
            page: 1,
            page_size: treePageSize,
            sort: 'sort_order:asc',
            filters: activeFilters()
          }
        : {
            page: page.value,
            page_size: pageSize.value,
            keyword: keyword.value,
            filters: activeFilters()
          },
      scopeOptions(definition.value.resource)
    )
    items.value = response.items ?? []
    total.value = Number(response.total ?? 0)
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  keyword.value = ''
  clearFilters()
  page.value = 1
  if (!isTreeList.value) void load()
}

function activeFilters() {
  return {
    ...listScopeFilters(definition.value?.resource ?? ''),
    ...Object.fromEntries(Object.entries(filters).filter(([, value]) => value !== ''))
  }
}

function clearFilters() {
  for (const key of Object.keys(filters)) delete filters[key]
}

function query() {
  page.value = 1
  if (!isTreeList.value) void load()
}

function openCreate(parent?: ResourceRow) {
  editingID.value = ''
  for (const key of Object.keys(form)) delete form[key]
  for (const field of resolvedFormFields.value) {
    if (field.default !== undefined) form[field.key] = field.default
    if (field.type === 'status') form[field.key] = 1
    if (field.type === 'boolean') form[field.key] = false
    if (field.type === 'number') form[field.key] = 0
  }
  if (parent?.id) form.parent_id = parent.id
  drawerOpen.value = true
  void loadFormOptions()
}

function openEdit(row: ResourceRow) {
  editingID.value = String(row.id ?? '')
  for (const key of Object.keys(form)) delete form[key]
  for (const field of resolvedFormFields.value) {
    if (field.key in row) form[field.key] = row[field.key]
  }
  // 平台管理员编辑时回填已绑定的平台角色 ID（Casbin g: v1=admin_id, v2=role_id）。
  if (definition.value?.resource === 'platform-admins' && row.id) {
    form.role_ids = []
    void loadPlatformAdminRoleIds(String(row.id))
  }
  drawerOpen.value = true
  void loadFormOptions()
}

async function loadPlatformAdminRoleIds(adminId: string) {
  try {
    const response = await managementApi.listResources('platform-casbin-rules', {
      page: 1,
      page_size: 200,
      filters: { ptype: 'g', v1: adminId }
    })
    form.role_ids = (response.items ?? [])
      .filter((item) => item.ptype === 'g' && String(item.v1) === adminId)
      .map((item) => Number(item.v2))
      .filter((id) => Number.isFinite(id) && id > 0)
  } catch {
    form.role_ids = []
  }
}

function rowIndent(row: ResourceRow) {
  if (!isTreeList.value) return undefined
  const depth = (row as ResourceTreeRow & { depth?: number }).depth ?? 0
  return { marginLeft: `${depth * 16}px` }
}

async function loadFormOptions() {
  const fields = resolvedFormFields.value
    .map((field) => ({ field, lookup: resolveFieldLookup(field, form) }))
    .filter(
      (entry): entry is { field: ResourceField; lookup: NonNullable<ResourceField['lookup']> } =>
        Boolean(entry.lookup)
    )
  if (fields.length === 0) {
    fieldOptions.value = {}
    return
  }
  optionsLoading.value = true
  try {
    const resources = [
      ...new Set(
        fields.flatMap((entry) => [
          entry.lookup.resource,
          ...(entry.lookup.exclude ? [entry.lookup.exclude.resource] : [])
        ])
      )
    ]
    const responses = await Promise.all(
      resources.map(
        async (resource) =>
          [
            resource,
            await managementApi.listResources(
              resource,
              {
                page: 1,
                page_size: resource === 'resources' ? treePageSize : 200,
                sort: resource === 'resources' ? 'sort_order:asc' : 'id:asc',
                filters: listScopeFilters(resource)
              },
              scopeOptions(resource)
            )
          ] as const
      )
    )
    const rowsByResource = new Map(
      responses.map(([resource, response]) => [resource, response.items ?? []])
    )
    fieldOptions.value = Object.fromEntries(
      fields.map(({ field, lookup }) => {
        const excludedValues = new Set(
          lookup.exclude
            ? (rowsByResource.get(lookup.exclude.resource) ?? [])
                .filter((row) => String(row.id ?? '') !== editingID.value)
                .map((row) => String(row[lookup.exclude!.valueKey] ?? ''))
            : []
        )
        return [
          field.key,
          buildLookupOptions(
            rowsByResource.get(lookup.resource) ?? [],
            lookup,
            editingID.value,
            excludedValues
          )
        ]
      })
    )
  } catch (error) {
    fieldOptions.value = {}
    ElMessage.error(error instanceof Error ? error.message : '表单候选项加载失败')
  } finally {
    optionsLoading.value = false
  }
}

async function save() {
  const missingField = resolvedFormFields.value.find((field) => {
    if (!field.required) return false
    const value = form[field.key]
    return (
      value === undefined || value === null || (typeof value === 'string' && value.trim() === '')
    )
  })
  if (missingField) {
    ElMessage.warning(`请填写或选择${missingField.label}`)
    return
  }

  const payload: ResourceRow = {}
  for (const field of resolvedFormFields.value) {
    if (
      field.key in form &&
      ![
        'created_at',
        'updated_at',
        'permission_version',
        'version',
        'created_by',
        'is_builtin'
      ].includes(field.key)
    ) {
      payload[field.key] = form[field.key]
    }
  }
  if (editingID.value)
    await managementApi.updateResource(
      definition.value.resource,
      editingID.value,
      payload,
      scopeOptions(definition.value.resource)
    )
  else
    await managementApi.createResource(
      definition.value.resource,
      payload,
      scopeOptions(definition.value.resource)
    )
  drawerOpen.value = false
  ElMessage.success(editingID.value ? '更新成功' : '创建成功')
  await load()
}

async function remove(row: ResourceRow) {
  await ElMessageBox.confirm('删除后不可从管理界面恢复，确认继续？', '危险操作确认', {
    type: 'warning'
  })
  await managementApi.deleteResource(
    definition.value.resource,
    String(row.id),
    scopeOptions(definition.value.resource)
  )
  ElMessage.success('删除成功')
  await load()
}

async function startExport() {
  if (!definition.value.exportLogType || exporting.value) return
  exporting.value = true
  try {
    exportTask.value = await logApi.createLogExport({
      logType: definition.value.exportLogType,
      keyword: keyword.value,
      filters: activeFilters()
    })
    ElMessage.success('导出任务已创建，后台处理中')
    for (
      let attempt = 0;
      attempt < 120 && active && (exportTask.value?.status ?? 0) < 3;
      attempt++
    ) {
      await new Promise((resolve) => globalThis.setTimeout(resolve, 1000))
      if (!active || !exportTask.value?.id) break
      exportTask.value = await logApi.getLogExport(exportTask.value.id)
    }
    if (exportTask.value?.status === 3) ElMessage.success('日志导出完成')
    if (exportTask.value?.status === 4)
      ElMessage.error(exportTask.value.failureReason || '日志导出失败')
  } finally {
    exporting.value = false
  }
}

async function downloadExport() {
  if (!exportTask.value?.id) return
  const result = await logApi.getLogExportDownloadURL(exportTask.value.id)
  if (!result.url) return
  const link = globalThis.document.createElement('a')
  link.href = result.url
  link.download = ''
  globalThis.document.body.appendChild(link)
  link.click()
  link.remove()
}

async function downloadExportRow(row: ResourceRow) {
  if (Number(row.status) !== 3 || !row.id) return
  const result = await logApi.getLogExportDownloadURL(String(row.id))
  if (!result.url) return
  const link = globalThis.document.createElement('a')
  link.href = result.url
  link.download = ''
  globalThis.document.body.appendChild(link)
  link.click()
  link.remove()
}

function displayValue(value: unknown, field: ResourceField, row: ResourceRow) {
  const formatted = field.format?.(value, row)
  if (formatted !== undefined) return formatted
  const label = field.valueLabels?.[String(value)]
  if (label) return label
  if (field.options) {
    return (
      field.options.find((option) => String(option.value) === String(value))?.label ?? value ?? '—'
    )
  }
  if (field.type === 'status') return Number(value) === 1 ? '启用' : '禁用'
  if (field.type === 'boolean') return value === true || value === 1 || value === '1' ? '是' : '否'
  return value ?? '—'
}

watch(
  () => form.ptype,
  (value, previous) => {
    if (!drawerOpen.value || value === previous) return
    delete form.v1
    delete form.v2
    void loadFormOptions()
  },
  { flush: 'sync' }
)

function onScopeTabChange() {
  drawerOpen.value = false
  if (isTreeList.value) return
  page.value = 1
  void load()
}

watch(
  scopeTabs,
  (tabs) => {
    if (tabs.length === 0) {
      activeScopeTab.value = ''
      return
    }
    if (!tabs.some((tab) => tab.key === activeScopeTab.value)) {
      activeScopeTab.value = tabs[0]?.key ?? ''
    }
  },
  { immediate: true }
)

watch(
  () => [props.resourceKey, props.targetTenantId],
  () => {
    page.value = 1
    keyword.value = ''
    clearFilters()
    drawerOpen.value = false
    fieldOptions.value = {}
    if (scopeTabs.value.length > 0) {
      activeScopeTab.value = scopeTabs.value[0]?.key ?? ''
    }
    void load()
  }
)
onMounted(load)
onBeforeUnmount(() => {
  active = false
})
</script>

<template>
  <section v-if="definition" class="resource-page" :class="{ 'resource-page--embedded': embedded }">
    <header v-if="!embedded" class="resource-heading">
      <div>
        <p>MANAGEMENT</p>
        <h1>{{ definition.title }}</h1>
        <span>{{ pageDescription }}</span>
      </div>
      <div class="heading-actions">
        <el-button
          v-if="definition.exportLogType"
          v-permission="`${definition.resource}:export`"
          data-testid="export-button"
          :icon="Download"
          :loading="exporting"
          @click="startExport"
        >
          异步导出
        </el-button>
        <router-link v-if="definition.exportLogType" to="/logs/exports">
          <el-button :icon="Download">导出记录</el-button>
        </router-link>
        <el-button
          v-if="!definition.readOnly"
          v-permission="`${definition.resource}:create`"
          type="danger"
          :icon="Plus"
          @click="openCreate()"
        >
          新建
        </el-button>
      </div>
    </header>
    <div
      v-if="embedded && !definition.readOnly"
      class="embedded-toolbar"
    >
      <span>{{ pageDescription }}</span>
      <el-button
        v-permission="`${definition.resource}:create`"
        type="danger"
        :icon="Plus"
        @click="openCreate()"
      >
        新建
      </el-button>
    </div>
    <el-tabs
      v-if="scopeTabs.length"
      v-model="activeScopeTab"
      class="scope-tabs"
      data-testid="resource-scope-tabs"
      @tab-change="onScopeTabChange"
    >
      <el-tab-pane
        v-for="tab in scopeTabs"
        :key="tab.key"
        :label="tab.label"
        :name="tab.key"
      />
    </el-tabs>
    <div v-if="exportTask" class="export-status" role="status">
      <div>
        <strong>导出任务 {{ exportTask.id }}</strong>
        <span v-if="exportTask.status === 1">等待 Worker 处理</span>
        <span v-else-if="exportTask.status === 2">正在生成受保护文件</span>
        <span v-else-if="exportTask.status === 3">已完成，共 {{ exportTask.rowCount }} 条</span>
        <span v-else>失败：{{ exportTask.failureReason || '请稍后重试' }}</span>
      </div>
      <el-button
        v-if="exportTask.status === 3"
        type="danger"
        :icon="Download"
        @click="downloadExport"
      >
        下载文件
      </el-button>
    </div>
    <div class="query-panel">
      <el-input v-model="keyword" clearable placeholder="输入关键词搜索" @keyup.enter="query" />
      <template v-for="filter in definition.filters ?? []" :key="filter.key">
        <el-select
          v-if="filter.type === 'select'"
          v-model="filters[filter.key]"
          clearable
          :placeholder="filter.label"
          :data-testid="`filter-${filter.key}`"
        >
          <el-option
            v-for="option in filter.options ?? []"
            :key="option.value"
            :label="option.label"
            :value="option.value"
          />
        </el-select>
        <el-input
          v-else
          v-model="filters[filter.key]"
          clearable
          :placeholder="filter.label"
          :data-testid="`filter-${filter.key}`"
          @keyup.enter="query"
        />
      </template>
      <el-button data-testid="query-button" :icon="Search" @click="query">查询</el-button>
      <el-button data-testid="reset-button" :icon="RefreshLeft" @click="resetQuery">重置</el-button>
    </div>
    <div class="table-panel">
      <el-table
        v-loading="loading"
        :data="tableRows"
        :row-key="isTreeList ? 'id' : undefined"
        :tree-props="isTreeList ? { children: 'children' } : undefined"
        :default-expand-all="isTreeList"
        stripe
      >
        <el-table-column v-if="!isTreeList" prop="id" label="ID" min-width="82" />
        <el-table-column
          v-for="field in isTreeList ? treeTableFields : tableFields"
          :key="field.key"
          :prop="field.key"
          :label="field.label"
          :min-width="field.key === 'name' && isTreeList ? 220 : 125"
        >
          <template #default="scope">{{
            displayValue(scope.row[field.key], field, scope.row)
          }}</template>
        </el-table-column>
        <el-table-column
          v-if="isTreeList"
          prop="id"
          label="ID"
          width="82"
        />
        <el-table-column v-if="!definition.readOnly" label="操作" fixed="right" :width="isTreeList ? 360 : 280">
          <template #default="scope">
            <router-link
              v-if="definition.resource === 'tenants'"
              :to="`/platform/tenants/${scope.row.id}/setup`"
            >
              <el-button link>开通配置</el-button>
            </router-link>
            <el-button
              v-if="definition.resource === 'tenants'"
              link
              data-testid="impersonate-tenant"
              @click="enterTenant(String(scope.row.id))"
            >
              进入租户
            </el-button>
            <el-button
              v-if="isTreeList && canAddResourceChild(scope.row)"
              v-permission="`${definition.resource}:create`"
              link
              data-testid="add-child-resource"
              :icon="Plus"
              @click="openCreate(scope.row)"
            >
              添加子级
            </el-button>
            <el-button
              v-permission="`${definition.resource}:update`"
              link
              :icon="Edit"
              @click="openEdit(scope.row)"
            >
              编辑 </el-button
            ><el-button
              v-permission="`${definition.resource}:delete`"
              link
              type="danger"
              :icon="Delete"
              @click="remove(scope.row)"
            >
              删除
            </el-button>
          </template>
        </el-table-column>
        <el-table-column
          v-else-if="definition.resource === 'log-exports'"
          label="操作"
          fixed="right"
          width="100"
        >
          <template #default="scope">
            <el-button
              :disabled="Number(scope.row.status) !== 3"
              link
              type="danger"
              :icon="Download"
              @click="downloadExportRow(scope.row)"
            >
              下载
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!loading && tableRows.length === 0" class="mobile-empty">暂无数据</div>
      <div class="mobile-cards">
        <article
          v-for="row in isTreeList ? flattenedTreeItems : items"
          :key="String(row.id)"
          :style="rowIndent(row)"
        >
          <strong
            >#{{ row.id }} ·
            {{
              row.name ?? row.display_name ?? row.summary ?? row.original_name ?? definition.title
            }}</strong
          >
          <dl>
            <template v-for="field in tableFields.slice(0, 5)" :key="field.key">
              <dt>{{ field.label }}</dt>
              <dd>{{ displayValue(row[field.key], field, row) }}</dd>
            </template>
          </dl>
          <div v-if="!definition.readOnly">
            <router-link
              v-if="definition.resource === 'tenants'"
              :to="`/platform/tenants/${row.id}/setup`"
              :data-testid="`tenant-setup-${row.id}`"
            >
              <el-button link>开通配置</el-button>
            </router-link>
            <el-button
              v-if="definition.resource === 'tenants'"
              link
              data-testid="impersonate-tenant"
              @click="enterTenant(String(row.id))"
            >
              进入租户
            </el-button>
            <el-button
              v-permission="`${definition.resource}:update`"
              link
              :icon="Edit"
              @click="openEdit(row)"
            >
              编辑 </el-button
            ><el-button
              v-permission="`${definition.resource}:delete`"
              link
              type="danger"
              :icon="Delete"
              @click="remove(row)"
            >
              删除
            </el-button>
          </div>
          <el-button
            v-else-if="definition.resource === 'log-exports'"
            :disabled="Number(row.status) !== 3"
            link
            type="danger"
            :icon="Download"
            @click="downloadExportRow(row)"
          >
            下载文件
          </el-button>
        </article>
      </div>
      <el-pagination
        v-if="!isTreeList"
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        @change="load"
      />
      <p v-else-if="!loading" class="tree-summary">共 {{ visibleItems.length }} 项资源</p>
    </div>
    <el-drawer
      v-model="drawerOpen"
      :title="editingID ? `编辑${definition.title}` : `新建${definition.title}`"
      direction="rtl"
      size="min(560px, 100%)"
    >
      <el-form label-position="top">
        <el-form-item
          v-for="field in resolvedFormFields"
          :key="field.key"
          :label="field.label"
          :required="field.required"
        >
          <ResourceFormField
            v-model="form[field.key]"
            :field="field"
            :options="fieldOptions[field.key]"
            :loading="optionsLoading"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="drawerOpen = false">取消</el-button
        ><el-button type="danger" :icon="Check" @click="save">保存</el-button>
      </template>
    </el-drawer>
  </section>
  <section v-else class="resource-page resource-page--missing">
    <p>未找到资源定义：{{ resourceKey }}</p>
  </section>
</template>

<style scoped lang="scss">
.resource-page {
  display: grid;
  gap: 18px;
}
.resource-page--embedded {
  gap: 12px;
}
.embedded-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 4px;
  color: var(--ka-muted);
  font-size: 13px;
}
.resource-page--missing p {
  margin: 0;
  color: var(--ka-muted);
}
.resource-heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
}
.resource-heading p {
  margin: 0 0 5px;
  color: var(--ka-accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.18em;
}
.resource-heading h1 {
  margin: 0;
  font-size: clamp(26px, 4vw, 38px);
  letter-spacing: -0.04em;
}
.resource-heading span {
  display: block;
  margin-top: 7px;
  color: var(--ka-muted);
  font-size: 13px;
}
.heading-actions {
  display: flex;
  gap: 10px;
}
.scope-tabs {
  margin-top: -6px;
}
.scope-tabs :deep(.el-tabs__header) {
  margin: 0;
}
.export-status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border-left: 3px solid var(--ka-accent);
  background: #fff;
  box-shadow: var(--ka-shadow);
}
.export-status div {
  display: grid;
  gap: 4px;
}
.export-status strong {
  overflow-wrap: anywhere;
  font-size: 13px;
}
.export-status span {
  color: var(--ka-muted);
  font-size: 12px;
}
.query-panel,
.table-panel {
  border: 1px solid rgb(0 0 0 / 5%);
  border-radius: 5px;
  background: #fff;
  box-shadow: var(--ka-shadow);
}
.query-panel {
  display: flex;
  gap: 10px;
  padding: 18px;
}
.query-panel .el-input {
  width: min(360px, 100%);
}
.table-panel {
  padding: 18px;
  overflow: hidden;
}
.el-pagination {
  justify-content: flex-end;
  margin-top: 18px;
}
.tree-summary {
  margin: 18px 0 0;
  color: var(--ka-muted);
  font-size: 12px;
  text-align: right;
}
.mobile-cards,
.mobile-empty {
  display: none;
}
@media (max-width: 640px) {
  .resource-heading,
  .export-status {
    align-items: stretch;
    flex-direction: column;
  }
  .query-panel {
    flex-wrap: wrap;
  }
  .query-panel .el-input {
    width: 100%;
  }
  .table-panel :deep(.el-table) {
    display: none;
  }
  .mobile-cards {
    display: grid;
    gap: 10px;
  }
  .mobile-cards article {
    padding: 16px;
    border: 1px solid #e8e8e8;
    border-left: 3px solid var(--ka-accent);
    border-radius: 3px;
  }
  dl {
    display: grid;
    grid-template-columns: 90px 1fr;
    gap: 7px;
    margin: 14px 0;
    font-size: 12px;
  }
  dt {
    color: var(--ka-muted);
  }
  dd {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .mobile-empty {
    display: block;
    padding: 50px 0;
    color: var(--ka-muted);
    text-align: center;
  }
  .el-pagination {
    justify-content: center;
  }
}
</style>
