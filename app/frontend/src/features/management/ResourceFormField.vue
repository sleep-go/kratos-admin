<script setup lang="ts">
import { computed } from 'vue'

import type { ResourceField, ResourceOption } from './resourceDefinitions'

const props = defineProps<{
  field: ResourceField
  options?: ResourceOption[]
  loading?: boolean
}>()

const model = defineModel<unknown>()
const defaultStatusOptions: ResourceOption[] = [
  { label: '启用', value: 1 },
  { label: '禁用', value: 2 }
]
const resolvedOptions = computed(() => {
  if (props.field.type === 'status') return props.field.options ?? defaultStatusOptions
  return props.field.options ?? props.options ?? []
})
const normalizedModel = computed({
  get() {
    if (props.field.multiple) {
      return Array.isArray(model.value) ? model.value : []
    }
    if (props.field.type === 'boolean') {
      return model.value === true || model.value === 1 || model.value === '1'
    }
    if (props.field.type === 'number' && model.value !== '' && model.value != null) {
      const value = Number(model.value)
      return Number.isFinite(value) ? value : model.value
    }
    if (['status', 'select', 'relation', 'tree'].includes(props.field.type ?? '')) {
      return findOption(resolvedOptions.value, model.value)?.value ?? model.value
    }
    return model.value
  },
  set(value) {
    model.value = value
  }
})

function findOption(options: ResourceOption[], value: unknown): ResourceOption | undefined {
  if (value === undefined || value === null) return undefined
  for (const option of options) {
    if (String(option.value) === String(value)) return option
    const child = findOption(option.children ?? [], value)
    if (child) return child
  }
  return undefined
}
</script>

<template>
  <el-switch v-if="field.type === 'boolean'" v-model="normalizedModel" />
  <el-select v-else-if="field.type === 'status'" v-model="normalizedModel">
    <el-option
      v-for="option in resolvedOptions"
      :key="String(option.value)"
      :label="option.label"
      :value="option.value"
    />
  </el-select>
  <el-select
    v-else-if="field.type === 'select' || field.type === 'relation'"
    v-model="normalizedModel"
    :filterable="field.type === 'relation'"
    :loading="loading"
    :multiple="field.multiple === true"
    :collapse-tags="field.multiple === true"
    :collapse-tags-tooltip="field.multiple === true"
  >
    <el-option
      v-for="option in resolvedOptions"
      :key="String(option.value)"
      :label="option.label"
      :value="option.value"
    />
  </el-select>
  <el-tree-select
    v-else-if="field.type === 'tree'"
    v-model="normalizedModel"
    :data="resolvedOptions"
    :loading="loading"
    check-strictly
    default-expand-all
    filterable
  />
  <el-input-number v-else-if="field.type === 'number'" v-model="normalizedModel" :min="0" />
  <el-input
    v-else
    v-model="model"
    :type="field.type === 'textarea' ? 'textarea' : field.type === 'password' ? 'password' : 'text'"
    :show-password="field.type === 'password'"
  />
</template>
