import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { UserDTO } from '@/types'
import { login as apiLogin, me as apiMe, logout as apiLogout } from '@/api/auth'
import { sessionExpired } from '@/composables/useSessionExpired'
import { setLoggedIn } from '@/api/client'

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
      console.log('[auth] login: calling apiLogin for', email)
      const resp = await apiLogin({ email, password })
      console.log('[auth] login: apiLogin response', resp.code, resp.message)
      if (resp.code === 0) {
        token.value = resp.data.token
        user.value = resp.data.user
        localStorage.setItem('salvo_token', resp.data.token)
        localStorage.setItem('salvo_user', JSON.stringify(resp.data.user))
        setLoggedIn(true)
        console.log('[auth] login: token stored, calling apiMe')
        const meResp = await apiMe()
        console.log('[auth] login: apiMe response', meResp.code, meResp.message)
        if (meResp.code === 0) {
          permissions.value = meResp.data.permissions
          localStorage.setItem('salvo_permissions', JSON.stringify(meResp.data.permissions))
          console.log('[auth] login: permissions stored', permissions.value.length, 'permissions')
        }
      }
      return resp
    } finally {
      loading.value = false
    }
  }

  async function fetchMe() {
    if (!token.value) {
      console.log('[auth] fetchMe: no token, skip')
      return
    }
    try {
      console.log('[auth] fetchMe: calling apiMe')
      const resp = await apiMe()
      console.log('[auth] fetchMe: apiMe response', resp.code, resp.message)
      if (resp.code === 0) {
        user.value = resp.data.user
        permissions.value = resp.data.permissions
        localStorage.setItem('salvo_user', JSON.stringify(resp.data.user))
        localStorage.setItem('salvo_permissions', JSON.stringify(resp.data.permissions))
        validated.value = true
        console.log('[auth] fetchMe: success, validated=true')
      }
    } catch (err) {
      console.warn('[auth] fetchMe: error', err)
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
    // Don't call setLoggedIn(true) here — wasLoggedIn should only be true
    // after an explicit login() call. This ensures that a stale token
    // from a previous session (e.g., after DB rebuild) fails silently
    // and redirects to login without showing the "会话已过期" dialog.
  }

  return { token, user, permissions, loading, validated, isLoggedIn, userRole, hasPermission, canAccess, login, fetchMe, validateToken, doLogout, initFromStorage }
})
