import { computed, ref, shallowRef } from 'vue'
import { defineStore } from 'pinia'

import * as authApi from '@/api/auth'
import { setAccessToken } from '@/api/http'
import type { CurrentUser, LoginRequest, TenantSummary } from '@/types/auth'

export const useAuthStore = defineStore('auth', () => {
  const accessToken = shallowRef('')
  const currentUser = ref<CurrentUser | null>(null)
  const tenants = ref<TenantSummary[]>([])
  const currentTenant = ref<TenantSummary | null>(null)
  const loading = shallowRef(false)
  const sessionRestored = shallowRef(false)
  const isAuthenticated = computed(() => Boolean(accessToken.value && currentUser.value))

  async function login(request: LoginRequest) {
    loading.value = true
    try {
      const response = await authApi.login(request)
      if (response.mfaRequired) {
        clearSession()
        return response
      }
      if (!response.accessToken || !response.user) {
        throw new Error('登录响应缺少访问令牌或用户资料')
      }
      accessToken.value = response.accessToken
      currentUser.value = response.user
      tenants.value = response.tenants ?? []
      currentTenant.value = response.currentTenant ?? response.tenants?.[0] ?? null
      setAccessToken(response.accessToken)
      sessionRestored.value = true
      return response
    } finally {
      loading.value = false
    }
  }

  async function restoreSession() {
    if (sessionRestored.value) {
      return isAuthenticated.value
    }
    try {
      const response = await authApi.refresh()
      if (!response.accessToken || !response.user) {
        throw new Error('刷新响应缺少访问令牌或用户资料')
      }
      accessToken.value = response.accessToken
      currentUser.value = response.user
      tenants.value = response.tenants ?? []
      currentTenant.value = response.currentTenant ?? response.tenants?.[0] ?? null
      setAccessToken(response.accessToken)
      return true
    } catch {
      clearSession()
      return false
    } finally {
      sessionRestored.value = true
    }
  }

  async function verifyMfa(challengeId: string, code: string) {
    loading.value = true
    try {
      const response = await authApi.verifyMfa({ challengeId, code })
      if (!response.accessToken || !response.user) {
        throw new Error('MFA响应缺少访问令牌或用户资料')
      }
      accessToken.value = response.accessToken
      currentUser.value = response.user
      tenants.value = response.tenants ?? []
      currentTenant.value = response.currentTenant ?? response.tenants?.[0] ?? null
      setAccessToken(response.accessToken)
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
      if (!switched.accessToken) throw new Error('租户切换响应缺少访问令牌')
      accessToken.value = switched.accessToken
      setAccessToken(switched.accessToken)
      const response = await authApi.refresh()
      if (!response.accessToken || !response.user) {
        throw new Error('切换后刷新响应缺少身份上下文')
      }
      accessToken.value = response.accessToken
      currentUser.value = response.user
      tenants.value = response.tenants ?? []
      currentTenant.value = response.currentTenant ?? switched.currentTenant ?? null
      setAccessToken(response.accessToken)
    } catch (error) {
      clearSession()
      throw error
    } finally {
      loading.value = false
      sessionRestored.value = true
    }
  }

  async function logout() {
    try {
      await authApi.logout()
    } finally {
      clearSession()
    }
  }

  function clearSession() {
    accessToken.value = ''
    currentUser.value = null
    tenants.value = []
    currentTenant.value = null
    setAccessToken(null)
  }

  return {
    accessToken,
    currentUser,
    tenants,
    currentTenant,
    loading,
    isAuthenticated,
    sessionRestored,
    login,
    restoreSession,
    verifyMfa,
    switchTenant,
    logout,
    clearSession
  }
})
