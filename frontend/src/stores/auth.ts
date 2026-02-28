import { defineStore } from 'pinia'

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
    token: localStorage.getItem('ops_token') || '',
    username: localStorage.getItem('ops_username') || '',
    expireAt: localStorage.getItem('ops_expire_at') || '',
    loadedProjects: {} as Record<string, boolean>,
  }),
  actions: {
    setSession(token: string, username: string, expireAt = '') {
      this.token = token
      this.username = username
      this.expireAt = expireAt
      localStorage.setItem('ops_token', token)
      localStorage.setItem('ops_username', username)
      if (expireAt) {
        localStorage.setItem('ops_expire_at', expireAt)
      } else {
        localStorage.removeItem('ops_expire_at')
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
        return true
      }
      this.clearSession()
      return false
    },
    clearSession() {
      this.token = ''
      this.username = ''
      this.expireAt = ''
      this.loadedProjects = {}
      localStorage.removeItem('ops_token')
      localStorage.removeItem('ops_username')
      localStorage.removeItem('ops_expire_at')
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
