<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { ElMessage, ElTree } from 'element-plus'

import * as managementApi from '@/api/management'
import type { ResourceRow } from '@/api/management'
import { Check } from '@/components/icons/actions'

const loading = ref(false)
const saving = ref(false)
const tenants = ref<ResourceRow[]>([])
const resources = ref<ResourceRow[]>([])
const grants = ref<ResourceRow[]>([])
const selectedTenant = ref<ResourceRow>()
const selectedResourceIDs = ref<string[]>([])
const featureTree = ref<InstanceType<typeof ElTree>>()

const resourceTree = computed(() => {
  const children = new Map<string, ResourceRow[]>()
  for (const item of resources.value.filter((row) => Number(row.status) === 1)) {
    const parent = String(item.parent_id ?? '0')
    children.set(parent, [...(children.get(parent) ?? []), item])
  }
  const build = (parent: string): Array<ResourceRow & { children: ResourceRow[] }> =>
    (children.get(parent) ?? []).map((item) => ({ ...item, children: build(String(item.id)) }))
  return build('0')
})
async function load() {
  loading.value = true
  try {
    const [tenantResponse, resourceResponse, grantResponse] = await Promise.all([
      managementApi.listResources('tenants', { page: 1, page_size: 200, sort: 'id:asc' }),
      managementApi.listResources('resources', { page: 1, page_size: 200, sort: 'sort_order:asc' }),
      managementApi.listResources('tenant-resources', { page: 1, page_size: 200 })
    ])
    tenants.value = tenantResponse.items ?? []
    resources.value = resourceResponse.items ?? []
    grants.value = grantResponse.items ?? []
    if (tenants.value[0]) await selectTenant(tenants.value[0])
  } finally {
    loading.value = false
  }
}

async function selectTenant(tenant: ResourceRow) {
  selectedTenant.value = tenant
  selectedResourceIDs.value = grants.value
    .filter((grant) => String(grant.tenant_id) === String(tenant.id))
    .map((grant) => String(grant.resource_id))
  await nextTick()
  featureTree.value?.setCheckedKeys?.(selectedResourceIDs.value, false)
}

function syncChecked() {
  selectedResourceIDs.value = (
    featureTree.value?.getCheckedKeys?.(false) ?? selectedResourceIDs.value
  ).map(String)
}

async function save() {
  if (!selectedTenant.value?.id) return
  syncChecked()
  saving.value = true
  try {
    await managementApi.updateTenantFeatures({
      tenantId: String(selectedTenant.value.id),
      resourceIds: selectedResourceIDs.value
    })
    ElMessage.success('租户功能授权已更新，相关访问令牌将在下次请求失效')
    await load()
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="feature-page">
    <header class="page-heading">
      <div>
        <p>TENANT ENTITLEMENTS</p>
        <h1>租户功能授权</h1>
        <span>租户只能在平台授权集合内继续配置角色权限。</span>
      </div>
      <el-button
        v-permission="'tenant-resources:update'"
        type="danger"
        :icon="Check"
        :loading="saving"
        :disabled="!selectedTenant"
        @click="save"
      >
        保存授权
      </el-button>
    </header>
    <div class="feature-layout">
      <aside class="tenant-list">
        <header>
          <strong>平台租户</strong><span>{{ tenants.length }} 个</span>
        </header>
        <button
          v-for="tenant in tenants"
          :key="String(tenant.id)"
          type="button"
          :class="{ active: selectedTenant?.id === tenant.id }"
          @click="selectTenant(tenant)"
        >
          <strong>{{ tenant.name }}</strong><span>{{ tenant.code }} · ID {{ tenant.id }}</span>
        </button>
      </aside>
      <main class="tree-panel">
        <header>
          <div>
            <small>FEATURE TREE</small>
            <h2>{{ selectedTenant?.name || '请选择租户' }}</h2>
          </div>
          <span>已授权 {{ selectedResourceIDs.length }} 项</span>
        </header>
        <el-tree
          ref="featureTree"
          :data="resourceTree"
          node-key="id"
          show-checkbox
          check-strictly
          default-expand-all
          :props="{ label: 'name', children: 'children' }"
          @check="syncChecked"
        >
          <template #default="{ data }">
            <div class="tree-node">
              <strong>{{ data.name }}</strong><span>{{ data.code }}</span>
            </div>
          </template>
        </el-tree>
      </main>
    </div>
  </section>
</template>

<style scoped lang="scss">
.feature-page {
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
.tree-panel small {
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
.feature-layout {
  min-height: 600px;
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
  gap: 16px;
}
.tenant-list,
.tree-panel {
  padding: 16px;
  background: #fff;
  box-shadow: var(--ka-shadow);
}
.tenant-list > header,
.tree-panel > header {
  display: flex;
  align-items: start;
  justify-content: space-between;
  padding: 8px 8px 16px;
  border-bottom: 1px solid var(--ka-border);
}
.tenant-list header span,
.tree-panel header > span {
  color: var(--ka-muted);
  font-size: 11px;
}
.tenant-list button {
  width: 100%;
  display: grid;
  gap: 4px;
  padding: 15px 12px;
  border: 0;
  border-bottom: 1px solid var(--ka-border);
  background: transparent;
  text-align: left;
  cursor: pointer;
}
.tenant-list button.active {
  border-left: 3px solid var(--ka-accent);
  background: #f5f5f5;
}
.tenant-list button span {
  color: var(--ka-muted);
  font-size: 11px;
}
.tree-panel h2 {
  margin: 0;
  font-size: 20px;
}
.tree-panel :deep(.el-tree) {
  margin-top: 16px;
}
.tree-panel :deep(.el-tree-node__content) {
  min-height: 42px;
  border-bottom: 1px solid #f0f0f0;
}
.tree-node {
  display: flex;
  align-items: center;
  gap: 10px;
}
.tree-node span {
  color: var(--ka-muted);
  font-size: 10px;
}
@media (max-width: 800px) {
  .feature-layout {
    grid-template-columns: 1fr;
  }
  .tenant-list {
    display: flex;
    overflow-x: auto;
  }
  .tenant-list > header {
    min-width: 115px;
    border: 0;
  }
  .tenant-list button {
    min-width: 165px;
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
  .tree-panel {
    padding: 12px;
    overflow-x: auto;
  }
}
</style>
