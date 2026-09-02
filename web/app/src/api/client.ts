import axios, { type AxiosRequestConfig } from 'axios'
import type { ApiResponse } from '@/types'
import { sessionExpired } from '@/composables/useSessionExpired'
import './fetchInterceptor'

/** Whether the user was actively logged in before this request. */
let wasLoggedIn = false

const client = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
})

client.interceptors.request.use((config) => {
  const token = localStorage.getItem('salvo_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

/** Mark session as expired and clear stored credentials. */
function handleSessionExpired() {
  if (!sessionExpired.value) {
    // Only show the "会话已过期" dialog if the user was actively logged in.
    // If they're just loading the page with a stale token, silently clear it.
    sessionExpired.value = wasLoggedIn
    localStorage.removeItem('salvo_token')
    localStorage.removeItem('salvo_user')
    localStorage.removeItem('salvo_permissions')
  }
}

/** Track whether the user is actively logged in. */
export function setLoggedIn(loggedIn: boolean) {
  wasLoggedIn = loggedIn
}

client.interceptors.response.use(
  (resp) => {
    // Backend returns HTTP 200 with body {"code":401} for expired tokens.
    if (resp.data?.code === 401) {
      console.warn('[client] 401 from backend:', resp.config?.url, resp.data?.message)
      handleSessionExpired()
      return Promise.reject(new Error(resp.data?.message || 'invalid or expired token'))
    }
    if (resp.data?.code === 403) {
      console.warn('[client] 403 from backend:', resp.config?.url, resp.data?.message)
    }
    return resp
  },
  (error) => {
    const status = error.response?.status
    if (status === 401 || status === 403) {
      console.warn('[client] HTTP', status, 'for', error.config?.url, error.response?.data)
      handleSessionExpired()
    }
    return Promise.reject(error)
  }
)

export async function post<T = any>(url: string, data?: any): Promise<ApiResponse<T>> {
  const resp = await client.post<ApiResponse<T>>(url, data)
  return resp.data
}

export async function get<T = any>(url: string, config?: AxiosRequestConfig): Promise<T> {
  const resp = await client.get<T>(url, config)
  return resp.data
}

export default client
