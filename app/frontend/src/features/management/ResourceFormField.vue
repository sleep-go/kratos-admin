<script setup lang="ts">
import type { ResourceField, ResourceOption } from './resourceDefinitions'

defineProps<{
  field: ResourceField
  options?: ResourceOption[]
  loading?: boolean
}>()

const model = defineModel<unknown>()
</script>

<template>
  <el-switch v-if="field.type === 'boolean'" v-model="model" />
  <el-select v-else-if="field.type === 'status'" v-model="model">
    <el-option label="启用" :value="1" />
    <el-option label="禁用" :value="2" />
  </el-select>
  <el-select
    v-else-if="field.type === 'select' || field.type === 'relation'"
    v-model="model"
    :filterable="field.type === 'relation'"
    :loading="loading"
  >
    <el-option
      v-for="option in field.options ?? options ?? []"
      :key="String(option.value)"
      :label="option.label"
      :value="option.value"
    />
  </el-select>
  <el-tree-select
    v-else-if="field.type === 'tree'"
    v-model="model"
    :data="options ?? []"
    :loading="loading"
    check-strictly
    default-expand-all
    filterable
  />
  <el-input-number v-else-if="field.type === 'number'" v-model="model" :min="0" />
  <el-input
    v-else
    v-model="model"
    :type="field.type === 'textarea' ? 'textarea' : field.type === 'password' ? 'password' : 'text'"
    :show-password="field.type === 'password'"
  />
</template>
