<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { storeToRefs } from 'pinia'
import { RouterLink, RouterView, useRouter } from 'vue-router'
import { SwitchButton } from '@element-plus/icons-vue'

import AppIcon from '@/components/icons/AppIcon.vue'
import TopNavigation from './components/TopNavigation.vue'
import { buildNavigationTree } from '@/features/navigation/registry'
import { resolveNavigationIconName } from '@/components/icons/registry'
import type { NavigationItem } from '@/features/navigation/types'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const router = useRouter()
const { currentUser } = storeToRefs(authStore)
const mobileOpen = shallowRef(false)
const userName = computed(() => currentUser.value?.displayName ?? '平台管理员')
const navigation = computed<NavigationItem[]>(() => buildNavigationTree(authStore.navigationItems))

async function handleLogout() {
  await authStore.logout()
  mobileOpen.value = false
  await router.push('/platform/login')
}
</script>

<template>
  <div class="app-shell">
    <TopNavigation
      :items="navigation"
      tenant-name="平台治理"
      :user-name="userName"
      :show-tenant-switcher="false"
      account-path="/platform/account"
      @open-mobile="mobileOpen = true"
      @logout="handleLogout"
    />

    <div
      class="mobile-navigation"
      :class="{ 'mobile-navigation--open': mobileOpen }"
      :aria-hidden="!mobileOpen"
      data-testid="mobile-navigation"
    >
      <button
        class="mobile-close"
        type="button"
        aria-label="关闭导航菜单"
        @click="mobileOpen = false"
      >
        <AppIcon name="close" :size="22" />
      </button>
      <div class="mobile-account">
        <strong>{{ userName }}</strong>
        <span>平台治理</span>
      </div>
      <template v-for="item in navigation" :key="item.to">
        <section v-if="item.children?.length" class="mobile-navigation-group">
          <strong>
            <AppIcon
              v-if="resolveNavigationIconName(item.label)"
              :name="resolveNavigationIconName(item.label)!"
              :size="13"
            />
            {{ item.label }}
          </strong>
          <RouterLink
            v-for="child in item.children"
            :key="child.to"
            :to="child.to"
            class="mobile-link mobile-link--child"
            @click="mobileOpen = false"
          >
            {{ child.label }}
          </RouterLink>
        </section>
        <RouterLink v-else :to="item.to" class="mobile-link" @click="mobileOpen = false">
          {{ item.label }}
        </RouterLink>
      </template>
      <button class="mobile-logout" type="button" @click="handleLogout">
        <el-icon :size="15"><SwitchButton /></el-icon>
        退出登录
      </button>
    </div>
    <button
      v-if="mobileOpen"
      class="mobile-backdrop"
      type="button"
      aria-label="关闭导航菜单"
      @click="mobileOpen = false"
    ></button>

    <main class="page-content">
      <RouterView />
    </main>
  </div>
</template>

<style scoped lang="scss">
@import './shell.scss';
</style>
