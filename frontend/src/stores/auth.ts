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
  const isAuthenticated = computed(() => Boolean(accessToken.value && currentUser.value))

  async function login(request: LoginRequest) {
    loading.value = true
    try {
      const response = await authApi.login(request)
      accessToken.value = response.accessToken
      currentUser.value = response.user
      tenants.value = response.tenants
      currentTenant.value = response.currentTenant ?? response.tenants[0] ?? null
      setAccessToken(response.accessToken)
      return response
    } finally {
      loading.value = false
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
    login,
    logout,
    clearSession
  }
})
