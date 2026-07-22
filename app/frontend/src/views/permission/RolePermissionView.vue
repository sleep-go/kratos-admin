<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import * as managementApi from '@/api/management'
import { useAuthStore } from '@/stores/auth'
import { Check, Delete, Plus } from '@/components/icons/actions'
import type { ResourceRow } from '@/api/management'

const props = defineProps<{
  targetTenantId?: string
  embedded?: boolean
  platformMode?: boolean
}>()
const actions = [
  { value: 'list', label: '查看' },
  { value: 'create', label: '新建' },
  { value: 'update', label: '编辑' },
  { value: 'delete', label: '删除' },
  { value: 'export', label: '导出' },
  { value: 'download', label: '下载' }
]
// 数据范围功能暂不开放，保存时固定为全部数据。
const DEFAULT_DATA_SCOPE = 1
const authStore = useAuthStore()

const loading = ref(false)
const saving = ref(false)
const roles = ref<ResourceRow[]>([])
const resources = ref<ResourceRow[]>([])
const policyRows = ref<ResourceRow[]>([])
const selectedRole = ref<ResourceRow>()
const selectedPermissions = ref<string[]>([])
const selectedAdminIDs = ref<string[]>([])
const platformAdmins = ref<ResourceRow[]>([])
const roleDialogOpen = ref(false)
const roleForm = reactive({ code: '', name: '', data_scope: DEFAULT_DATA_SCOPE, status: 1 })
const roleResourceKey = computed(() => (props.platformMode ? 'platform-roles' : 'roles'))
const policyResourceKey = computed(() =>
  props.platformMode ? 'platform-casbin-rules' : 'casbin-rules'
)
const targetScope = computed(() =>
  props.targetTenantId ? { targetTenantId: props.targetTenantId } : undefined
)

const selectableResources = computed(() =>
  resources.value.filter((item) => {
    if (Number(item.status) !== 1 || Number(item.type) < 2) return false
    if (props.platformMode) return (Number(item.scope_mask) & 1) === 1
    return true
  })
)

const selectablePlatformAdmins = computed(() =>
  platformAdmins.value.filter((admin) => !admin.is_super_admin && Number(admin.status) === 1)
)

const pageTitle = computed(() => (props.platformMode ? '平台角色' : '角色授权'))
const pageDescription = computed(() =>
  props.platformMode
    ? '配置平台角色权限，并将普通平台管理员绑定到角色。超级管理员无需绑定。'
    : '统一配置 Casbin 资源动作权限。'
)

async function load() {
  loading.value = true
  try {
    const adminPromise = props.platformMode
      ? managementApi.listResources('platform-admins', { page: 1, page_size: 200, sort: 'id:asc' })
      : Promise.resolve({ items: [], total: 0 })
    const [roleResponse, resourceResponse, grantResponse, adminResponse] = await Promise.all([
      managementApi.listResources(
        roleResourceKey.value,
        { page: 1, page_size: 200, sort: 'sort_order:asc' },
        targetScope.value
      ),
      managementApi.listResources('resources', { page: 1, page_size: 200, sort: 'sort_order:asc' }),
      props.targetTenantId
        ? managementApi.listResources(
            'tenant-resources',
            { page: 1, page_size: 200 },
            targetScope.value
          )
        : Promise.resolve({ items: [], total: 0 }),
      adminPromise
    ])
    roles.value = roleResponse.items ?? []
    platformAdmins.value = adminResponse.items ?? []
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
    if (!selectedRole.value && roles.value[0]) await selectRole(roles.value[0])
  } finally {
    loading.value = false
  }
}

async function selectRole(role: ResourceRow) {
  selectedRole.value = role
  const roleId = String(role.id)
  const [policies, groupPolicies] = await Promise.all([
    managementApi.listResources(
      policyResourceKey.value,
      { page: 1, page_size: 200, filters: { ptype: 'p', v1: roleId } },
      targetScope.value
    ),
    props.platformMode
      ? managementApi.listResources(policyResourceKey.value, {
          page: 1,
          page_size: 200,
          filters: { ptype: 'g', v2: roleId }
        })
      : Promise.resolve({ items: [], total: 0 })
  ])
  policyRows.value = (policies.items ?? []).filter(
    (item) => item.ptype === 'p' && String(item.v1) === roleId
  )
  selectedPermissions.value = policyRows.value.map((item) => `${item.v2}:${item.v3}`)
  selectedAdminIDs.value = (groupPolicies.items ?? [])
    .filter((item) => item.ptype === 'g' && String(item.v2) === roleId)
    .map((item) => String(item.v1))
}

