import { defineStore } from 'pinia'

const EXPIRE_AT_KEY = 'ops_expire_at'
const TOKEN_KEY = 'ops_token'
const USERNAME_KEY = 'ops_username'
const IDLE_TTL_KEY = 'ops_idle_ttl_seconds'
const WINDOW_CLOSE_META_KEY_PREFIX = 'ops_last_unload_meta:'
const ACTIVE_BROWSER_TABS_KEY_PREFIX = 'ops_browser_active_tabs:'
const PROJECT_CACHE_DEADLINE_KEY_PREFIX = 'ops_project_cache_deadline:'
const PROJECT_CACHE_RELOGIN_LOCK_KEY_PREFIX = 'ops_project_cache_relogin_lock:'

type BrowserWindowCloseMeta = {
  reason: 'last_page_closed' | 'reopen_timeout'
  closed_at_ms: number
  timeout_at_ms: number
  idle_ttl_seconds: number
}

function createBrowserTabId(): string {
  return `tab_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`
}

function getWindowCloseMetaKey(token: string): string {
  const value = String(token || '').trim()
  return value ? `${WINDOW_CLOSE_META_KEY_PREFIX}${value}` : ''
}

function getActiveBrowserTabsKey(token: string): string {
  const value = String(token || '').trim()
  return value ? `${ACTIVE_BROWSER_TABS_KEY_PREFIX}${value}` : ''
}

function readStringArray(key: string): string[] {
  if (!key) {
    return []
  }
  try {
    const raw = localStorage.getItem(key)
    if (!raw) {
      return []
    }
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      return []
    }
    return Array.from(
      new Set(
        parsed
          .map((item) => String(item || '').trim())
          .filter(Boolean),
      ),
    )
  } catch {
    return []
  }
}

function writeStringArray(key: string, values: string[]) {
  if (!key) {
    return
  }
  const next = Array.from(new Set(values.map((item) => String(item || '').trim()).filter(Boolean)))
  if (next.length === 0) {
    localStorage.removeItem(key)
    return
  }
  localStorage.setItem(key, JSON.stringify(next))
}

