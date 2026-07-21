<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'

import * as managementApi from '@/api/management'
import type { ResourceRow } from '@/api/management'
import { settingKeyLabel } from './settingLabels'
import { Check } from '@/components/icons/actions'

const props = defineProps<{
  open: boolean
  item?: ResourceRow
  stored?: ResourceRow
  targetTenantId: string
  platformContext: boolean
}>()

const emit = defineEmits<{ close: []; saved: [] }>()
const saving = defineModel<boolean>('saving', { default: false })
const form = reactive({
  valueType: 'string',
  settingValue: '',
  allowTenantOverride: false,
  isSecret: false
})
const title = computed(
  () =>
    `配置 ${settingKeyLabel(String(props.item?.category ?? ''), String(props.item?.setting_key ?? ''))}`
)

watch(
  () => [props.open, props.item, props.stored] as const,
  () => {
    if (!props.open || !props.item) return
    form.valueType = String(props.stored?.value_type ?? props.item.value_type ?? 'string')
    form.isSecret = Boolean(props.stored?.is_secret ?? props.item.is_secret)
    form.allowTenantOverride = Boolean(
      props.stored?.allow_tenant_override ?? props.item.allow_tenant_override
    )
    const value = props.stored?.setting_value ?? props.item.setting_value
    form.settingValue = form.isSecret ? '' : formatValue(value)
  },
  { immediate: true }
)

function formatValue(value: unknown) {
  if (value === undefined || value === null) return ''
  return typeof value === 'object' ? JSON.stringify(value, null, 2) : String(value)
}

function parseValue() {
  if (form.valueType === 'number') {
    const value = Number(form.settingValue)
    if (!Number.isFinite(value)) throw new Error('请输入有效数字')
    return value
  }
  if (form.valueType === 'boolean') return form.settingValue === 'true'
  if (form.valueType === 'json') return JSON.parse(form.settingValue)
  return form.settingValue
}

async function submit() {
  saving.value = true
  try {
    const payload: ResourceRow = {
      category: props.item?.category,
      setting_key: props.item?.setting_key,
      value_type: form.valueType,
      allow_tenant_override: props.platformContext ? form.allowTenantOverride : false,
      is_secret: form.isSecret
    }
    if (!form.isSecret || form.settingValue) payload.setting_value = parseValue()
    if (props.stored?.id) {
      await managementApi.updateResource('settings', String(props.stored.id), payload, {
        targetTenantId: props.targetTenantId
      })
    } else {
      if (form.isSecret && !form.settingValue) throw new Error('首次配置敏感值时不能为空')
      await managementApi.createResource('settings', payload, {
        targetTenantId: props.targetTenantId
      })
    }
    ElMessage.success('配置已保存')
    emit('saved')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '配置保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog
    :model-value="open"
    :title="title"
    width="min(520px, calc(100vw - 32px))"
    @close="emit('close')"
  >
    <el-form label-position="top">
      <el-form-item label="值类型">
        <el-select v-model="form.valueType" :disabled="Boolean(stored?.id)">
          <el-option label="字符串" value="string" />
          <el-option label="数字" value="number" />
          <el-option label="布尔" value="boolean" />
          <el-option label="JSON" value="json" />
        </el-select>
      </el-form-item>
      <el-form-item
        :label="form.isSecret && stored?.configured ? '配置值（留空保留现值）' : '配置值'"
      >
        <el-select v-if="form.valueType === 'boolean'" v-model="form.settingValue">
          <el-option label="是" value="true" />
          <el-option label="否" value="false" />
        </el-select>
        <el-input
          v-else
          v-model="form.settingValue"
          :type="form.isSecret ? 'password' : form.valueType === 'json' ? 'textarea' : 'text'"
          :rows="5"
          :show-password="form.isSecret"
          :autocomplete="form.isSecret ? 'new-password' : undefined"
        />
      </el-form-item>
      <el-form-item v-if="platformContext" label="覆盖策略">
        <el-switch v-model="form.allowTenantOverride" active-text="允许租户覆盖" />
      </el-form-item>
      <el-form-item label="敏感配置">
        <el-switch v-model="form.isSecret" active-text="加密保存且不回显" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('close')">取消</el-button>
      <el-button type="danger" :icon="Check" :loading="saving" @click="submit">保存配置</el-button>
    </template>
  </el-dialog>
</template>
