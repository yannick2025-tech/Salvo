import axios, { type AxiosRequestConfig } from 'axios'
import type { ApiResponse } from '@/types'

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

client.interceptors.response.use(
  (resp) => resp,
  (error) => {
    const status = error.response?.status
    if (status === 401 || status === 403) {
      localStorage.removeItem('salvo_token')
      localStorage.removeItem('salvo_user')
      localStorage.removeItem('salvo_permissions')
      window.location.href = '/login'
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
