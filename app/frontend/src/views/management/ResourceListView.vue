<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, shallowRef, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

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
  type ResourceOption
} from '@/features/management/resourceDefinitions'
import { buildLookupOptions, resolveFieldLookup } from '@/features/management/resourceFormOptions'
import type { ResourceRow } from '@/api/management'
import type { AdminV1LogExport } from '@/api/generated'

const props = defineProps<{ resourceKey: string }>()
const definition = computed(() => resourceDefinitions[props.resourceKey])
const tableFields = computed(() => definition.value.fields.filter((field) => field.table))
const formFields = computed(() =>
  definition.value.fields.filter(
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
)
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

async function load() {
  loading.value = true
  try {
    const response = await managementApi.listResources(definition.value.resource, {
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value,
      filters: activeFilters()
    })
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
  void load()
}

function activeFilters() {
  return Object.fromEntries(Object.entries(filters).filter(([, value]) => value !== ''))
}

function clearFilters() {
  for (const key of Object.keys(filters)) delete filters[key]
}

function query() {
  page.value = 1
  void load()
}

function openCreate() {
  editingID.value = ''
  for (const key of Object.keys(form)) delete form[key]
  for (const field of formFields.value) {
    if (field.default !== undefined) form[field.key] = field.default
    if (field.type === 'status') form[field.key] = 1
    if (field.type === 'boolean') form[field.key] = false
    if (field.type === 'number') form[field.key] = 0
  }
  drawerOpen.value = true
  void loadFormOptions()
}

function openEdit(row: ResourceRow) {
  editingID.value = String(row.id ?? '')
  for (const key of Object.keys(form)) delete form[key]
  for (const field of formFields.value) {
    if (field.key in row) form[field.key] = row[field.key]
  }
  drawerOpen.value = true
  void loadFormOptions()
}

async function loadFormOptions() {
  const fields = formFields.value
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
            await managementApi.listResources(resource, { page: 1, page_size: 200, sort: 'id:asc' })
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
  const missingField = formFields.value.find((field) => {
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
  for (const field of formFields.value) {
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
    await managementApi.updateResource(definition.value.resource, editingID.value, payload)
  else await managementApi.createResource(definition.value.resource, payload)
  drawerOpen.value = false
  ElMessage.success(editingID.value ? '更新成功' : '创建成功')
  await load()
}

async function remove(row: ResourceRow) {
  await ElMessageBox.confirm('删除后不可从管理界面恢复，确认继续？', '危险操作确认', {
    type: 'warning'
  })
  await managementApi.deleteResource(definition.value.resource, String(row.id))
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

function displayValue(value: unknown, field: ResourceField) {
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

watch(
  () => props.resourceKey,
  () => {
    page.value = 1
    keyword.value = ''
    clearFilters()
    drawerOpen.value = false
    fieldOptions.value = {}
    void load()
  }
)
onMounted(load)
onBeforeUnmount(() => {
  active = false
})
</script>

<template>
  <section v-if="definition" class="resource-page">
    <header class="resource-heading">
      <div>
        <p>MANAGEMENT</p>
        <h1>{{ definition.title }}</h1>
        <span>{{ definition.description }}</span>
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
          @click="openCreate"
        >
          新建
        </el-button>
      </div>
    </header>
    <div v-if="exportTask" class="export-status" role="status">
      <div>
        <strong>导出任务 {{ exportTask.id }}</strong>
        <span v-if="exportTask.status === 1">等待 Worker 处理</span>
        <span v-else-if="exportTask.status === 2">正在生成受保护文件</span>
        <span v-else-if="exportTask.status === 3">已完成，共 {{ exportTask.rowCount }} 条</span>
        <span v-else>失败：{{ exportTask.failureReason || '请稍后重试' }}</span>
      </div>
      <el-button v-if="exportTask.status === 3" type="danger" :icon="Download" @click="downloadExport">
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
      <el-table v-loading="loading" :data="items" stripe>
        <el-table-column prop="id" label="ID" min-width="82" />
        <el-table-column
          v-for="field in tableFields"
          :key="field.key"
          :prop="field.key"
          :label="field.label"
          min-width="125"
        >
          <template #default="scope">{{ displayValue(scope.row[field.key], field) }}</template>
        </el-table-column>
        <el-table-column v-if="!definition.readOnly" label="操作" fixed="right" width="138">
          <template #default="scope">
            <el-button
              v-permission="`${definition.resource}:update`"
              link
              :icon="Edit"
              @click="openEdit(scope.row)"
            >
              编辑
            </el-button><el-button
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
      <div v-if="!loading && items.length === 0" class="mobile-empty">暂无数据</div>
      <div class="mobile-cards">
        <article v-for="row in items" :key="String(row.id)">
          <strong>#{{ row.id }} ·
            {{
              row.name ?? row.display_name ?? row.summary ?? row.original_name ?? definition.title
            }}</strong>
          <dl>
            <template v-for="field in tableFields.slice(0, 5)" :key="field.key">
              <dt>{{ field.label }}</dt>
              <dd>{{ displayValue(row[field.key], field) }}</dd>
            </template>
          </dl>
          <div v-if="!definition.readOnly">
            <el-button v-permission="`${definition.resource}:update`" link :icon="Edit" @click="openEdit(row)">
              编辑
            </el-button><el-button
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
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        @change="load"
      />
    </div>
    <el-drawer
      v-model="drawerOpen"
      :title="editingID ? `编辑${definition.title}` : `新建${definition.title}`"
      direction="rtl"
      size="min(560px, 100%)"
    >
      <el-form label-position="top">
        <el-form-item
          v-for="field in formFields"
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
        <el-button @click="drawerOpen = false">取消</el-button><el-button type="danger" :icon="Check" @click="save">保存</el-button>
      </template>
    </el-drawer>
  </section>
</template>

<style scoped lang="scss">
.resource-page {
  display: grid;
  gap: 18px;
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
