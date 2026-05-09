<template>
  <div class="main-layout">
    <aside class="sidebar">
      <div class="sidebar-header">
        <div class="logo">
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="var(--accent-primary)" stroke-width="2">
            <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/>
          </svg>
          <span class="logo-text">Salvo</span>
        </div>
      </div>

      <nav class="sidebar-nav">
        <router-link
          v-for="item in menuItems"
          :key="item.path"
          :to="item.path"
          class="nav-item"
          :class="{ active: isActive(item.path) }"
        >
          <component :is="item.icon" class="nav-icon" />
          <span>{{ item.label }}</span>
        </router-link>
      </nav>

      <div class="sidebar-footer">
        <div class="theme-toggle" @click="themeStore.toggle()">
          <svg v-if="themeStore.theme === 'dark'" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
          </svg>
          <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
          </svg>
          <span>{{ themeStore.theme === 'dark' ? '浅色' : '深色' }}</span>
        </div>
      </div>
    </aside>

    <div class="main-content">
      <header class="top-bar">
        <div class="breadcrumb">
          <span class="breadcrumb-item">{{ currentRouteLabel }}</span>
        </div>
        <div class="user-area">
          <span class="user-name">{{ authStore.user?.nickname || authStore.user?.email }}</span>
          <span class="user-role badge">{{ authStore.userRole }}</span>
          <button class="btn-logout" @click="showLogoutConfirm = true">退出</button>
        </div>
      </header>

      <main class="page-content">
        <router-view />
      </main>
    </div>

    <div v-if="showLogoutConfirm" class="modal-overlay" @click.self="showLogoutConfirm = false">
      <div class="confirm-dialog">
        <div class="confirm-icon">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/>
            <line x1="12" y1="8" x2="12" y2="12"/>
            <line x1="12" y1="16" x2="12.01" y2="16"/>
          </svg>
        </div>
        <h3 class="confirm-title">确认退出</h3>
        <p class="confirm-msg">确定要退出登录吗？</p>
        <div class="confirm-actions">
          <button class="btn-cancel" @click="showLogoutConfirm = false">取消</button>
          <button class="btn-danger-confirm" @click="handleLogout">确认退出</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const themeStore = useThemeStore()
const showLogoutConfirm = ref(false)

const DashboardIcon = {
  render() {
    return h('svg', { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': 2 }, [
      h('rect', { x: 3, y: 3, width: 7, height: 7, rx: 1 }),
      h('rect', { x: 14, y: 3, width: 7, height: 7, rx: 1 }),
      h('rect', { x: 3, y: 14, width: 7, height: 7, rx: 1 }),
      h('rect', { x: 14, y: 14, width: 7, height: 7, rx: 1 }),
    ])
  },
}

const SceneIcon = {
  render() {
    return h('svg', { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': 2 }, [
      h('polygon', { points: '12 2 2 7 12 12 22 7 12 2' }),
      h('polyline', { points: '2 17 12 22 22 17' }),
      h('polyline', { points: '2 12 12 17 22 12' }),
    ])
  },
}

const RunnerIcon = {
  render() {
    return h('svg', { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': 2 }, [
      h('polygon', { points: '5 3 19 12 5 21 5 3' }),
    ])
  },
}

const ReportIcon = {
  render() {
    return h('svg', { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': 2 }, [
      h('path', { d: 'M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z' }),
      h('polyline', { points: '14 2 14 8 20 8' }),
      h('line', { x1: 16, y1: 13, x2: 8, y2: 13 }),
      h('line', { x1: 16, y1: 17, x2: 8, y2: 17 }),
    ])
  },
}

const TraceIcon = {
  render() {
    return h('svg', { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': 2 }, [
      h('polyline', { points: '22 12 18 12 15 21 9 3 6 12 2 12' }),
    ])
  },
}

const UserIcon = {
  render() {
    return h('svg', { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': 2 }, [
      h('path', { d: 'M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2' }),
      h('circle', { cx: 9, cy: 7, r: 4 }),
      h('path', { d: 'M23 21v-2a4 4 0 0 0-3-3.87' }),
      h('path', { d: 'M16 3.13a4 4 0 0 1 0 7.75' }),
    ])
  },
}

