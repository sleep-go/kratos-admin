<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import * as managementApi from '@/api/management'
import type { ResourceRow } from '@/api/management'
import ResourceListView from '@/views/management/ResourceListView.vue'
import RolePermissionView from '@/views/permission/RolePermissionView.vue'
import TenantFeatureView from '@/views/permission/TenantFeatureView.vue'

const props = defineProps<{ tenantId: string }>()

const activeTab = ref('features')
const tenant = ref<ResourceRow>()
const loaded = ref(false)
const tenantName = computed(() => String(tenant.value?.name ?? '待初始化租户'))
const targetAvailable = computed(() => Boolean(tenant.value) && Number(tenant.value?.status) === 1)

async function loadTenant() {
  const response = await managementApi.listResources('tenants', {
    page: 1,
    page_size: 1,
    filters: { id: props.tenantId }
  })
  tenant.value = (response.items ?? []).find((item) => String(item.id) === props.tenantId)
  loaded.value = true
}

onMounted(loadTenant)
</script>

<template>
  <section class="tenant-setup-page">
    <header class="page-heading">
      <div>
        <p>TENANT SETUP</p>
        <h1>租户初始化 · {{ tenantName }}</h1>
        <span>目标租户 ID {{ tenantId }}。先授权功能，再创建角色、全局用户并添加为租户成员。</span>
      </div>
      <router-link to="/platform/tenants">返回租户列表</router-link>
    </header>

    <el-tabs v-if="targetAvailable" v-model="activeTab" class="setup-tabs">
      <el-tab-pane data-testid="setup-tab" label="功能授权" name="features">
        <TenantFeatureView :target-tenant-id="tenantId" />
      </el-tab-pane>
      <el-tab-pane data-testid="setup-tab" label="角色管理" name="roles" lazy>
        <RolePermissionView :target-tenant-id="tenantId" />
      </el-tab-pane>
      <el-tab-pane data-testid="setup-tab" label="用户管理" name="users" lazy>
        <ResourceListView resource-key="users" />
      </el-tab-pane>
      <el-tab-pane data-testid="setup-tab" label="成员管理" name="members" lazy>
        <ResourceListView resource-key="members" :target-tenant-id="tenantId" />
      </el-tab-pane>
    </el-tabs>
    <div v-else-if="loaded" class="target-unavailable">
      目标租户不存在、已冻结或已删除，不能继续初始化。
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
