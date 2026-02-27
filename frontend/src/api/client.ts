import { useAuthStore } from '@/stores/auth'
import router from '@/router'

const AUTH_EXPIRED_CODE = 'AUTH_EXPIRED'
let redirectingToLogin = false

function createAuthExpiredError() {
  const err = new Error('会话已过期，请重新登录')
  ;(err as any).code = AUTH_EXPIRED_CODE
  return err
}

async function redirectToLoginIfNeeded() {
  if (redirectingToLogin) {
    return
  }
  redirectingToLogin = true
  try {
    const auth = useAuthStore()
    auth.clearSession()
    if (router.currentRoute.value.path !== '/login') {
      await router.replace('/login')
    }
  } finally {
    redirectingToLogin = false
  }
}

export function isAuthExpiredError(err: any): boolean {
  return !!err && (err.code === AUTH_EXPIRED_CODE || err.__authExpired === true)
}

export async function apiRequest(path: string, method = 'GET', body?: any) {
  const auth = useAuthStore()
  if (auth.token && !auth.isSessionValid()) {
    await redirectToLoginIfNeeded()
    throw createAuthExpiredError()
  }
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (auth.token) {
    headers.Authorization = `Bearer ${auth.token}`
  }

  const res = await fetch(`${auth.apiBase}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })

  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    if (res.status === 401) {
      await redirectToLoginIfNeeded()
      throw createAuthExpiredError()
    }
    throw new Error(data.error || data.message || `HTTP ${res.status}`)
  }
  return data
}
