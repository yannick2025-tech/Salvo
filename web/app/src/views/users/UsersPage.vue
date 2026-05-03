<template>
  <div class="users-page">
    <div class="page-header">
      <h2>用户管理</h2>
      <button class="btn-primary" @click="showCreate = true">+ 新建用户</button>
    </div>

    <div class="table-wrapper">
      <table class="data-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>邮箱</th>
            <th>昵称</th>
            <th>角色</th>
            <th>状态</th>
            <th>最后登录</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="users.length === 0"><td colspan="7" class="empty">暂无用户</td></tr>
          <tr v-for="u in users" :key="u.id">
            <td class="mono">{{ u.id }}</td>
            <td>{{ u.email }}</td>
            <td>{{ u.nickname || '-' }}</td>
            <td><span class="role-badge">{{ u.role_name }}</span></td>
            <td><span :class="['status-badge', u.status]">{{ u.status }}</span></td>
            <td>{{ formatTime(u.last_login_at) }}</td>
            <td class="actions">
              <button class="btn-sm" @click="editUser(u)">编辑</button>
              <button class="btn-sm warn" @click="resetUserPassword(u)">重置密码</button>
              <button v-if="u.role_name !== 'admin'" class="btn-sm danger" @click="handleDelete(u.id)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
      <div class="modal">
        <h3>{{ editingUser ? '编辑用户' : '新建用户' }}</h3>
        <div class="form-group">
          <label>邮箱</label>
          <input v-model="createForm.email" :disabled="!!editingUser" placeholder="user@example.com" />
        </div>
        <div class="form-group" v-if="!editingUser">
          <label>密码</label>
          <input v-model="createForm.password" type="password" placeholder="密码" />
        </div>
        <div class="form-group">
          <label>昵称</label>
          <input v-model="createForm.nickname" placeholder="昵称" />
        </div>
        <div class="form-group">
          <label>角色</label>
          <select v-model="createForm.role_id">
            <option :value="0">请选择角色</option>
            <option v-for="r in roles" :key="r.id" :value="r.id">{{ r.name }}</option>
          </select>
        </div>
        <div class="form-group" v-if="editingUser">
          <label>状态</label>
          <select v-model="createForm.status">
            <option value="active">启用</option>
            <option value="disabled">禁用</option>
          </select>
        </div>
        <div v-if="formError" class="form-error">{{ formError }}</div>
        <div class="modal-actions">
          <button class="btn-secondary" @click="closeModal">取消</button>
          <button class="btn-primary" @click="handleSave">{{ editingUser ? '保存' : '创建' }}</button>
        </div>
      </div>
    </div>

    <div v-if="showResetPwd" class="modal-overlay" @click.self="showResetPwd = false">
      <div class="modal">
        <h3>重置密码 - {{ resetTargetUser?.email }}</h3>
        <div class="form-group">
          <label>新密码</label>
          <input v-model="newPassword" type="password" placeholder="输入新密码" />
        </div>
        <div class="form-group">
          <label>确认密码</label>
          <input v-model="confirmPassword" type="password" placeholder="再次输入新密码" />
        </div>
        <div v-if="formError" class="form-error">{{ formError }}</div>
        <div class="modal-actions">
          <button class="btn-secondary" @click="showResetPwd = false">取消</button>
          <button class="btn-primary" @click="handleResetPassword">确认重置</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { listUsers, createUser, updateUser, deleteUser, resetPassword } from '@/api/user'
import { listRoles } from '@/api/user'
import type { UserDTO, RoleDTO } from '@/types'

const users = ref<UserDTO[]>([])
const roles = ref<RoleDTO[]>([])
const showCreate = ref(false)
const editingUser = ref<UserDTO | null>(null)
const createForm = reactive({ email: '', password: '', nickname: '', role_id: 0, status: 'active' })
const formError = ref('')
const showResetPwd = ref(false)
const resetTargetUser = ref<UserDTO | null>(null)
const newPassword = ref('')
const confirmPassword = ref('')

async function fetchUsers() {
  try {
    const resp = await listUsers({ limit: 100 })
    if (resp.code === 0) users.value = resp.data.items || []
  } catch { /* ignore */ }
}

async function fetchRoles() {
  try {
    const resp = await listRoles({ limit: 100 })
    if (resp.code === 0) roles.value = resp.data.items || []
  } catch { /* ignore */ }
}

function editUser(u: UserDTO) {
  editingUser.value = u
  createForm.email = u.email
  createForm.nickname = u.nickname
  createForm.role_id = u.role_id
  createForm.status = u.status
  showCreate.value = true
}

function closeModal() {
  showCreate.value = false
  editingUser.value = null
  createForm.email = ''
  createForm.password = ''
  createForm.nickname = ''
  createForm.role_id = 0
  createForm.status = 'active'
  formError.value = ''
}

