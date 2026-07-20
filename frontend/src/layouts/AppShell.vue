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
const tenantDialogOpen = shallowRef(false)
const tenantName = computed(() => currentTenant.value?.name ?? '请选择租户')
const userName = computed(() => currentUser.value?.displayName ?? '超级管理员')

async function selectTenant(tenantID: string) {
  await authStore.switchTenant(tenantID)
  tenantDialogOpen.value = false
}
</script>

<template>
  <div class="app-shell">
    <TopNavigation
      :items="navigation"
      :tenant-name="tenantName"
      :user-name="userName"
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
            ×
          </button>
        </header>
        <button
          v-if="currentUser?.platformAdmin"
          type="button"
          :class="{ active: String(currentTenant?.id ?? '0') === '0' }"
          @click="selectTenant('0')"
        >
          <strong>平台管理</strong><span>跨租户治理视角</span>
        </button>
        <button
          v-for="tenant in authStore.tenants"
          :key="String(tenant.id)"
          type="button"
          :class="{ active: String(currentTenant?.id) === String(tenant.id) }"
          @click="selectTenant(String(tenant.id))"
        >
          <strong>{{ tenant.name }}</strong
          ><span>租户 ID {{ tenant.id }}</span>
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

.tenant-dialog {
  position: fixed;
  z-index: 60;
  inset: 0;
  display: grid;
  place-items: center;
}

.tenant-dialog__backdrop {
  position: absolute;
  inset: 0;
  border: 0;
  background: rgb(0 0 0 / 42%);
}

.tenant-dialog section {
  position: relative;
  width: min(440px, calc(100% - 32px));
  max-height: min(620px, calc(100vh - 48px));
  padding: 24px;
  overflow-y: auto;
  background: #fff;
  box-shadow: 0 22px 70px rgb(0 0 0 / 28%);
}

.tenant-dialog header {
  display: flex;
  align-items: start;
  justify-content: space-between;
  margin-bottom: 20px;

  small {
    color: var(--ka-accent);
    font-weight: 800;
    letter-spacing: 0.14em;
  }
  h2 {
    margin: 5px 0 0;
    font-size: 25px;
  }
  button {
    border: 0;
    background: transparent;
    font-size: 24px;
    cursor: pointer;
  }
}

.tenant-dialog section > button {
  width: 100%;
  display: grid;
  gap: 4px;
  padding: 15px 16px;
  border: 1px solid var(--ka-border);
  color: var(--ka-text);
  background: #fff;
  text-align: left;
  cursor: pointer;

  & + button {
    margin-top: 10px;
  }
  &.active {
    border-left: 3px solid var(--ka-accent);
    background: #f7f7f7;
  }
  span {
    color: var(--ka-muted);
    font-size: 12px;
  }
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
