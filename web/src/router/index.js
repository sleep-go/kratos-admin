import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue'),
  },
  {
    path: '/',
    name: 'AdminList',
    component: () => import('../views/AdminList.vue'),
    meta: { requiresAuth: true },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to) => {
  if (to.meta.requiresAuth) {
    const auth = useAuthStore()
    if (!auth.user) {
      await auth.fetchCurrent()
    }
    if (!auth.user) {
      return { name: 'Login' }
    }
  }
})

export default router
