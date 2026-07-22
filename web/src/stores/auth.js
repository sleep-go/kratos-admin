import { defineStore } from 'pinia'
import { ref } from 'vue'
import { login as apiLogin, logout as apiLogout, getCurrent } from '../api/admin'

export const useAuthStore = defineStore('auth', () => {
  const user = ref(null)
  const loading = ref(false)

  async function login(username, password) {
    loading.value = true
    try {
      user.value = await apiLogin(username, password)
      return user.value
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      await apiLogout()
    } finally {
      user.value = null
    }
  }

  async function fetchCurrent() {
    try {
      user.value = await getCurrent()
      return user.value
    } catch {
      user.value = null
      return null
    }
  }

  return { user, loading, login, logout, fetchCurrent }
})
