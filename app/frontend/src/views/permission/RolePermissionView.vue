<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, ElTree } from 'element-plus'

import * as managementApi from '@/api/management'
import { useAuthStore } from '@/stores/auth'
import { Check, Delete, Plus } from '@/components/icons/actions'
import type { ResourceRow } from '@/api/management'

const props = defineProps<{ targetTenantId?: string }>()
const actions = [
  { value: 'list', label: '查看' },
  { value: 'create', label: '新建' },
  { value: 'update', label: '编辑' },
  { value: 'delete', label: '删除' },
  { value: 'export', label: '导出' },
  { value: 'download', label: '下载' }
]
const authStore = useAuthStore()
const scopeOptions = [
  { value: 1, label: '全部数据' },
  { value: 2, label: '本部门及下级' },
  { value: 3, label: '本部门' },
  { value: 4, label: '仅本人' },
  { value: 5, label: '自定义部门' }
]

const loading = ref(false)
const saving = ref(false)
const roles = ref<ResourceRow[]>([])
const resources = ref<ResourceRow[]>([])
const departments = ref<ResourceRow[]>([])
const policyRows = ref<ResourceRow[]>([])
const scopeRows = ref<ResourceRow[]>([])
const selectedRole = ref<ResourceRow>()
const selectedPermissions = ref<string[]>([])
const selectedDepartments = ref<string[]>([])
const dataScope = ref(4)
const departmentTreeRef = ref<InstanceType<typeof ElTree>>()
const roleDialogOpen = ref(false)
const roleForm = reactive({ code: '', name: '', data_scope: 4, status: 1 })
const targetScope = computed(() =>
  props.targetTenantId ? { targetTenantId: props.targetTenantId } : undefined
)

const selectableResources = computed(() =>
  resources.value.filter((item) => Number(item.type) >= 2 && Number(item.status) === 1)
)
const departmentTree = computed(() => {
  const children = new Map<string, ResourceRow[]>()
  for (const item of departments.value) {
    const parent = String(item.parent_id ?? '0')
    children.set(parent, [...(children.get(parent) ?? []), item])
  }
  const build = (parent: string): Array<ResourceRow & { children: ResourceRow[] }> =>
    (children.get(parent) ?? []).map((item) => ({
      ...item,
      children: build(String(item.id))
    }))
  return build('0')
})

async function load() {
  loading.value = true
  try {
    const [roleResponse, resourceResponse, departmentResponse, grantResponse] = await Promise.all([
      managementApi.listResources(
        'roles',
        { page: 1, page_size: 200, sort: 'sort_order:asc' },
        targetScope.value
      ),
      managementApi.listResources('resources', { page: 1, page_size: 200, sort: 'sort_order:asc' }),
      managementApi.listResources(
        'departments',
        { page: 1, page_size: 200, sort: 'sort_order:asc' },
        targetScope.value
      ),
      props.targetTenantId
        ? managementApi.listResources('tenant-resources', { page: 1, page_size: 200 })
        : Promise.resolve({ items: [], total: 0 })
    ])
    roles.value = roleResponse.items ?? []
    const enabledResourceIDs = new Set(
      (grantResponse.items ?? [])
        .filter((grant) => String(grant.tenant_id) === props.targetTenantId)
        .map((grant) => String(grant.resource_id))
    )
    resources.value = props.targetTenantId
      ? (resourceResponse.items ?? []).filter((resource) =>
          enabledResourceIDs.has(String(resource.id))
        )
      : (resourceResponse.items ?? [])
    departments.value = departmentResponse.items ?? []
    if (!selectedRole.value && roles.value[0]) await selectRole(roles.value[0])
  } finally {
    loading.value = false
  }
}

async function selectRole(role: ResourceRow) {
  selectedRole.value = role
  dataScope.value = Number(role.data_scope ?? 4)
  const roleId = String(role.id)
  const [policies, scopes] = await Promise.all([
    managementApi.listResources(
      'casbin-rules',
      { page: 1, page_size: 200, filters: { ptype: 'p', v1: roleId } },
      targetScope.value
    ),
    managementApi.listResources(
      'role-scope-departments',
      { page: 1, page_size: 200, filters: { role_id: roleId } },
      targetScope.value
    )
  ])
  policyRows.value = (policies.items ?? []).filter(
    (item) => item.ptype === 'p' && String(item.v1) === roleId
  )
  scopeRows.value = scopes.items ?? []
  selectedPermissions.value = policyRows.value.map((item) => `${item.v2}:${item.v3}`)
  selectedDepartments.value = scopeRows.value.map((item) => String(item.department_id))
  await nextTick()
  departmentTreeRef.value?.setCheckedKeys?.(selectedDepartments.value, false)
}

