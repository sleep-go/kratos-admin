<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { storeToRefs } from 'pinia'
import { ElMessage, ElMessageBox } from 'element-plus'

import * as managementApi from '@/api/management'
import type { ResourceRow } from '@/api/management'
import ProviderConfigDialog from './ProviderConfigDialog.vue'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const { currentTenant } = storeToRefs(authStore)
const loading = shallowRef(false)
const saving = shallowRef(false)
const dialogOpen = shallowRef(false)
const items = shallowRef<ResourceRow[]>([])
const editing = shallowRef<ResourceRow>()
const testingID = shallowRef('')
const targetTenantId = computed(() => String(currentTenant.value?.id ?? '0'))

async function load() {
  loading.value = true
  try {
    const response = await managementApi.listResources('providers', { page: 1, page_size: 200 })
    items.value = response.items ?? []
  } finally {
    loading.value = false
  }
}

function openEditor(row?: ResourceRow) {
  editing.value = row
  dialogOpen.value = true
}

async function testConnection(row: ResourceRow) {
  testingID.value = String(row.id)
  try {
    const result = await managementApi.testProviderConnection(testingID.value)
    ElMessage.success(result.message || '连接测试成功')
  } finally {
    testingID.value = ''
  }
}

async function remove(row: ResourceRow) {
  await ElMessageBox.confirm('删除后无法恢复，确认删除该渠道配置？', '危险操作确认', {
    type: 'warning'
  })
  await managementApi.deleteResource('providers', String(row.id))
  ElMessage.success('渠道配置已删除')
  await load()
}

function typeLabel(type: unknown) {
  if (type === 'email') return '邮件'
  if (type === 'sms') return '短信'
  return '对象存储'
}

async function handleSaved() {
  dialogOpen.value = false
  await load()
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="provider-page">
    <header class="provider-heading">
      <div>
        <p>PROVIDER CENTER</p>
        <h1>渠道配置</h1>
        <span>管理邮件、短信与对象存储实现，敏感参数使用主密钥加密。</span>
      </div>
      <div>
        <router-link to="/settings"><el-button>系统设置</el-button></router-link><el-button type="danger" @click="openEditor()">新建渠道</el-button>
      </div>
    </header>
    <div class="provider-summary">
      <strong>当前作用域 ID {{ targetTenantId }}</strong><span>{{ items.length }} 个渠道 · 连接测试不会泄露密钥</span>
    </div>
    <div class="provider-grid">
      <article v-for="row in items" :key="String(row.id)">
        <header>
          <div>
            <span>{{ typeLabel(row.provider_type) }}</span>
            <h2>{{ row.display_name }}</h2>
          </div>
          <el-tag :type="Number(row.status) === 1 ? 'success' : 'info'" effect="plain">
            {{
              Number(row.status) === 1 ? '启用' : '禁用'
            }}
          </el-tag>
        </header>
        <dl>
          <div>
            <dt>实现</dt>
            <dd>{{ row.provider_name }}</dd>
          </div>
          <div>
            <dt>配置状态</dt>
            <dd>{{ row.configured ? '已加密配置' : '未配置' }}</dd>
          </div>
          <div>
            <dt>默认渠道</dt>
            <dd>{{ row.is_default ? '是' : '否' }}</dd>
          </div>
        </dl>
        <footer>
          <el-button link @click="openEditor(row)">编辑</el-button><el-button link :loading="testingID === String(row.id)" @click="testConnection(row)">
            连接测试
          </el-button><el-button link type="danger" @click="remove(row)">删除</el-button>
        </footer>
      </article>
      <div v-if="!loading && items.length === 0" class="provider-empty">
        <strong>尚未配置渠道</strong><span>先创建本地模拟器、SMTP、阿里云短信或 OSS 配置。</span><el-button type="danger" @click="openEditor()">创建第一个渠道</el-button>
      </div>
    </div>
    <ProviderConfigDialog
      v-model:saving="saving"
      :open="dialogOpen"
      :row="editing"
      :target-tenant-id="targetTenantId"
      @close="dialogOpen = false"
      @saved="handleSaved"
    />
  </section>
</template>

<style scoped lang="scss">
.provider-page {
  display: grid;
  gap: 22px;
}
.provider-heading,
.provider-summary,
.provider-grid > article,
.provider-empty {
  background: #fff;
  box-shadow: var(--ka-shadow);
}
.provider-heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  padding: 28px 30px;
  border-top: 3px solid var(--ka-accent);
}
.provider-heading > div:last-child {
  display: flex;
  gap: 10px;
}
.provider-heading p {
  margin: 0 0 5px;
  color: var(--ka-accent);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.14em;
}
.provider-heading h1 {
  margin: 0;
  font-size: 27px;
}
.provider-heading span,
.provider-summary span,
.provider-empty span {
  color: var(--ka-muted);
  font-size: 13px;
}
.provider-summary {
  display: flex;
  justify-content: space-between;
  padding: 16px 22px;
  border-left: 3px solid #555;
}
.provider-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 18px;
}
.provider-grid article {
  min-width: 0;
  padding: 22px;
}
.provider-grid article > header {
  display: flex;
  align-items: start;
  justify-content: space-between;
}
.provider-grid article header span {
  color: var(--ka-accent);
  font-size: 11px;
  font-weight: 700;
}
.provider-grid h2 {
  margin: 4px 0 0;
  font-size: 19px;
}
.provider-grid dl {
  display: grid;
  gap: 10px;
  margin: 22px 0;
}
.provider-grid dl div {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}
.provider-grid dt {
  color: var(--ka-muted);
  font-size: 12px;
}
.provider-grid dd {
  margin: 0;
  font-size: 13px;
  font-weight: 650;
}
.provider-grid footer {
  padding-top: 14px;
  border-top: 1px solid var(--ka-border);
}
.provider-empty {
  grid-column: 1 / -1;
  display: grid;
  place-items: center;
  gap: 8px;
  min-height: 260px;
  padding: 32px;
  text-align: center;
}
@media (max-width: 980px) {
  .provider-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 640px) {
  .provider-heading,
  .provider-summary {
    flex-direction: column;
    align-items: start;
    gap: 16px;
  }
  .provider-grid {
    grid-template-columns: 1fr;
  }
}
</style>
