<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { Menu } from '@element-plus/icons-vue'

import AppIcon from '@/components/icons/AppIcon.vue'
import { SwitchButton } from '@/components/icons/actions'
import { resolveNavigationIconName } from '@/components/icons/registry'
import type { NavigationItem } from '@/features/navigation/types'

const props = withDefaults(
  defineProps<{
    items: NavigationItem[]
    tenantName: string
    userName: string
    showTenantSwitcher?: boolean
    accountPath?: string
  }>(),
  {
    showTenantSwitcher: true,
    accountPath: '/console/account'
  }
)

const initials = computed(() => props.userName.trim().slice(0, 1) || '管')
const route = useRoute()

function isActive(item: NavigationItem) {
  if (item.to === '/console') return route.path === '/console' || route.path === '/console/'
  return item.children?.some((child) => route.path === child.to) ?? route.path === item.to
}

const emit = defineEmits<{
  openMobile: []
  switchTenant: []
  logout: []
}>()
</script>

<template>
  <header class="top-navigation">
    <div class="brand" data-testid="brand">
      <span class="brand-mark">KRATOS</span>
      <span class="brand-subtitle">Administration System</span>
    </div>

    <nav class="desktop-navigation" aria-label="主导航">
      <template v-for="item in items" :key="item.to">
        <div
          v-if="item.children?.length"
          class="navigation-group"
          :data-testid="`navigation-group-${item.label}`"
        >
          <button
            type="button"
            class="navigation-link navigation-group__trigger"
            :class="{ 'navigation-link--active': isActive(item) }"
            aria-haspopup="menu"
          >
            <AppIcon
              v-if="resolveNavigationIconName(item.label)"
              :name="resolveNavigationIconName(item.label)!"
              :size="15"
            />
            {{ item.label }}
            <AppIcon name="arrow-down" :size="12" />
          </button>
          <div class="navigation-submenu" role="menu">
            <RouterLink
              v-for="child in item.children"
              :key="child.to"
              :to="child.to"
              role="menuitem"
            >
              {{ child.label }}
            </RouterLink>
          </div>
        </div>
        <RouterLink v-else :to="item.to" class="navigation-link">
          <AppIcon
            v-if="resolveNavigationIconName(item.label)"
            :name="resolveNavigationIconName(item.label)!"
            :size="15"
          />
          {{ item.label }}
        </RouterLink>
      </template>
    </nav>

    <div class="account-area">
      <button
        v-if="showTenantSwitcher"
        class="tenant-switcher"
        data-testid="tenant-switcher"
        type="button"
        @click="emit('switchTenant')"
      >
        <AppIcon name="switch" :size="14" />
        {{ tenantName }}
        <AppIcon name="arrow-down" :size="12" />
      </button>
      <span v-if="showTenantSwitcher" class="account-divider" aria-hidden="true"></span>
      <RouterLink class="account-button" :to="accountPath" aria-label="打开个人中心">
        <span class="avatar">{{ initials }}</span>
        <span class="account-name">{{ userName }}</span>
      </RouterLink>
      <button class="logout-button" type="button" @click="emit('logout')">
        <el-icon :size="14"><SwitchButton /></el-icon>
        退出
      </button>
    </div>

    <button
      class="mobile-menu-button"
      data-testid="mobile-menu-button"
      type="button"
      aria-label="打开导航菜单"
      @click="emit('openMobile')"
    >
      <el-icon :size="22"><Menu /></el-icon>
    </button>
  </header>
</template>

<style scoped lang="scss">
.top-navigation {
  min-height: 64px;
  display: flex;
  align-items: stretch;
  padding: 0 clamp(18px, 3vw, 44px);
  color: #fff;
  background: linear-gradient(105deg, #8c8c8c 0%, #555 42%, #272727 100%);
  box-shadow: 0 4px 18px rgb(0 0 0 / 16%);
}

.brand {
  min-width: 220px;
  display: flex;
  align-items: center;
  gap: 9px;
}

.brand-mark {
  font-family: 'Arial Black', 'Noto Sans SC', sans-serif;
  font-size: 23px;
  letter-spacing: -1.5px;
}

.brand-subtitle {
  max-width: 88px;
  font-size: 9px;
  line-height: 1.25;
  opacity: 0.82;
}

.desktop-navigation {
  display: flex;
  justify-content: center;
  flex: 1;
  gap: clamp(24px, 3vw, 52px);
}

.navigation-link {
  position: relative;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 3px;
  color: rgb(255 255 255 / 84%);
  font-size: 14px;
  font-weight: 650;
  text-decoration: none;
  transition: color 180ms ease;
  border: 0;
  background: transparent;
  cursor: pointer;

  &::after {
    position: absolute;
    right: 0;
    bottom: 0;
    left: 0;
    height: 3px;
    background: #ed1515;
    content: '';
    transform: scaleX(0);
    transition: transform 180ms ease;
  }

  &:hover,
  &.router-link-active,
  &.navigation-link--active {
    color: #fff;
  }

  &.router-link-active::after,
  &.navigation-link--active::after {
    transform: scaleX(1);
  }
}

.navigation-group {
  position: relative;
  display: flex;
}

.navigation-group__trigger {
  font-family: inherit;
}

.navigation-submenu {
  position: absolute;
  z-index: 50;
  top: calc(100% - 1px);
  left: 50%;
  min-width: 168px;
  padding: 8px 0;
  visibility: hidden;
  background: #fff;
  box-shadow: 0 14px 34px rgb(0 0 0 / 20%);
  opacity: 0;
  transform: translate(-50%, 8px);
  transition:
    opacity 160ms ease,
    transform 160ms ease,
    visibility 160ms ease;

  a {
    display: block;
    padding: 11px 18px;
    color: #444;
    font-size: 13px;
    text-decoration: none;

    &:hover,
    &.router-link-active {
      color: var(--ka-accent);
      background: #f6f6f6;
    }
  }
}

.navigation-group:hover .navigation-submenu,
.navigation-group:focus-within .navigation-submenu {
  visibility: visible;
  opacity: 1;
  transform: translate(-50%, 0);
}

.account-area {
  display: flex;
  align-items: center;
  gap: 12px;
}

.tenant-switcher,
.account-button,
.logout-button,
.mobile-menu-button {
  border: 0;
  color: inherit;
  background: transparent;
  cursor: pointer;
}

.tenant-switcher,
.account-button {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 12px;
}

.account-button {
  color: inherit;
  text-decoration: none;
}

.logout-button {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  opacity: 0.78;
}

.account-divider {
  width: 1px;
  height: 18px;
  background: rgb(255 255 255 / 62%);
}

.avatar {
  width: 29px;
  height: 29px;
  display: grid;
  place-items: center;
  border: 1px solid rgb(255 255 255 / 80%);
  border-radius: 50%;
  font-size: 12px;
}

.mobile-menu-button {
  display: none;
  width: 40px;
  padding: 10px;
  place-items: center;
  grid-template-columns: 1fr;
}

@media (max-width: 1040px) {
  .desktop-navigation,
  .account-area {
    display: none;
  }

  .brand {
    flex: 1;
  }

  .mobile-menu-button {
    display: grid;
  }
}
</style>
