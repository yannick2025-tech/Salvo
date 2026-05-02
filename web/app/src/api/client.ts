import axios from 'axios'
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
    if (error.response?.status === 401) {
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

export default client
