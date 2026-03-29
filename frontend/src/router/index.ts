import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue') },
    { path: '/', name: 'home', component: () => import('@/views/DashboardView.vue') },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (auth.hasExceededReopenIdleTimeout()) {
    await auth.cleanupSessionAfterReopenTimeout()
    if (to.path !== '/login') {
      return '/login'
    }
    return
  }
  const validSession = await auth.resumeSessionAfterReopen()
  if (to.path !== '/login' && !validSession) {
    return '/login'
  }
  if (to.path === '/login' && validSession) {
    return '/'
  }
})

export default router
