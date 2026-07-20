<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { storeToRefs } from 'pinia'
import { RouterLink, RouterView } from 'vue-router'

import TopNavigation, { type NavigationItem } from './components/TopNavigation.vue'
import { useAuthStore } from '@/stores/auth'

const navigation: NavigationItem[] = [
  { label: '工作台', to: '/' },
  { label: '平台管理', to: '/platform/tenants' },
  { label: '组织管理', to: '/organization/users' },
  { label: '权限中心', to: '/permission/roles' },
  { label: '日志中心', to: '/logs/audit' },
  { label: '文件管理', to: '/files' },
  { label: '系统设置', to: '/settings' }
]

const authStore = useAuthStore()
const { currentTenant, currentUser } = storeToRefs(authStore)
const mobileOpen = shallowRef(false)
const tenantName = computed(() => currentTenant.value?.name ?? '请选择租户')
const userName = computed(() => currentUser.value?.displayName ?? '超级管理员')
</script>

<template>
  <div class="app-shell">
    <TopNavigation
      :items="navigation"
      :tenant-name="tenantName"
      :user-name="userName"
      @open-mobile="mobileOpen = true"
      @logout="authStore.logout()"
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
        ×
      </button>
      <div class="mobile-account">
        <strong>{{ userName }}</strong>
        <span>{{ tenantName }}</span>
      </div>
      <RouterLink
        v-for="item in navigation"
        :key="item.to"
        :to="item.to"
        class="mobile-link"
        @click="mobileOpen = false"
      >
        {{ item.label }}
      </RouterLink>
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
.app-shell {
  min-height: 100vh;
  background: var(--ka-page-bg);
}

.page-content {
  width: min(1440px, calc(100% - 48px));
  margin: 0 auto;
  padding: 34px 0 48px;
}

.mobile-navigation {
  position: fixed;
  z-index: 40;
  top: 0;
  right: 0;
  bottom: 0;
  width: min(84vw, 340px);
  padding: 24px;
  color: #fff;
  background: linear-gradient(150deg, #555, #252525);
  box-shadow: -12px 0 36px rgb(0 0 0 / 25%);
  transform: translateX(105%);
  transition: transform 220ms ease;

  &--open {
    transform: translateX(0);
  }
}

.mobile-close {
  float: right;
  border: 0;
  color: #fff;
  background: transparent;
  font-size: 28px;
}

.mobile-account {
  display: grid;
  gap: 4px;
  padding: 42px 8px 20px;
  border-bottom: 1px solid rgb(255 255 255 / 18%);

  span {
    color: rgb(255 255 255 / 62%);
    font-size: 12px;
  }
}

.mobile-link {
  display: block;
  padding: 15px 8px;
  border-bottom: 1px solid rgb(255 255 255 / 10%);
  color: rgb(255 255 255 / 78%);
  text-decoration: none;

  &.router-link-active {
    border-left: 3px solid var(--ka-accent);
    color: #fff;
    background: rgb(255 255 255 / 5%);
  }
}

.mobile-backdrop {
  position: fixed;
  z-index: 30;
  inset: 0;
  border: 0;
  background: rgb(0 0 0 / 38%);
}

@media (min-width: 1041px) {
  .mobile-navigation {
    display: none;
  }
}

@media (max-width: 640px) {
  .page-content {
    width: calc(100% - 28px);
    padding-top: 22px;
  }
}
</style>
