<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { storeToRefs } from 'pinia'
import { RouterLink, RouterView, useRouter } from 'vue-router'
import { SwitchButton } from '@element-plus/icons-vue'

import AppIcon from '@/components/icons/AppIcon.vue'
import TopNavigation from './components/TopNavigation.vue'
import { resolveNavigation, resolveTopNavigation } from '@/features/navigation/registry'
import { resolveNavigationIconName } from '@/components/icons/registry'
import type { NavigationItem } from '@/features/navigation/types'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const router = useRouter()
const { currentTenant, currentUser } = storeToRefs(authStore)
const mobileOpen = shallowRef(false)
const tenantDialogOpen = shallowRef(false)
const tenantName = computed(() => currentTenant.value?.name ?? '请选择租户')
const userName = computed(() => currentUser.value?.displayName ?? '租户管理员')
const navigation = computed<NavigationItem[]>(() =>
  resolveTopNavigation(resolveNavigation(authStore.navigationItems), 'console')
)

function openMobileTenantDialog() {
  mobileOpen.value = false
  tenantDialogOpen.value = true
}

async function selectTenant(tenantID: string) {
  await authStore.switchTenant(tenantID)
  tenantDialogOpen.value = false
  await router.push('/console')
}

async function exitImpersonation() {
  await authStore.exitImpersonation()
  await router.push('/platform/login')
}
</script>

<template>
  <div class="app-shell">
    <div v-if="currentUser?.impersonating" class="impersonation-banner" data-testid="impersonation-banner">
      <span>当前处于代维模式：{{ tenantName }}</span>
      <button type="button" @click="exitImpersonation">退出代维</button>
    </div>

    <TopNavigation
      :items="navigation"
      :tenant-name="tenantName"
      :user-name="userName"
      :show-tenant-switcher="!currentUser?.impersonating && authStore.tenants.length > 1"
      account-path="/console/account"
      @open-mobile="mobileOpen = true"
      @switch-tenant="tenantDialogOpen = true"
      @logout="authStore.logout()"
    />

    <div v-if="tenantDialogOpen" class="tenant-dialog" role="dialog" aria-label="切换租户">
      <button
        class="tenant-dialog__backdrop"
        aria-label="关闭租户切换"
        @click="tenantDialogOpen = false"
      ></button>
      <section>
        <header>
          <div>
            <small>TENANT CONTEXT</small>
            <h2>切换租户</h2>
          </div>
          <button type="button" aria-label="关闭租户切换" @click="tenantDialogOpen = false">
            <AppIcon name="close" :size="18" />
          </button>
        </header>
        <button
          v-for="tenant in authStore.tenants"
          :key="String(tenant.id)"
          type="button"
          :class="{ active: String(currentTenant?.id) === String(tenant.id) }"
          @click="selectTenant(String(tenant.id))"
        >
          <strong>{{ tenant.name }}</strong><span>租户 ID {{ tenant.id }}</span>
        </button>
      </section>
    </div>

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
        <span>{{ tenantName }}</span>
        <button
          v-if="!currentUser?.impersonating && authStore.tenants.length > 1"
          type="button"
          data-testid="mobile-tenant-switcher"
          @click="openMobileTenantDialog"
        >
          <AppIcon name="switch" :size="14" />
          切换租户
        </button>
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
          <AppIcon
            v-if="resolveNavigationIconName(item.label)"
            :name="resolveNavigationIconName(item.label)!"
            :size="15"
          />
          {{ item.label }}
        </RouterLink>
      </template>
      <RouterLink to="/console/account" class="mobile-link" @click="mobileOpen = false">
        <AppIcon name="account" :size="15" />
        个人中心
      </RouterLink>
      <button class="mobile-logout" type="button" @click="authStore.logout()">
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

.impersonation-banner {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding: 10px 16px;
  color: #fff;
  background: #b42318;
  font-size: 13px;
  font-weight: 600;

  button {
    border: 1px solid rgb(255 255 255 / 70%);
    padding: 4px 12px;
    color: #fff;
    background: transparent;
    cursor: pointer;
  }
}
</style>
