<template>
  <router-view />

  <!-- Session expired dialog -->
  <div v-if="showSessionDialog" class="session-overlay">
    <div class="session-dialog">
      <div class="session-icon">
        <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="8" x2="12" y2="12"/>
          <line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
      </div>
      <h3 class="session-title">会话已过期</h3>
      <p class="session-msg">您的登录会话已失效，请重新登录</p>
      <div class="session-actions">
        <button class="btn-session-primary" @click="handleReLogin">重新登录</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { sessionExpired } from '@/composables/useSessionExpired'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

// Only show dialog when not on the login page.
const showSessionDialog = computed(() => sessionExpired.value && route.path !== '/login')

// When session expires while on login page, just reset the flag.
watch(sessionExpired, (expired) => {
  if (expired && route.path === '/login') {
    sessionExpired.value = false
  }
})

function handleReLogin() {
  sessionExpired.value = false
  authStore.doLogout()
  router.push('/login')
}
</script>

<style scoped>
.session-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  animation: sessionFadeIn 0.2s ease;
}

@keyframes sessionFadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.session-dialog {
  background: var(--bg-card);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: 32px;
  width: 380px;
  text-align: center;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3), 0 0 1px rgba(0, 0, 0, 0.15);
  animation: sessionScaleIn 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

@keyframes sessionScaleIn {
  from { opacity: 0; transform: scale(0.96) translateY(8px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}

.session-icon {
  width: 52px;
  height: 52px;
  margin: 0 auto 18px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(227, 179, 65, 0.1);
  color: var(--accent-warning);
}

.session-icon svg {
  width: 26px;
  height: 26px;
}

.session-title {
  font-size: 17px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 8px;
}

.session-msg {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0 0 28px;
  line-height: 1.6;
}

.session-actions {
  display: flex;
  justify-content: center;
}

.btn-session-primary {
  padding: 9px 24px;
  border: none;
  border-radius: var(--radius-md);
  background: var(--btn-primary-bg, var(--accent-info));
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-session-primary:hover {
  background: var(--btn-primary-bg-hover, var(--accent-info));
  opacity: 0.92;
}
</style>
