import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { UserDTO } from '@/types'
import { login as apiLogin, me as apiMe, logout as apiLogout } from '@/api/auth'
import { sessionExpired } from '@/composables/useSessionExpired'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('salvo_token') || '')
  const user = ref<UserDTO | null>(null)
  const permissions = ref<string[]>([])
  const loading = ref(false)
  const validated = ref(false)

  const isLoggedIn = computed(() => !!token.value)
  const userRole = computed(() => user.value?.role_name || '')

  function hasPermission(perm: string): boolean {
    return permissions.value.includes(perm)
  }

  function canAccess(requiredPerms: string[]): boolean {
    if (userRole.value === 'admin') return true
    return requiredPerms.some((p) => permissions.value.includes(p))
  }

  async function login(email: string, password: string) {
    loading.value = true
    try {
      const resp = await apiLogin({ email, password })
      if (resp.code === 0) {
        token.value = resp.data.token
        user.value = resp.data.user
        localStorage.setItem('salvo_token', resp.data.token)
        localStorage.setItem('salvo_user', JSON.stringify(resp.data.user))
        const meResp = await apiMe()
        if (meResp.code === 0) {
          permissions.value = meResp.data.permissions
          localStorage.setItem('salvo_permissions', JSON.stringify(meResp.data.permissions))
        }
      }
      return resp
    } finally {
      loading.value = false
    }
  }

  async function fetchMe() {
    if (!token.value) return
    try {
      const resp = await apiMe()
      if (resp.code === 0) {
        user.value = resp.data.user
        permissions.value = resp.data.permissions
        localStorage.setItem('salvo_user', JSON.stringify(resp.data.user))
        localStorage.setItem('salvo_permissions', JSON.stringify(resp.data.permissions))
        validated.value = true
      }
    } catch {
      // Session expired: interceptor already set the flag.
      // Don't logout here — the global dialog handles redirect.
      if (sessionExpired.value) {
        validated.value = true
      } else {
        doLogout()
      }
    }
  }

  async function validateToken() {
    if (!token.value || validated.value) return
    await fetchMe()
  }

  function doLogout() {
    token.value = ''
    user.value = null
    permissions.value = []
    validated.value = false
    localStorage.removeItem('salvo_token')
    localStorage.removeItem('salvo_user')
    localStorage.removeItem('salvo_permissions')
    apiLogout().catch(() => {})
  }

  function initFromStorage() {
    const storedUser = localStorage.getItem('salvo_user')
    const storedPerms = localStorage.getItem('salvo_permissions')
    if (storedUser) {
      try { user.value = JSON.parse(storedUser) } catch { /* ignore */ }
    }
    if (storedPerms) {
      try { permissions.value = JSON.parse(storedPerms) } catch { /* ignore */ }
    }
  }

  return { token, user, permissions, loading, validated, isLoggedIn, userRole, hasPermission, canAccess, login, fetchMe, validateToken, doLogout, initFromStorage }
})