function readWindowCloseMeta(token: string): BrowserWindowCloseMeta | null {
  const key = getWindowCloseMetaKey(token)
  if (!key) {
    return null
  }
  try {
    const raw = localStorage.getItem(key)
    if (!raw) {
      return null
    }
    const parsed = JSON.parse(raw)
    const closedAtMs = Number(parsed?.closed_at_ms || 0)
    const timeoutAtMs = Number(parsed?.timeout_at_ms || 0)
    const idleTTLSeconds = Math.max(1, Number(parsed?.idle_ttl_seconds || 0) || 0)
    if (!Number.isFinite(closedAtMs) || closedAtMs <= 0 || !Number.isFinite(timeoutAtMs) || timeoutAtMs <= 0) {
      return null
    }
    return {
      reason: 'last_page_closed',
      closed_at_ms: closedAtMs,
      timeout_at_ms: timeoutAtMs,
      idle_ttl_seconds: idleTTLSeconds,
    }
  } catch {
    return null
  }
}

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
    browserTabId: createBrowserTabId(),
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
      this.registerBrowserPage()
    },
    setIdleTTL(idleTtlSeconds?: number) {
      const next = Math.max(1, Number(idleTtlSeconds || this.idleTtlSeconds || 3600) || 3600)
      this.idleTtlSeconds = next
      localStorage.setItem(IDLE_TTL_KEY, String(next))
    },
    getWindowCloseMeta(): BrowserWindowCloseMeta | null {
      if (!this.token) {
        return null
      }
      return readWindowCloseMeta(this.token)
    },
    hasExceededReopenIdleTimeout() {
      if (!this.token) {
        return false
      }
      const meta = this.getWindowCloseMeta()
      if (!meta) {
        return false
      }
      return Date.now() >= meta.timeout_at_ms
    },
    async cancelWindowCloseCountdown(meta?: BrowserWindowCloseMeta | null) {
      const staleToken = this.token
      const currentMeta = meta || this.getWindowCloseMeta()
      if (!staleToken || !currentMeta) {
        return
      }
      try {
        await fetch(`${this.apiBase}/api/auth/window-close-cancel`, {
          method: 'POST',
          headers: {
            Authorization: `Bearer ${staleToken}`,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            reason: 'reopen_cancel',
            closed_at_ms: currentMeta.closed_at_ms,
            timeout_at_ms: currentMeta.timeout_at_ms,
            reopened_at_ms: Date.now(),
            idle_ttl_seconds: currentMeta.idle_ttl_seconds || this.idleTtlSeconds,
          }),
        })
      } catch {
        // ignore resume logging failures
      }
    },
    registerBrowserPage() {
      if (!this.token) {
        return
      }
      const key = getActiveBrowserTabsKey(this.token)
      const current = readStringArray(key)
      current.push(this.browserTabId)
      writeStringArray(key, current)
      this.clearWindowClosedMarker()
    },
    markWindowClosed(): BrowserWindowCloseMeta | null {
      if (!this.token) {
        return null
      }
      const key = getActiveBrowserTabsKey(this.token)
      const current = readStringArray(key).filter((id) => id !== this.browserTabId)
      writeStringArray(key, current)
      if (current.length > 0) {
        return null
      }
      const closedAtMs = Date.now()
      const idleTTLSeconds = Math.max(1, Number(this.idleTtlSeconds || 3600) || 3600)
      const meta: BrowserWindowCloseMeta = {
        reason: 'last_page_closed',
        closed_at_ms: closedAtMs,
        timeout_at_ms: closedAtMs + idleTTLSeconds * 1000,
        idle_ttl_seconds: idleTTLSeconds,
      }
      const closeKey = getWindowCloseMetaKey(this.token)
      if (closeKey) {
        localStorage.setItem(closeKey, JSON.stringify(meta))
      }
      return meta
    },
    clearWindowClosedMarker() {
      const key = getWindowCloseMetaKey(this.token)
      if (key) {
        localStorage.removeItem(key)
      }
    },
    async cleanupSessionAfterReopenTimeout() {
      const staleToken = this.token
      const meta = this.getWindowCloseMeta()
      const staleIdleTTL = this.idleTtlSeconds
      this.clearSession()
      if (!staleToken) {
        return
      }
      try {
        await fetch(`${this.apiBase}/api/auth/logout`, {
          method: 'POST',
          headers: {
            Authorization: `Bearer ${staleToken}`,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            reason: 'reopen_timeout',
            closed_at_ms: meta?.closed_at_ms || 0,
            timeout_at_ms: meta?.timeout_at_ms || 0,
            idle_ttl_seconds: meta?.idle_ttl_seconds || staleIdleTTL,
          }),
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
      if (this.isSessionValid()) {
        this.registerBrowserPage()
        return true
      }
      this.clearSession()
      return false
    },
    async resumeSessionAfterReopen() {
      if (!this.token) {
        return false
      }
      const closeMeta = this.getWindowCloseMeta()
      if (!this.isSessionValid()) {
        this.clearSession()
        return false
      }
      this.registerBrowserPage()
      if (closeMeta && Date.now() < closeMeta.timeout_at_ms) {
        await this.cancelWindowCloseCountdown(closeMeta)
      }
      return true
    },
    clearSession() {
      const deadlineKey = getProjectCacheDeadlineKey(this.token)
      const reloginLockKey = getProjectCacheReloginLockKey(this.token)
      const closeKey = getWindowCloseMetaKey(this.token)
      const activeTabsKey = getActiveBrowserTabsKey(this.token)
      this.token = ''
      this.username = ''
      this.expireAt = ''
      this.idleTtlSeconds = 3600
      this.loadedProjects = {}
      localStorage.removeItem(TOKEN_KEY)
      localStorage.removeItem(USERNAME_KEY)
      localStorage.removeItem(EXPIRE_AT_KEY)
      localStorage.removeItem(IDLE_TTL_KEY)
      if (deadlineKey) {
        localStorage.removeItem(deadlineKey)
      }
      if (reloginLockKey) {
        localStorage.removeItem(reloginLockKey)
      }
      if (closeKey) {
        localStorage.removeItem(closeKey)
      }
      if (activeTabsKey) {
        localStorage.removeItem(activeTabsKey)
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
