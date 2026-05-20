import { post } from './client'
import type { LoginRequest, LoginResponse, UserDTO, ChangePasswordRequest } from '@/types'

export function login(req: LoginRequest) {
  return post<LoginResponse>('/auth/login', req)
}

export function me() {
  return post<{ user: UserDTO; permissions: string[] }>('/auth/me')
}

export function logout() {
  return post('/auth/logout')
}

export function changePassword(req: ChangePasswordRequest) {
  return post('/auth/change-password', req)
}