async function save() {
  if (!selectedRole.value?.id) return
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
      dataScope: DEFAULT_DATA_SCOPE,
      grants: [...grantMap].map(([resourceCode, resourceActions]) => ({
        resourceCode,
        actions: resourceActions
      })),
      departmentIds: [],
      adminIds: props.platformMode ? selectedAdminIDs.value.map((id) => Number(id)) : undefined,
      targetTenantId: props.targetTenantId
    })
    if (!props.targetTenantId) await authStore.renewSession()
    ElMessage.success('角色授权已生效，现有令牌将在下次请求重新加载')
    await selectRole(selectedRole.value)
  } finally {
    saving.value = false
  }
}

function openRoleDialog() {
  Object.assign(roleForm, { code: '', name: '', data_scope: DEFAULT_DATA_SCOPE, status: 1 })
  roleDialogOpen.value = true
}

async function createRole() {
  await managementApi.createResource(roleResourceKey.value, roleForm, targetScope.value)
  roleDialogOpen.value = false
  selectedRole.value = undefined
  ElMessage.success('角色已创建')
  if (!props.targetTenantId) await authStore.renewSession()
  await load()
}

async function removeRole(role: ResourceRow) {
  await ElMessageBox.confirm(`确认删除角色“${role.name}”？`, '危险操作确认', { type: 'warning' })
  await managementApi.deleteResource(roleResourceKey.value, String(role.id), targetScope.value)
  selectedRole.value = undefined
  ElMessage.success('角色已删除')
  if (!props.targetTenantId) await authStore.renewSession()
  await load()
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="permission-page" :class="{ 'permission-page--embedded': embedded }">
    <header v-if="!embedded" class="page-heading">
      <div>
        <p>RBAC POLICY</p>
        <h1>{{ pageTitle }}</h1>
        <span>{{ pageDescription }}</span>
      </div>
      <el-button
        v-permission="platformMode ? 'platform-roles:update' : 'roles:update'"
        type="danger"
        :icon="Check"
        :loading="saving"
        :disabled="!selectedRole"
        @click="save"
      >
        保存并生效
      </el-button>
    </header>
    <div v-else-if="selectedRole" class="embedded-toolbar">
      <span>为当前租户配置角色权限</span>
      <el-button
        v-permission="platformMode ? 'platform-roles:update' : 'roles:update'"
        type="danger"
        :icon="Check"
        :loading="saving"
        @click="save"
      >
        保存并生效
      </el-button>
    </div>

    <div class="permission-layout">
      <aside class="role-list">
        <header>
          <div>
            <strong>角色</strong><span>{{ roles.length }} 个</span>
          </div>
          <el-button
            v-permission="platformMode ? 'platform-roles:create' : 'roles:create'"
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
            v-permission="platformMode ? 'platform-roles:delete' : 'roles:delete'"
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
        <section v-if="platformMode" class="admin-section">
          <div class="section-title">
            <div>
              <small>PLATFORM ADMINS</small>
              <h2>绑定平台管理员</h2>
            </div>
            <span>{{ selectedAdminIDs.length }} 人已选</span>
          </div>
          <p class="section-hint">超级管理员拥有全部权限，无需绑定角色。</p>
          <el-checkbox-group v-model="selectedAdminIDs" class="admin-list">
            <el-checkbox
              v-for="admin in selectablePlatformAdmins"
              :key="String(admin.id)"
              :value="String(admin.id)"
            >
              {{ admin.display_name }}（{{ admin.username }}）
            </el-checkbox>
          </el-checkbox-group>
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
.permission-page--embedded {
  gap: 12px;
}
.embedded-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: var(--ka-muted);
  font-size: 13px;
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
.editor-panel {
  padding: 24px;
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
.resource-section {
  padding-top: 0;
}
.admin-section {
  margin-bottom: 24px;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--ka-border);
}
.section-hint {
  margin: 8px 0 12px;
  color: var(--ka-muted);
  font-size: 12px;
}
.admin-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
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
  .resource-list {
    grid-template-columns: 1fr;
  }
}
</style>
