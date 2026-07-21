<script setup lang="ts">
import { computed } from 'vue'

import { iconRegistry, type IconName } from './registry'

const props = withDefaults(
  defineProps<{
    name: IconName
    size?: number | string
  }>(),
  {
    size: 16
  }
)

const component = computed(() => iconRegistry[props.name])
</script>

<template>
  <el-icon class="app-icon" :size="size" aria-hidden="true">
    <component :is="component" />
  </el-icon>
</template>

<style scoped lang="scss">
.app-icon {
  flex-shrink: 0;
}
</style>
