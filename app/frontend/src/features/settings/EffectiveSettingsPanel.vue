<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { storeToRefs } from 'pinia'

import * as managementApi from '@/api/management'
import type { ResourceRow } from '@/api/management'
import SettingEditorDialog from './SettingEditorDialog.vue'
import {
  settingCategoryLabel,
  settingKeyLabel,
  settingSourceLabel,
  settingValueTypeLabel
} from './settingLabels'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const { currentTenant, currentUser } = storeToRefs(authStore)
const loading = shallowRef(false)
const editorOpen = shallowRef(false)
const saving = shallowRef(false)
const items = shallowRef<ResourceRow[]>([])
const storedItems = shallowRef<ResourceRow[]>([])
const selected = shallowRef<ResourceRow>()
const targetTenantId = computed(() => String(currentTenant.value?.id ?? '0'))
const platformContext = computed(
  () => Boolean(currentUser.value?.platformAdmin) && targetTenantId.value === '0'
)
const categories = computed(() => {
  const result = new Map<string, ResourceRow[]>()
  for (const item of items.value) {
    const category = String(item.category)
    result.set(category, [...(result.get(category) ?? []), item])
  }
  return [...result.entries()]
})

async function load() {
  loading.value = true
  try {
    const [effective, stored] = await Promise.all([
      managementApi.getEffectiveSettings(),
      managementApi.listResources('settings', { page: 1, page_size: 200 })
    ])
    items.value = effective.items ?? []
    storedItems.value = stored.items ?? []
  } finally {
    loading.value = false
  }
}

function storedFor(item?: ResourceRow) {
  if (!item) return undefined
  return storedItems.value.find(
    (row) => row.category === item.category && row.setting_key === item.setting_key
  )
}

function edit(item: ResourceRow) {
  selected.value = item
  editorOpen.value = true
}

function displayValue(item: ResourceRow) {
  if (item.is_secret) return item.configured ? '已安全配置' : '尚未配置'
  const value = item.setting_value
  return typeof value === 'object' ? JSON.stringify(value) : String(value ?? '—')
}

async function handleSaved() {
  editorOpen.value = false
  await load()
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="settings-page">
    <header class="settings-heading">
      <div>
        <p>SYSTEM SETTINGS</p>
        <h1>系统设置</h1>
        <span>按代码安全默认、平台默认和租户允许覆盖值解析当前生效配置。</span>
      </div>
      <router-link to="/settings/providers"><el-button>渠道配置</el-button></router-link>
    </header>

    <div class="scope-notice">
      <strong>{{ platformContext ? '平台配置作用域' : '当前租户作用域' }}</strong>
      <span>作用域 ID {{ targetTenantId }} · 敏感值只显示配置状态</span>
    </div>

    <section v-for="[category, categoryItems] in categories" :key="category" class="setting-group">
      <header>
        <h2>{{ settingCategoryLabel(category) }}</h2>
        <span>{{ categoryItems.length }} 项配置</span>
      </header>
      <div class="setting-grid">
        <article v-for="item in categoryItems" :key="String(item.setting_key)">
          <div class="setting-card__top">
            <div>
              <small>{{ settingKeyLabel(category, String(item.setting_key)) }}</small
              ><strong>{{ displayValue(item) }}</strong>
            </div>
            <el-tag :type="item.source === 'tenant' ? 'danger' : 'info'" effect="plain">
              {{ settingSourceLabel(String(item.source)) }}
            </el-tag>
          </div>
          <footer>
            <span
              >{{ settingValueTypeLabel(String(item.value_type))
              }}<template v-if="item.allow_tenant_override"> · 可覆盖</template></span
            >
            <el-button link type="danger" @click="edit(item)">配置</el-button>
          </footer>
        </article>
      </div>
    </section>

    <SettingEditorDialog
      v-model:saving="saving"
      :open="editorOpen"
      :item="selected"
      :stored="storedFor(selected)"
      :target-tenant-id="targetTenantId"
      :platform-context="platformContext"
      @close="editorOpen = false"
      @saved="handleSaved"
    />
  </section>
</template>

<style scoped lang="scss">
.settings-page {
  display: grid;
  gap: 22px;
}
.settings-heading,
.scope-notice,
.setting-group {
  background: var(--ka-card-bg);
  box-shadow: var(--ka-shadow);
}
.settings-heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  padding: 28px 30px;
  border-top: 3px solid var(--ka-accent);
}
.settings-heading p {
  margin: 0 0 5px;
  color: var(--ka-accent);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.14em;
}
.settings-heading h1 {
  margin: 0;
  font-size: 27px;
}
.settings-heading span,
.scope-notice span {
  color: var(--ka-muted);
  font-size: 13px;
}
.scope-notice {
  display: flex;
  justify-content: space-between;
  padding: 16px 22px;
  border-left: 3px solid #555;
}
.setting-group {
  padding: 22px;
}
.setting-group > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.setting-group h2 {
  margin: 0;
  font-size: 18px;
  text-transform: capitalize;
}
.setting-group header span {
  color: var(--ka-muted);
  font-size: 12px;
}
.setting-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}
.setting-grid article {
  min-width: 0;
  padding: 18px;
  border: 1px solid var(--ka-border);
  background: #fafafa;
}
.setting-card__top {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 12px;
}
.setting-card__top div {
  min-width: 0;
  display: grid;
  gap: 8px;
}
.setting-card__top small {
  overflow: hidden;
  color: var(--ka-muted);
  text-overflow: ellipsis;
}
.setting-card__top strong {
  overflow-wrap: anywhere;
  font-size: 17px;
}
.setting-grid footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 18px;
  padding-top: 12px;
  border-top: 1px solid var(--ka-border);
  color: var(--ka-muted);
  font-size: 12px;
}
@media (max-width: 980px) {
  .setting-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 640px) {
  .settings-heading {
    align-items: start;
    gap: 18px;
    padding: 22px;
  }
  .settings-heading,
  .scope-notice {
    flex-direction: column;
  }
  .setting-grid {
    grid-template-columns: 1fr;
  }
}
</style>