const SettingsIcon = {
  render() {
    return h('svg', { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': 2 }, [
      h('circle', { cx: 12, cy: 12, r: 3 }),
      h('path', { d: 'M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z' }),
    ])
  },
}

const allMenuItems = [
  { path: '/dashboard', label: 'Dashboard', icon: DashboardIcon },
  { path: '/scenes', label: '场景管理', icon: SceneIcon },
  { path: '/runner', label: '运行控制', icon: RunnerIcon },
  { path: '/reports', label: '测试报告', icon: ReportIcon },
  { path: '/traces', label: '链路追踪', icon: TraceIcon },
  { path: '/users', label: '用户管理', icon: UserIcon, permission: 'users:read' },
  { path: '/settings', label: '系统设置', icon: SettingsIcon },
]

const menuItems = computed(() => {
  return allMenuItems.filter((item) => {
    if (!item.permission) return true
    return authStore.canAccess([item.permission])
  })
})

const currentRouteLabel = computed(() => {
  return (route.name as string) || 'Dashboard'
})

function isActive(path: string) {
  return route.path.startsWith(path)
}

function handleLogout() {
  showLogoutConfirm.value = false
  authStore.doLogout()
  router.push('/login')
}
</script>

<style scoped>
.main-layout {
  display: flex;
  height: 100%;
  width: 100%;
}

.sidebar {
  width: var(--sidebar-width);
  background: var(--bg-secondary);
  border-right: 1px solid var(--border-secondary);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}

.sidebar-header {
  height: var(--header-height);
  display: flex;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid var(--border-secondary);
}

.logo {
  display: flex;
  align-items: center;
  gap: 10px;
}

.logo-text {
  font-size: 20px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: 1px;
}

.sidebar-nav {
  flex: 1;
  padding: 12px 8px;
  overflow-y: auto;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  font-size: 14px;
  transition: all 0.15s ease;
  cursor: pointer;
  text-decoration: none;
}

.nav-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.nav-item.active {
  background: var(--bg-tertiary);
  color: var(--accent-primary);
}

.nav-icon {
  flex-shrink: 0;
}

.sidebar-footer {
  padding: 12px 16px;
  border-top: 1px solid var(--border-secondary);
}

.theme-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: var(--radius-md);
  cursor: pointer;
  color: var(--text-secondary);
  font-size: 13px;
  transition: all 0.15s ease;
}

.theme-toggle:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.top-bar {
  height: var(--header-height);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-secondary);
  flex-shrink: 0;
}

.breadcrumb {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.user-area {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-name {
  font-size: 13px;
  color: var(--text-secondary);
}

.badge {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 10px;
  background: var(--bg-tertiary);
  color: var(--accent-primary);
  border: 1px solid var(--border-primary);
}

.btn-logout {
  font-size: 13px;
  padding: 4px 12px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--accent-danger);
  background: transparent;
  color: var(--accent-danger);
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-logout:hover {
  background: var(--accent-danger);
  color: #fff;
  border-color: var(--accent-danger);
}

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.confirm-dialog {
  background: var(--bg-card);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  padding: 28px;
  width: 380px;
  text-align: center;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3), 0 0 1px rgba(0, 0, 0, 0.15);
}

.confirm-icon {
  width: 48px;
  height: 48px;
  margin: 0 auto 16px;
  border-radius: 50%;
  background: rgba(239, 68, 68, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--accent-danger);
}

.confirm-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 8px;
}

.confirm-msg {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0 0 24px;
  line-height: 1.5;
}

.confirm-actions {
  display: flex;
  justify-content: center;
  gap: 10px;
}

.btn-cancel {
  padding: 8px 20px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  background: var(--bg-tertiary);
  color: var(--text-primary);
  font-size: 13px;
  cursor: pointer;
  transition: background 0.15s ease;
}

.btn-cancel:hover {
  background: var(--bg-hover);
}

.btn-danger-confirm {
  padding: 8px 20px;
  border: none;
  border-radius: var(--radius-md);
  background: var(--accent-danger);
  color: #fff;
  font-size: 13px;
  cursor: pointer;
  transition: opacity 0.15s ease;
}

.btn-danger-confirm:hover {
  opacity: 0.88;
}

.page-content {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
  background: var(--bg-primary);
}
</style>
