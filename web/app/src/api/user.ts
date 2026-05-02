import { post } from './client'
import type { UserDTO, RoleDTO, ListResponse, CreateUserRequest, UpdateUserRequest } from '@/types'

export function listUsers(params?: { status?: string; offset?: number; limit?: number }) {
  return post<ListResponse<UserDTO>>('/users/list', params)
}

export function createUser(req: CreateUserRequest) {
  return post<UserDTO>('/users/create', req)
}

export function updateUser(req: UpdateUserRequest) {
  return post<UserDTO>('/users/update', req)
}

export function deleteUser(id: number) {
  return post('/users/delete', { id })
}

export function listRoles(params?: { offset?: number; limit?: number }) {
  return post<ListResponse<RoleDTO>>('/roles/list', params)
}

export function createRole(req: { name: string; description?: string }) {
  return post<RoleDTO>('/roles/create', req)
}

export function updateRole(req: { id: string; name?: string; description?: string; permissions?: string[] }) {
  return post<RoleDTO>('/roles/update', req)
}

export function deleteRole(id: string) {
  return post('/roles/delete', { id })
}

export function resetPassword(userId: number, newPassword: string) {
  return post('/auth/reset-password', { user_id: userId, new_password: newPassword })
}
