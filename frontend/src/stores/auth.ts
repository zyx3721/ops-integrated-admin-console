import { defineStore } from 'pinia'

const EXPIRE_AT_KEY = 'ops_expire_at'
const TOKEN_KEY = 'ops_token'
const USERNAME_KEY = 'ops_username'
const IDLE_TTL_KEY = 'ops_idle_ttl_seconds'
const LAST_UNLOAD_AT_KEY = 'ops_last_unload_at'
const PROJECT_CACHE_DEADLINE_KEY_PREFIX = 'ops_project_cache_deadline:'
const PROJECT_CACHE_RELOGIN_LOCK_KEY_PREFIX = 'ops_project_cache_relogin_lock:'

export function getProjectCacheDeadlineKey(token: string): string {
  const value = String(token || '').trim()
  return value ? `${PROJECT_CACHE_DEADLINE_KEY_PREFIX}${value}` : ''
}

export function getProjectCacheReloginLockKey(token: string): string {
  const value = String(token || '').trim()
  return value ? `${PROJECT_CACHE_RELOGIN_LOCK_KEY_PREFIX}${value}` : ''
}

function normalizeApiBase(raw: string): string {
  return raw.trim().replace(/\/+$/, '')
}

function defaultApiBase(): string {
  // 浏览器端请求默认走同源（/api），由 Vite dev server 代理到 VITE_API_BASE。
  // 如需强制浏览器直连后端，可在前端环境变量里配置 VITE_BROWSER_API_BASE。
  const browserBase = import.meta.env.VITE_BROWSER_API_BASE as string | undefined
  if (browserBase && browserBase.trim()) {
    return normalizeApiBase(browserBase)
  }
  return ''
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    apiBase: defaultApiBase(),
    token: localStorage.getItem(TOKEN_KEY) || '',
    username: localStorage.getItem(USERNAME_KEY) || '',
    expireAt: localStorage.getItem(EXPIRE_AT_KEY) || '',
    idleTtlSeconds: Math.max(1, Number(localStorage.getItem(IDLE_TTL_KEY) || 3600) || 3600),
    loadedProjects: {} as Record<string, boolean>,
  }),
  actions: {
    setSession(token: string, username: string, expireAt = '', idleTtlSeconds?: number) {
      this.token = token
      this.username = username
      this.expireAt = expireAt
      this.setIdleTTL(idleTtlSeconds)
      localStorage.setItem(TOKEN_KEY, token)
      localStorage.setItem(USERNAME_KEY, username)
      if (expireAt) {
        localStorage.setItem(EXPIRE_AT_KEY, expireAt)
      } else {
        localStorage.removeItem(EXPIRE_AT_KEY)
      }
      localStorage.removeItem(LAST_UNLOAD_AT_KEY)
    },
    setIdleTTL(idleTtlSeconds?: number) {
      const next = Math.max(1, Number(idleTtlSeconds || this.idleTtlSeconds || 3600) || 3600)
      this.idleTtlSeconds = next
      localStorage.setItem(IDLE_TTL_KEY, String(next))
    },
    hasExceededReopenIdleTimeout() {
      if (!this.token) {
        return false
      }
      const raw = Number(localStorage.getItem(LAST_UNLOAD_AT_KEY) || 0)
      if (!Number.isFinite(raw) || raw <= 0) {
        return false
      }
      return Date.now() - raw >= this.idleTtlSeconds * 1000
    },
    markWindowClosed() {
      if (!this.token) {
        return
      }
      localStorage.setItem(LAST_UNLOAD_AT_KEY, String(Date.now()))
    },
    clearWindowClosedMarker() {
      localStorage.removeItem(LAST_UNLOAD_AT_KEY)
    },
    async cleanupSessionAfterReopenTimeout() {
      const staleToken = this.token
      this.clearSession()
      if (!staleToken) {
        return
      }
      try {
        await fetch(`${this.apiBase}/api/auth/logout`, {
          method: 'POST',
          headers: { Authorization: `Bearer ${staleToken}` },
          keepalive: true,
        })
      } catch {
        // ignore network cleanup errors on reopen timeout
      }
    },
    isSessionValid() {
      if (!this.token) {
        return false
      }
      const rawExpire = (this.expireAt || '').trim()
      if (!rawExpire) {
        return true
      }
      const ts = Date.parse(rawExpire)
      if (Number.isNaN(ts)) {
        return true
      }
      return Date.now() < ts
    },
    ensureSession() {
      if (!this.token) {
        return false
      }
      this.clearWindowClosedMarker()
      if (this.isSessionValid()) {
        return true
      }
      this.clearSession()
      return false
    },
    clearSession() {
      const deadlineKey = getProjectCacheDeadlineKey(this.token)
      const reloginLockKey = getProjectCacheReloginLockKey(this.token)
      this.token = ''
      this.username = ''
      this.expireAt = ''
      this.idleTtlSeconds = 3600
      this.loadedProjects = {}
      localStorage.removeItem(TOKEN_KEY)
      localStorage.removeItem(USERNAME_KEY)
      localStorage.removeItem(EXPIRE_AT_KEY)
      localStorage.removeItem(IDLE_TTL_KEY)
      localStorage.removeItem(LAST_UNLOAD_AT_KEY)
      if (deadlineKey) {
        localStorage.removeItem(deadlineKey)
      }
      if (reloginLockKey) {
        localStorage.removeItem(reloginLockKey)
      }
    },
    markProjectLoaded(project: string, loaded = true) {
      this.loadedProjects[project] = loaded
    },
    resetProjectLoaded(project: string) {
      delete this.loadedProjects[project]
    },
    clearLoadedProjects() {
      this.loadedProjects = {}
    },
  },
})
