<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import * as managementApi from '@/api/management'
import type { ResourceRow } from '@/api/management'
import ResourceListView from '@/views/management/ResourceListView.vue'
import RolePermissionView from '@/views/permission/RolePermissionView.vue'
import TenantFeatureView from '@/views/permission/TenantFeatureView.vue'

const props = defineProps<{ tenantId: string }>()

const activeTab = ref('features')
const tenant = ref<ResourceRow>()
const loading = ref(false)
const loaded = ref(false)
const tenantName = computed(() => String(tenant.value?.name ?? '待开通租户'))
const targetAvailable = computed(() => Boolean(tenant.value) && Number(tenant.value?.status) === 1)

async function loadTenant() {
  loading.value = true
  loaded.value = false
  try {
    const response = await managementApi.listResources('tenants', {
      page: 1,
      page_size: 200,
      sort: 'id:asc'
    })
    tenant.value = (response.items ?? []).find((item) => String(item.id) === props.tenantId)
  } finally {
    loading.value = false
    loaded.value = true
  }
}

watch(
  () => props.tenantId,
  () => {
    activeTab.value = 'features'
    void loadTenant()
  },
  { immediate: true }
)
</script>

<template>
  <section v-loading="loading" class="tenant-setup-page">
    <header class="page-heading">
      <div>
        <p>TENANT PROVISIONING</p>
        <h1>租户开通 · {{ tenantName }}</h1>
        <span>目标租户 ID {{ tenantId }}。组织、权限、日志等后台治理由平台管理员在开通页维护；此处仅授权租户业务功能模块。</span>
      </div>
      <router-link to="/platform/tenants">返回租户列表</router-link>
    </header>

    <el-tabs v-if="targetAvailable" v-model="activeTab" class="setup-tabs">
      <el-tab-pane data-testid="setup-tab" label="功能授权" name="features">
        <TenantFeatureView embedded :target-tenant-id="tenantId" />
      </el-tab-pane>
      <el-tab-pane data-testid="setup-tab" label="角色管理" name="roles" lazy>
        <RolePermissionView embedded :target-tenant-id="tenantId" />
      </el-tab-pane>
      <el-tab-pane data-testid="setup-tab" label="租户管理员" name="tenant-admins" lazy>
        <ResourceListView embedded resource-key="tenant-admins" :target-tenant-id="tenantId" />
      </el-tab-pane>
    </el-tabs>
    <div v-else-if="loaded" class="target-unavailable">
      目标租户不存在、已冻结或已删除，不能继续开通配置。
    </div>
  </section>
</template>

<style scoped lang="scss">
.tenant-setup-page {
  display: grid;
  gap: 18px;
}
.page-heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 18px;
}
.page-heading p {
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
.page-heading a {
  color: var(--ka-accent);
  font-size: 13px;
  font-weight: 650;
  text-decoration: none;
  white-space: nowrap;
}
.setup-tabs {
  padding: 16px;
  background: #fff;
  box-shadow: var(--ka-shadow);
}
.target-unavailable {
  padding: 36px 20px;
  background: #fff;
  color: var(--ka-muted);
  text-align: center;
  box-shadow: var(--ka-shadow);
}
@media (max-width: 560px) {
  .page-heading {
    align-items: start;
    flex-direction: column;
  }
}
</style>
