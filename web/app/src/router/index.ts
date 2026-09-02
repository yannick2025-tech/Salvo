import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/views/login/LoginPage.vue'),
      meta: { requiresAuth: false },
    },
    {
      path: '/',
      name: 'Layout',
      component: () => import('@/layouts/MainLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        { path: '', redirect: '/dashboard' },
        { path: 'dashboard', name: 'Dashboard', component: () => import('@/views/dashboard/DashboardPage.vue') },
        { path: 'scenes', name: 'Scenes', component: () => import('@/views/scenes/ScenesPage.vue') },
        { path: 'scenes/:id', name: 'SceneDetail', component: () => import('@/views/scenes/SceneDetailPage.vue'), props: true },
        { path: 'scenes/:id/settings', name: 'SceneSettings', component: () => import('@/views/scenes/SceneSettingsPage.vue'), props: true },
        { path: 'runner', name: 'Runner', component: () => import('@/views/runner/RunnerPage.vue') },
        { path: 'reports', name: 'Reports', component: () => import('@/views/reports/ReportsPage.vue') },
        { path: 'reports/:id', name: 'ReportDetail', component: () => import('@/views/reports/ReportDetailPage.vue'), props: true },
        { path: 'traces', name: 'Traces', component: () => import('@/views/traces/TracesPage.vue') },
        { path: 'traces/:id', name: 'TraceDetail', component: () => import('@/views/traces/TraceDetailPage.vue'), props: true },
        { path: 'users', name: 'Users', component: () => import('@/views/users/UsersPage.vue'), meta: { permission: 'users:read' } },
        { path: 'settings', name: 'Settings', component: () => import('@/views/settings/SettingsPage.vue') },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
  ],
})

router.beforeEach(async (to, _from, next) => {
  const auth = useAuthStore()
  console.log('[router] beforeEach:', to.path, 'token:', !!auth.token, 'validated:', auth.validated, 'isLoggedIn:', auth.isLoggedIn)

  if (auth.token && !auth.validated) {
    console.log('[router] calling validateToken')
    await auth.validateToken()
    console.log('[router] after validateToken: validated=', auth.validated, 'isLoggedIn=', auth.isLoggedIn)
  }

  if (to.meta.requiresAuth !== false && !auth.isLoggedIn) {
    console.log('[router] redirecting to Login')
    next({ name: 'Login', query: { redirect: to.fullPath } })
    return
  }

  if (to.name === 'Login' && auth.isLoggedIn) {
    console.log('[router] redirecting to Dashboard (already logged in)')
    next({ name: 'Dashboard' })
    return
  }

  if (to.meta.permission && !auth.canAccess([to.meta.permission as string])) {
    console.log('[router] permission denied for', to.meta.permission)
    next({ name: 'Dashboard' })
    return
  }

  next()
})

export default router

// 路由变更时 HMR 无法自动更新已注册的路由表，需要完整刷新
if (import.meta.hot) {
  import.meta.hot.accept(() => {
    import.meta.hot!.invalidate()
  })
}