function syncDepartments() {
  selectedDepartments.value = (
    departmentTreeRef.value?.getCheckedKeys?.(false) ?? selectedDepartments.value
  ).map(String)
}

async function save() {
  if (!selectedRole.value?.id) return
  syncDepartments()
  saving.value = true
  try {
    const roleId = String(selectedRole.value.id)
    const grantMap = new Map<string, string[]>()
    for (const permission of selectedPermissions.value) {
      const separator = permission.lastIndexOf(':')
      const code = permission.slice(0, separator)
      grantMap.set(code, [...(grantMap.get(code) ?? []), permission.slice(separator + 1)])
    }
    await managementApi.updateRoleAuthorization({
      roleId,
      dataScope: dataScope.value,
      grants: [...grantMap].map(([resourceCode, resourceActions]) => ({
        resourceCode,
        actions: resourceActions
      })),
      departmentIds: dataScope.value === 5 ? selectedDepartments.value : [],
      targetTenantId: props.targetTenantId
    })
    if (!props.targetTenantId) await authStore.renewSession()
    ElMessage.success('角色授权与数据范围已生效，现有令牌将在下次请求重新加载')
    await selectRole(selectedRole.value)
  } finally {
    saving.value = false
  }
}

function openRoleDialog() {
  Object.assign(roleForm, { code: '', name: '', data_scope: 4, status: 1 })
  roleDialogOpen.value = true
}

async function createRole() {
  await managementApi.createResource('roles', roleForm, targetScope.value)
  roleDialogOpen.value = false
  selectedRole.value = undefined
  ElMessage.success('角色已创建')
  if (!props.targetTenantId) await authStore.renewSession()
  await load()
}

