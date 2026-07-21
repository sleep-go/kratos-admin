<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'

import * as managementApi from '@/api/management'
import type { ResourceRow } from '@/api/management'
import { providerFields, providerImplementations, sanitizeProviderConfig } from './providerSchemas'
import { Check } from '@/components/icons/actions'

const props = defineProps<{ open: boolean; row?: ResourceRow; targetTenantId: string }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const form = reactive({
  providerType: 'email',
  providerName: 'local',
  displayName: '',
  status: 1,
  isDefault: false,
  config: {} as Record<string, unknown>
})
const saving = defineModel<boolean>('saving', { default: false })
const fields = computed(() => providerFields(form.providerType, form.providerName))
const implementations = computed(() => providerImplementations(form.providerType))

watch(
  () => [props.open, props.row] as const,
  () => {
    if (!props.open) return
    form.providerType = String(props.row?.provider_type ?? 'email')
    form.providerName = String(props.row?.provider_name ?? 'local')
    form.displayName = String(props.row?.display_name ?? '')
    form.status = Number(props.row?.status ?? 1)
    form.isDefault = Boolean(props.row?.is_default)
    form.config = { ...((props.row?.config as Record<string, unknown> | undefined) ?? {}) }
    applyDefaults()
  },
  { immediate: true }
)

watch(
  () => form.providerType,
  () => {
    if (!implementations.value.includes(form.providerName))
      form.providerName = implementations.value[0] ?? ''
  }
)

watch(() => form.providerName, applyDefaults)

function applyDefaults() {
  for (const field of fields.value) {
    if (form.config[field.key] === undefined && field.default !== undefined)
      form.config[field.key] = field.default
  }
}

async function submit() {
  if (!form.displayName.trim()) return ElMessage.warning('请输入配置名称')
  saving.value = true
  try {
    const payload: ResourceRow = {
      provider_type: form.providerType,
      provider_name: form.providerName,
      display_name: form.displayName,
      status: form.status,
      is_default: form.isDefault,
      target_tenant_id: props.targetTenantId,
      config: sanitizeProviderConfig(form.config)
    }
    if (props.row?.id)
      await managementApi.updateResource('providers', String(props.row.id), payload)
    else await managementApi.createResource('providers', payload)
    ElMessage.success('渠道配置已加密保存')
    emit('saved')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '渠道配置保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog
    :model-value="open"
    :title="row?.id ? '编辑渠道配置' : '新建渠道配置'"
    width="min(620px, calc(100vw - 32px))"
    @close="emit('close')"
  >
    <el-form label-position="top" class="provider-form">
      <div class="provider-form__columns">
        <el-form-item label="渠道类型">
          <el-select v-model="form.providerType" :disabled="Boolean(row?.id)">
            <el-option label="邮件" value="email" /><el-option label="短信" value="sms" /><el-option
              label="对象存储"
              value="storage"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Provider 实现">
          <el-select v-model="form.providerName" :disabled="Boolean(row?.id)">
            <el-option v-for="name in implementations" :key="name" :label="name" :value="name" />
          </el-select>
        </el-form-item>
      </div>
      <el-form-item label="配置名称">
        <el-input v-model="form.displayName" placeholder="例如：生产环境 SMTP" />
      </el-form-item>
      <div v-if="fields.length" class="provider-config-box">
        <header><strong>连接参数</strong><span>敏感参数保存后不会再次回显</span></header>
        <el-form-item v-for="field in fields" :key="field.key" :label="field.label">
          <el-switch v-if="field.boolean" v-model="form.config[field.key]" />
          <el-input
            v-else
            v-model="form.config[field.key]"
            :type="field.secret ? 'password' : 'text'"
            :placeholder="
              field.secret && form.config[`${field.key}_configured`] ? '已配置，留空保持不变' : ''
            "
            show-password
            autocomplete="new-password"
          />
        </el-form-item>
      </div>
      <div class="provider-form__columns">
        <el-form-item label="状态">
          <el-select v-model="form.status">
            <el-option label="启用" :value="1" /><el-option label="禁用" :value="2" />
          </el-select>
        </el-form-item>
        <el-form-item label="默认渠道">
          <el-switch v-model="form.isDefault" active-text="设为当前作用域默认" />
        </el-form-item>
      </div>
    </el-form>
    <template #footer>
      <el-button @click="emit('close')">取消</el-button><el-button type="danger" :icon="Check" :loading="saving" @click="submit">加密保存</el-button>
    </template>
  </el-dialog>
</template>

<style scoped lang="scss">
.provider-form__columns {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}
.provider-config-box {
  margin: 4px 0 20px;
  padding: 18px;
  border: 1px solid var(--ka-border);
  background: #fafafa;
}
.provider-config-box header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
}
.provider-config-box header span {
  color: var(--ka-muted);
  font-size: 12px;
}
@media (max-width: 600px) {
  .provider-form__columns {
    grid-template-columns: 1fr;
    gap: 0;
  }
  .provider-config-box header {
    display: grid;
    gap: 4px;
  }
}
</style>
