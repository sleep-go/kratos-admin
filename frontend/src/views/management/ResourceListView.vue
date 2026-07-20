<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import * as managementApi from '@/api/management'
import { resourceDefinitions } from '@/features/management/resourceDefinitions'
import type { ResourceRow } from '@/api/management'

const props = defineProps<{ resourceKey: string }>()
const definition = computed(() => resourceDefinitions[props.resourceKey])
const tableFields = computed(() => definition.value.fields.filter((field) => field.table))
const loading = ref(false)
const items = ref<ResourceRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const dialogOpen = ref(false)
const editingID = ref('')
const form = reactive<ResourceRow>({})

async function load() {
  loading.value = true
  try {
    const response = await managementApi.listResources(definition.value.resource, {
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value
    })
    items.value = response.items ?? []
    total.value = Number(response.total ?? 0)
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingID.value = ''
  for (const key of Object.keys(form)) delete form[key]
  for (const field of definition.value.fields) {
    if (field.type === 'status') form[field.key] = 1
    if (field.type === 'boolean') form[field.key] = false
    if (field.type === 'number') form[field.key] = 0
  }
  dialogOpen.value = true
}

function openEdit(row: ResourceRow) {
  editingID.value = String(row.id ?? '')
  for (const key of Object.keys(form)) delete form[key]
  for (const field of definition.value.fields) {
    if (field.key in row) form[field.key] = row[field.key]
  }
  dialogOpen.value = true
}

async function save() {
  const payload: ResourceRow = {}
  for (const field of definition.value.fields) {
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
  dialogOpen.value = false
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

function displayValue(value: unknown, type?: string) {
  if (type === 'status') return Number(value) === 1 ? '启用' : '禁用'
  if (type === 'boolean') return value === true || value === 1 || value === '1' ? '是' : '否'
  return value ?? '—'
}

watch(
  () => props.resourceKey,
  () => {
    page.value = 1
    void load()
  }
)
onMounted(load)
</script>

<template>
  <section v-if="definition" class="resource-page">
    <header class="resource-heading">
      <div>
        <p>MANAGEMENT</p>
        <h1>{{ definition.title }}</h1>
        <span>{{ definition.description }}</span>
      </div>
      <el-button
        v-if="!definition.readOnly"
        v-permission="`${definition.resource}:create`"
        type="danger"
        @click="openCreate"
        >新建</el-button
      >
    </header>
    <div class="query-panel">
      <el-input v-model="keyword" clearable placeholder="输入关键词搜索" @keyup.enter="load" />
      <el-button @click="load">查询</el-button>
      <el-button
        @click="
          keyword = ''
          load()
        "
        >重置</el-button
      >
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
          <template #default="scope">{{ displayValue(scope.row[field.key], field.type) }}</template>
        </el-table-column>
        <el-table-column v-if="!definition.readOnly" label="操作" fixed="right" width="138">
          <template #default="scope"
            ><el-button
              v-permission="`${definition.resource}:update`"
              link
              @click="openEdit(scope.row)"
              >编辑</el-button
            ><el-button
              v-permission="`${definition.resource}:delete`"
              link
              type="danger"
              @click="remove(scope.row)"
              >删除</el-button
            ></template
          >
        </el-table-column>
      </el-table>
      <div v-if="!loading && items.length === 0" class="mobile-empty">暂无数据</div>
      <div class="mobile-cards">
        <article v-for="row in items" :key="String(row.id)">
          <strong
            >#{{ row.id }} ·
            {{
              row.name ?? row.display_name ?? row.summary ?? row.original_name ?? definition.title
            }}</strong
          >
          <dl>
            <template v-for="field in tableFields.slice(0, 5)" :key="field.key"
              ><dt>{{ field.label }}</dt>
              <dd>{{ displayValue(row[field.key], field.type) }}</dd></template
            >
          </dl>
          <div v-if="!definition.readOnly">
            <el-button v-permission="`${definition.resource}:update`" link @click="openEdit(row)"
              >编辑</el-button
            ><el-button
              v-permission="`${definition.resource}:delete`"
              link
              type="danger"
              @click="remove(row)"
              >删除</el-button
            >
          </div>
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
    <el-dialog
      v-model="dialogOpen"
      :title="editingID ? `编辑${definition.title}` : `新建${definition.title}`"
      width="min(560px, 92vw)"
    >
      <el-form label-position="top">
        <el-form-item
          v-for="field in definition.fields.filter(
            (item) =>
              ![
                'created_at',
                'updated_at',
                'permission_version',
                'version',
                'created_by',
                'is_builtin'
              ].includes(item.key)
          )"
          :key="field.key"
          :label="field.label"
          :required="field.required"
        >
          <el-switch v-if="field.type === 'boolean'" v-model="form[field.key]" />
          <el-select v-else-if="field.type === 'status'" v-model="form[field.key]"
            ><el-option label="启用" :value="1" /><el-option label="禁用" :value="2"
          /></el-select>
          <el-input-number v-else-if="field.type === 'number'" v-model="form[field.key]" :min="0" />
          <el-input
            v-else
            v-model="form[field.key]"
            :type="field.type === 'textarea' ? 'textarea' : 'text'"
          />
        </el-form-item>
      </el-form>
      <template #footer
        ><el-button @click="dialogOpen = false">取消</el-button
        ><el-button type="danger" @click="save">保存</el-button></template
      >
    </el-dialog>
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
