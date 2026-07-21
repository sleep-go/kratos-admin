import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'

import { useAuthStore } from '@/stores/auth'

export const http = axios.create({
  baseURL: '/api/v1',
  timeout: 15_000,
  withCredentials: true
})

let refreshPromise: Promise<string> | null = null

async function renewAccessToken(): Promise<string> {
  if (!refreshPromise) {
    refreshPromise = (async () => {
      const authStore = useAuthStore()
      await authStore.renewSession()
      if (!authStore.accessToken) {
        throw new Error('刷新访问令牌失败')
      }
      return authStore.accessToken
    })().finally(() => {
      refreshPromise = null
    })
  }
  return refreshPromise
}

http.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const config = error.config as InternalAxiosRequestConfig & { _retried?: boolean }
    const status = error.response?.status
    const reason = (error.response?.data as { reason?: string } | undefined)?.reason
    const isAuthEndpoint = config?.url?.includes('/auth/') || config?.url?.includes('/platform/auth/')
    if (status !== 401 || config?._retried || isAuthEndpoint) {
      return Promise.reject(error)
    }
    if (reason !== 'AUTH_ACCESS_INVALID' && reason !== 'AUTH_ACCESS_STALE') {
      return Promise.reject(error)
    }
    try {
      const token = await renewAccessToken()
      config._retried = true
      config.headers.Authorization = `Bearer ${token}`
      return http.request(config)
    } catch {
      useAuthStore().clearSession()
      return Promise.reject(error)
    }
  }
)

export function setAccessToken(token: string | null) {
  if (token) {
    http.defaults.headers.common.Authorization = `Bearer ${token}`
    return
  }
  delete http.defaults.headers.common.Authorization
}
