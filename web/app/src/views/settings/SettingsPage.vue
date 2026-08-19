<template>
  <div class="settings-page">
    <div class="page-header">
      <h2>个人设置</h2>
    </div>

    <div class="settings-grid">
      <div class="card">
        <h3>修改密码</h3>
        <div class="form-group">
          <label>当前密码</label>
          <input v-model="pwdForm.old_password" type="password" placeholder="当前密码" />
        </div>
        <div class="form-group">
          <label>新密码</label>
          <input v-model="pwdForm.new_password" type="password" placeholder="新密码" />
        </div>
        <div class="form-group">
          <label>确认新密码</label>
          <input v-model="pwdForm.confirm_password" type="password" placeholder="确认新密码" />
        </div>
        <button class="btn-primary" @click="handleChangePassword">修改密码</button>
        <div v-if="pwdMsg" :class="['msg', pwdMsgType]">{{ pwdMsg }}</div>
      </div>

      <div class="card">
        <h3>个人信息</h3>
        <div class="info-row"><span class="label">邮箱</span><span class="value">{{ authStore.user?.email }}</span></div>
        <div class="info-row"><span class="label">昵称</span><span class="value">{{ authStore.user?.nickname || '-' }}</span></div>
        <div class="info-row"><span class="label">角色</span><span class="value role-badge">{{ authStore.userRole }}</span></div>
        <div class="info-row"><span class="label">最后登录</span><span class="value">{{ formatTime(authStore.user?.last_login_at) }}</span></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { changePassword } from '@/api/auth'

const authStore = useAuthStore()

const pwdForm = reactive({ old_password: '', new_password: '', confirm_password: '' })
const pwdMsg = ref('')
const pwdMsgType = ref('')

async function handleChangePassword() {
  pwdMsg.value = ''
  if (pwdForm.new_password !== pwdForm.confirm_password) {
    pwdMsg.value = '两次输入的密码不一致'
    pwdMsgType.value = 'error'
    return
  }
  try {
    const resp = await changePassword({
      old_password: pwdForm.old_password,
      new_password: pwdForm.new_password,
    })
    if (resp.code === 0) {
      pwdMsg.value = '密码修改成功'
      pwdMsgType.value = 'success'
      pwdForm.old_password = ''
      pwdForm.new_password = ''
      pwdForm.confirm_password = ''
    } else {
      pwdMsg.value = resp.message || '修改失败'
      pwdMsgType.value = 'error'
    }
  } catch {
    pwdMsg.value = '网络错误'
    pwdMsgType.value = 'error'
  }
}

function formatTime(t?: string) {
  if (!t) return '-'
  const d = new Date(t)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
</script>

<style scoped>
.settings-page { display: flex; flex-direction: column; gap: 16px; }
.page-header h2 { font-size: 18px; font-weight: 600; }
.settings-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.card { background: var(--bg-card); border: 1px solid var(--border-secondary); border-radius: var(--radius-md); padding: 20px; }
.card h3 { font-size: 14px; font-weight: 600; margin-bottom: 16px; }
.form-group { margin-bottom: 14px; }
.form-group label { display: block; font-size: 13px; color: var(--text-secondary); margin-bottom: 4px; }
.form-group input { width: 100%; height: 36px; padding: 0 10px; border: 1px solid var(--border-primary); border-radius: var(--radius-sm); background: var(--bg-input); color: var(--text-primary); font-size: 13px; outline: none; }
.form-group input:focus { border-color: var(--accent-primary); }
.msg { margin-top: 12px; font-size: 13px; padding: 8px 12px; border-radius: var(--radius-sm); }
.msg.success { background: rgba(63,185,80,0.15); color: var(--accent-success); }
.msg.error { background: rgba(248,81,73,0.15); color: var(--accent-danger); }
.info-row { display: flex; align-items: center; padding: 10px 0; border-bottom: 1px solid var(--border-secondary); }
.info-row:last-child { border-bottom: none; }
.label { width: 100px; font-size: 13px; color: var(--text-secondary); flex-shrink: 0; }
.value { font-size: 14px; color: var(--text-primary); }
.role-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; background: rgba(88,166,255,0.15); color: var(--accent-primary); }
</style>
