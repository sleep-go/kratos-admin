import { computed, ref, shallowRef } from 'vue'
import { defineStore } from 'pinia'

import * as authApi from '@/api/auth'
import { setAccessToken } from '@/api/http'
import type { CurrentUser, LoginRequest, TenantSummary } from '@/types/auth'
import type { AdminV1NavigationItem } from '@/api/generated'

function isPlatformRealm(user: CurrentUser | null | undefined) {
  return user?.realm === 'platform' && !user?.impersonating
}

export const useAuthStore = defineStore('auth', () => {
  const accessToken = shallowRef('')
  const currentUser = ref<CurrentUser | null>(null)
  const tenants = ref<TenantSummary[]>([])
  const currentTenant = ref<TenantSummary | null>(null)
  const navigationItems = ref<AdminV1NavigationItem[]>([])
  const loading = shallowRef(false)
  const sessionRestored = shallowRef(false)
  const isAuthenticated = computed(() => Boolean(accessToken.value && currentUser.value))
  const isImpersonating = computed(() => Boolean(currentUser.value?.impersonating))

  function applyAuthResponse(response: {
    accessToken?: string
    user?: CurrentUser
    tenants?: TenantSummary[]
    currentTenant?: TenantSummary
  }) {
    if (!response.accessToken || !response.user) {
      throw new Error('认证响应缺少访问令牌或用户资料')
    }
    accessToken.value = response.accessToken
    currentUser.value = response.user
    tenants.value = response.tenants ?? []
    currentTenant.value = response.currentTenant ?? response.tenants?.[0] ?? null
    setAccessToken(response.accessToken)
  }

  async function login(request: LoginRequest) {
    loading.value = true
    try {
      const response = await authApi.login(request)
      if (response.mfaRequired) {
        clearSession()
        return response
      }
      applyAuthResponse(response)
      await loadNavigation()
      sessionRestored.value = true
      return response
    } finally {
      loading.value = false
    }
  }

  async function platformLogin(request: LoginRequest) {
    loading.value = true
    try {
      const response = await authApi.platformLogin(request)
      if (response.mfaRequired) {
        clearSession()
        return response
      }
      applyAuthResponse(response)
      await loadNavigation()
      sessionRestored.value = true
      return response
    } finally {
      loading.value = false
    }
  }

  async function restoreSession(pathname = window.location.pathname) {
    if (sessionRestored.value) {
      return isAuthenticated.value
    }
    const usePlatformApi = pathname.startsWith('/platform')
    try {
      const response = usePlatformApi ? await authApi.platformRefresh() : await authApi.refresh()
      applyAuthResponse(response)
      await loadNavigation()
      return true
    } catch {
      if (!usePlatformApi) {
        try {
          const response = await authApi.platformRefresh()
          applyAuthResponse(response)
          await loadNavigation()
          return true
        } catch {
          clearSession()
          return false
        }
      }
      clearSession()
      return false
    } finally {
      sessionRestored.value = true
    }
  }

  async function renewSession() {
    const response = isPlatformRealm(currentUser.value)
      ? await authApi.platformRefresh()
      : await authApi.refresh()
    applyAuthResponse(response)
    await loadNavigation()
  }

  async function verifyMfa(challengeId: string, code: string) {
    loading.value = true
    try {
      const response = await authApi.verifyMfa({ challengeId, code })
      applyAuthResponse(response)
      await loadNavigation()
      sessionRestored.value = true
      return response
    } finally {
      loading.value = false
    }
  }

  async function switchTenant(tenantId: string) {
    loading.value = true
    try {
      const switched = await authApi.switchTenant(tenantId)
      applyAuthResponse(switched)
      await loadNavigation()
    } catch (error) {
      clearSession()
      throw error
    } finally {
      loading.value = false
      sessionRestored.value = true
    }
  }

  async function impersonateTenant(tenantId: string) {
    loading.value = true
    try {
      const response = await authApi.impersonate(tenantId)
      applyAuthResponse(response)
      await loadNavigation()
      sessionRestored.value = true
      return response
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      if (isPlatformRealm(currentUser.value)) {
        await authApi.platformLogout()
      } else {
        await authApi.logout()
      }
    } finally {
      clearSession()
      sessionRestored.value = false
    }
  }

  async function exitImpersonation() {
    try {
      const response = await authApi.exitImpersonation()
      applyAuthResponse(response)
      await loadNavigation()
      sessionRestored.value = true
    } catch (error) {
      clearSession()
      sessionRestored.value = false
      throw error
    }
  }

  function applyProfile(profile: CurrentUser) {
    currentUser.value = { ...currentUser.value, ...profile }
  }

  async function loadNavigation() {
    try {
      navigationItems.value = isPlatformRealm(currentUser.value)
        ? ((await authApi.platformListNavigation()).items ?? [])
        : ((await authApi.listNavigation()).items ?? [])
    } catch {
      navigationItems.value = []
    }
  }

  function clearSession() {
    accessToken.value = ''
    currentUser.value = null
    tenants.value = []
    currentTenant.value = null
    navigationItems.value = []
    setAccessToken(null)
  }

  return {
    accessToken,
    currentUser,
    tenants,
    currentTenant,
    navigationItems,
    loading,
    isAuthenticated,
    isImpersonating,
    sessionRestored,
    login,
    platformLogin,
    restoreSession,
    renewSession,
    verifyMfa,
    switchTenant,
    impersonateTenant,
    logout,
    exitImpersonation,
    applyProfile,
    loadNavigation,
    clearSession
  }
})