async function removeRole(role: ResourceRow) {
  await ElMessageBox.confirm(`确认删除角色“${role.name}”？`, '危险操作确认', { type: 'warning' })
  await managementApi.deleteResource('roles', String(role.id), targetScope.value)
  selectedRole.value = undefined
  ElMessage.success('角色已删除')
  if (!props.targetTenantId) await authStore.renewSession()
  await load()
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="permission-page">
    <header class="page-heading">
      <div>
        <p>RBAC POLICY</p>
        <h1>角色授权</h1>
        <span>统一配置 Casbin 资源动作与五类数据范围。</span>
      </div>
      <el-button
        v-permission="'roles:update'"
        type="danger"
        :icon="Check"
        :loading="saving"
        :disabled="!selectedRole"
        @click="save"
      >
        保存并生效
      </el-button>
    </header>

    <div class="permission-layout">
      <aside class="role-list">
        <header>
          <div>
            <strong>角色</strong><span>{{ roles.length }} 个</span>
          </div>
          <el-button
            v-permission="'roles:create'"
            link
            type="danger"
            :icon="Plus"
            @click="openRoleDialog"
          >
            新建角色
          </el-button>
        </header>
        <div
          v-for="role in roles"
          :key="String(role.id)"
          class="role-item"
          :class="{ active: selectedRole?.id === role.id }"
        >
          <button type="button" @click="selectRole(role)">
            <strong>{{ role.name }}</strong
            ><span>{{ role.code }}</span>
          </button>
          <el-button
            v-if="!role.is_builtin"
            v-permission="'roles:delete'"
            link
            type="danger"
            :icon="Delete"
            @click="removeRole(role)"
          >
            删除
          </el-button>
        </div>
      </aside>

      <main v-if="selectedRole" class="editor-panel">
        <section class="scope-section">
          <div class="section-title">
            <div>
              <small>DATA SCOPE</small>
              <h2>数据范围</h2>
            </div>
            <span>多角色在仓储层取并集</span>
          </div>
          <el-radio-group v-model="dataScope" class="scope-grid">
            <el-radio v-for="option in scopeOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </el-radio>
          </el-radio-group>
          <div v-show="dataScope === 5" class="department-scope">
            <strong>自定义部门</strong>
            <el-tree
              ref="departmentTreeRef"
              :data="departmentTree"
              node-key="id"
              show-checkbox
              :props="{ label: 'name', children: 'children' }"
              @check="syncDepartments"
            />
          </div>
        </section>

        <section class="resource-section">
          <div class="section-title">
            <div>
              <small>RESOURCE ACTIONS</small>
              <h2>菜单、按钮与 API 权限</h2>
            </div>
            <span>{{ selectedPermissions.length }} 项已选</span>
          </div>
          <el-checkbox-group v-model="selectedPermissions" class="resource-list">
            <article v-for="resource in selectableResources" :key="String(resource.id)">
              <header>
                <div>
                  <strong>{{ resource.name }}</strong
                  ><span>{{ resource.code }}</span>
                </div>
                <small>{{
                  Number(resource.type) === 2
                    ? '菜单'
                    : Number(resource.type) === 3
                      ? '按钮'
                      : 'API'
                }}</small>
              </header>
              <div class="action-grid">
                <el-checkbox
                  v-for="action in actions"
                  :key="action.value"
                  :value="`${resource.code}:${action.value}`"
                >
                  {{ action.label }}
                </el-checkbox>
              </div>
            </article>
          </el-checkbox-group>
        </section>
      </main>
      <div v-else class="empty-state">请先创建角色</div>
    </div>
    <el-dialog v-model="roleDialogOpen" title="新建角色" width="min(520px, 92vw)">
      <el-form label-position="top">
        <el-form-item label="角色名称" required><el-input v-model="roleForm.name" /></el-form-item>
        <el-form-item label="角色编码" required><el-input v-model="roleForm.code" /></el-form-item>
        <el-form-item label="初始数据范围">
          <el-select v-model="roleForm.data_scope" class="full-width">
            <el-option
              v-for="option in scopeOptions"
              :key="option.value"
              :label="option.label"
              :value="option.value"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="roleDialogOpen = false">取消</el-button>
        <el-button type="danger" :icon="Plus" @click="createRole">创建</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<style scoped lang="scss">
.permission-page {
  display: grid;
  gap: 18px;
}
.page-heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 18px;
}
.page-heading p,
.section-title small {
  margin: 0 0 5px;
  color: var(--ka-accent);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.18em;
}
.page-heading h1 {
  margin: 0;
  font-size: clamp(28px, 4vw, 40px);
}
.page-heading span {
  display: block;
  margin-top: 8px;
  color: var(--ka-muted);
}
.permission-layout {
  min-height: 600px;
  display: grid;
  grid-template-columns: 250px minmax(0, 1fr);
  gap: 16px;
}
.role-list,
.editor-panel,
.empty-state {
  background: #fff;
  box-shadow: var(--ka-shadow);
}
.role-list {
  padding: 14px;
}
.role-list > header {
  display: flex;
  justify-content: space-between;
  padding: 10px 8px 16px;
  border-bottom: 1px solid var(--ka-border);
}
.role-list > header > div {
  display: grid;
  gap: 3px;
}
.role-list > header span {
  color: var(--ka-muted);
  font-size: 12px;
}
.role-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  border-bottom: 1px solid var(--ka-border);
}
.role-item > button {
  width: 100%;
  display: grid;
  gap: 4px;
  padding: 14px 12px;
  border: 0;
  background: transparent;
  text-align: left;
  cursor: pointer;
}
.role-item.active {
  border-left: 3px solid var(--ka-accent);
  background: #f5f5f5;
}
.role-item button span {
  color: var(--ka-muted);
  font-size: 11px;
}
.role-item > .el-button {
  padding-right: 8px;
}
.full-width {
  width: 100%;
}
.editor-panel {
  padding: 24px;
}
.scope-section {
  padding-bottom: 24px;
  border-bottom: 1px solid var(--ka-border);
}
.section-title {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 12px;
}
.section-title h2 {
  margin: 0;
  font-size: 19px;
}
.section-title > span {
  color: var(--ka-muted);
  font-size: 11px;
}
.scope-grid {
  margin-top: 18px;
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 8px;
}
.department-scope {
  margin-top: 16px;
  padding: 16px;
  background: #f7f7f7;
}
.department-scope strong {
  display: block;
  margin-bottom: 10px;
  font-size: 13px;
}
.resource-section {
  padding-top: 24px;
}
.resource-list {
  margin-top: 14px;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}
.resource-list article {
  padding: 15px;
  border: 1px solid var(--ka-border);
}
.resource-list article > header {
  display: flex;
  justify-content: space-between;
  gap: 10px;
}
.resource-list header div {
  display: grid;
  gap: 3px;
}
.resource-list header span,
.resource-list header small {
  color: var(--ka-muted);
  font-size: 10px;
}
.action-grid {
  margin-top: 12px;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}
.empty-state {
  display: grid;
  place-items: center;
  color: var(--ka-muted);
}
@media (max-width: 900px) {
  .permission-layout {
    grid-template-columns: 1fr;
  }
  .role-list {
    display: flex;
    overflow-x: auto;
  }
  .role-list > header {
    min-width: 100px;
    border: 0;
  }
  .role-item {
    min-width: 150px;
    border-left: 0;
  }
  .scope-grid,
  .resource-list {
    grid-template-columns: 1fr 1fr;
  }
}
@media (max-width: 560px) {
  .page-heading {
    align-items: start;
    flex-direction: column;
  }
  .page-heading .el-button {
    width: 100%;
  }
  .editor-panel {
    padding: 18px;
  }
  .scope-grid,
  .resource-list {
    grid-template-columns: 1fr;
  }
}
</style>