async function handleSave() {
  formError.value = ''
  if (!createForm.email || (!editingUser.value && !createForm.password)) {
    formError.value = '邮箱和密码为必填项'
    return
  }
  const emailRe = /^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/
  if (!emailRe.test(createForm.email)) {
    formError.value = '邮箱格式不正确'
    return
  }
  if (!createForm.role_id || createForm.role_id === 0) {
    formError.value = '请选择角色'
    return
  }
  try {
    if (editingUser.value) {
      const resp = await updateUser({
        id: editingUser.value.id,
        nickname: createForm.nickname,
        role_id: createForm.role_id,
        status: createForm.status,
      })
      if (resp.code !== 0) {
        formError.value = resp.message || '更新失败'
        return
      }
    } else {
      const resp = await createUser({
        email: createForm.email,
        password: createForm.password,
        nickname: createForm.nickname,
        role_id: createForm.role_id,
      })
      if (resp.code !== 0) {
        formError.value = resp.message || '创建失败'
        return
      }
    }
    closeModal()
    fetchUsers()
  } catch (e: any) {
    formError.value = e.message || '操作失败'
  }
}

function resetUserPassword(u: UserDTO) {
  resetTargetUser.value = u
  newPassword.value = ''
  confirmPassword.value = ''
  formError.value = ''
  showResetPwd.value = true
}

async function handleResetPassword() {
  formError.value = ''
  if (!newPassword.value || newPassword.value.length < 6) {
    formError.value = '密码长度至少6位'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    formError.value = '两次输入的密码不一致'
    return
  }
  try {
    const resp = await resetPassword(resetTargetUser.value!.id, newPassword.value)
    if (resp.code === 0) {
      showResetPwd.value = false
    } else {
      formError.value = resp.message || '重置失败'
    }
  } catch (e: any) {
    formError.value = e.message || '重置失败'
  }
}

async function handleDelete(id: string) {
  if (!confirm('确定要删除该用户吗？此操作不可撤销。')) return
  await deleteUser(id)
  fetchUsers()
}

function formatTime(t?: string) {
  if (!t) return '-'
  return new Date(t).toLocaleString()
}

onMounted(() => {
  fetchUsers()
  fetchRoles()
})
</script>

<style scoped>
.users-page { display: flex; flex-direction: column; gap: 16px; }
.page-header { display: flex; justify-content: space-between; align-items: center; }
.page-header h2 { font-size: 18px; font-weight: 600; }
.btn-primary { padding: 8px 16px; border: none; border-radius: var(--radius-md); background: var(--accent-primary); color: #fff; font-size: 13px; cursor: pointer; }
.btn-secondary { padding: 8px 16px; border: 1px solid var(--border-primary); border-radius: var(--radius-md); background: transparent; color: var(--text-secondary); font-size: 13px; cursor: pointer; }
.btn-sm { padding: 4px 10px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm); background: transparent; color: var(--text-secondary); font-size: 12px; cursor: pointer; }
.btn-sm.danger { color: var(--accent-danger); border-color: var(--accent-danger); }
.btn-sm.warn { color: #f0ad4e; border-color: #f0ad4e; }
.form-error { font-size: 12px; color: var(--accent-danger, #e74c3c); background: rgba(248,81,73,0.1); padding: 6px 10px; border-radius: var(--radius-sm); margin-bottom: 8px; }

.table-wrapper { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); overflow: auto; }
.data-table { width: 100%; border-collapse: collapse; }
.data-table th, .data-table td { padding: 10px 14px; text-align: left; font-size: 13px; border-bottom: 1px solid var(--border-secondary); }
.data-table th { color: var(--text-secondary); font-weight: 500; background: var(--bg-tertiary); }
.empty { text-align: center; color: var(--text-tertiary); padding: 32px 0; }
.mono { font-family: monospace; font-size: 12px; }
.actions { display: flex; gap: 6px; }
.role-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; background: rgba(88,166,255,0.15); color: var(--accent-primary); }
.status-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; }
.status-badge.active { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.status-badge.disabled { background: rgba(139,148,158,0.15); color: var(--text-tertiary); }

.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--bg-card); border: 1px solid var(--border-primary); border-radius: var(--radius-lg); padding: 24px; width: 420px; }
.modal h3 { font-size: 16px; margin-bottom: 16px; }
.form-group { margin-bottom: 14px; }
.form-group label { display: block; font-size: 13px; color: var(--text-secondary); margin-bottom: 4px; }
.form-group input, .form-group select { width: 100%; height: 36px; padding: 0 10px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm); background: var(--bg-input); color: var(--text-primary); font-size: 13px; outline: none; }
.form-group input:disabled { opacity: 0.5; }
.form-group input:focus, .form-group select:focus { border-color: var(--accent-primary); }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 20px; }
</style>
