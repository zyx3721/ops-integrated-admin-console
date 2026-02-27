import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue') },
    { path: '/', name: 'home', component: () => import('@/views/DashboardView.vue') },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  const validSession = auth.ensureSession()
  if (to.path !== '/login' && !validSession) {
    return '/login'
  }
  if (to.path === '/login' && validSession) {
    return '/'
  }
})

export default router
