<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'

export interface NavigationItem {
  label: string
  to: string
}

const props = defineProps<{
  items: NavigationItem[]
  tenantName: string
  userName: string
}>()

const initials = computed(() => props.userName.trim().slice(0, 1) || '管')

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
      <RouterLink v-for="item in items" :key="item.to" :to="item.to" class="navigation-link">
        {{ item.label }}
      </RouterLink>
    </nav>

    <div class="account-area">
      <button
        class="tenant-switcher"
        data-testid="tenant-switcher"
        type="button"
        @click="emit('switchTenant')"
      >
        {{ tenantName }}
        <span aria-hidden="true">⌄</span>
      </button>
      <span class="account-divider" aria-hidden="true"></span>
      <button class="account-button" type="button" @click="emit('logout')">
        <span class="avatar">{{ initials }}</span>
        <span class="account-name">{{ userName }}</span>
      </button>
    </div>

    <button
      class="mobile-menu-button"
      data-testid="mobile-menu-button"
      type="button"
      aria-label="打开导航菜单"
      @click="emit('openMobile')"
    >
      <span></span><span></span><span></span>
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
  padding: 0 3px;
  color: rgb(255 255 255 / 84%);
  font-size: 14px;
  font-weight: 650;
  text-decoration: none;
  transition: color 180ms ease;

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
  &.router-link-active {
    color: #fff;
  }

  &.router-link-active::after {
    transform: scaleX(1);
  }
}

.account-area {
  display: flex;
  align-items: center;
  gap: 12px;
}

.tenant-switcher,
.account-button,
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

  span {
    display: block;
    height: 2px;
    margin: 4px 0;
    background: #fff;
  }
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
    display: block;
  }
}
</style>
