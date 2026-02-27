<template>
  <div class="login-wrap" @keyup.enter="onPrimaryAction">
    <transition name="auth-panel" mode="out-in" appear>
      <div :key="mode" class="auth-panel-wrap">
        <n-card class="login-card" :title="mode === 'login' ? '登录' : '注册'">
          <div class="login-subtitle">
            {{ mode === 'login' ? '欢迎使用运维集成管理后台系统' : '创建管理员账号' }}
          </div>

          <n-form
            :class="['login-form', mode === 'register' ? 'is-register' : 'is-login']"
            label-placement="left"
            label-align="right"
            :label-width="96"
          >
            <n-form-item v-if="mode === 'login'" label="管理员账号">
              <n-input v-model:value="loginUsername" placeholder="请输入账号" />
            </n-form-item>
            <n-form-item v-else label="管理员账号">
              <n-input v-model:value="registerUsername" placeholder="请输入账号" />
            </n-form-item>

            <n-form-item v-if="mode === 'login'" label="管理员密码">
              <n-input
                v-model:value="loginPassword"
                type="password"
                placeholder="请输入密码"
                show-password-on="click"
              />
            </n-form-item>
            <n-form-item v-else label="管理员密码">
              <n-input
                v-model:value="registerPassword"
                type="password"
                placeholder="请输入密码"
                show-password-on="click"
              />
            </n-form-item>

            <n-form-item v-if="mode === 'register'" label="确认密码">
              <n-input
                v-model:value="registerConfirmPassword"
                type="password"
                placeholder="请再次输入密码"
                show-password-on="click"
              />
            </n-form-item>

            <div :class="['login-options', mode === 'register' ? 'is-register-options' : 'is-login-options']">
              <n-checkbox v-if="mode === 'login'" v-model:checked="rememberMe">记住我</n-checkbox>
              <span v-else class="login-options-placeholder"></span>
              <n-button text type="primary" class="switch-link" @click="switchMode">
                {{ mode === 'login' ? '还没有账号？立即注册' : '已有账号？立即登录' }}
              </n-button>
            </div>

            <n-button class="login-submit" block type="primary" :loading="loading" @click="onPrimaryAction">
              {{ mode === 'login' ? '登录' : '注册' }}
            </n-button>
          </n-form>
        </n-card>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { NCard, NForm, NFormItem, NInput, NButton, NCheckbox, useMessage } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const message = useMessage()
const auth = useAuthStore()

const REMEMBER_FLAG_KEY = 'ops_remember_login'
const REMEMBER_USERNAME_KEY = 'ops_remember_username'
const REMEMBER_PASSWORD_KEY = 'ops_remember_password'

function loadRememberedLogin() {
  const remembered = localStorage.getItem(REMEMBER_FLAG_KEY) === '1'
  if (!remembered) {
    return { remembered: false, username: '', password: '' }
  }
  return {
    remembered: true,
    username: localStorage.getItem(REMEMBER_USERNAME_KEY) || '',
    password: localStorage.getItem(REMEMBER_PASSWORD_KEY) || '',
  }
}

function persistRememberedLogin(remember: boolean, loginUsername: string, loginPassword: string) {
  if (!remember) {
    localStorage.removeItem(REMEMBER_FLAG_KEY)
    localStorage.removeItem(REMEMBER_USERNAME_KEY)
    localStorage.removeItem(REMEMBER_PASSWORD_KEY)
    return
  }
  localStorage.setItem(REMEMBER_FLAG_KEY, '1')
  localStorage.setItem(REMEMBER_USERNAME_KEY, loginUsername)
  localStorage.setItem(REMEMBER_PASSWORD_KEY, loginPassword)
}

const rememberedLogin = loadRememberedLogin()
const initialLoginState = {
  username: rememberedLogin.username,
  password: rememberedLogin.password,
  remember: rememberedLogin.remembered,
}

const mode = ref<'login' | 'register'>('login')
const loginUsername = ref(rememberedLogin.username)
const loginPassword = ref(rememberedLogin.password)
const registerUsername = ref('')
const registerPassword = ref('')
const registerConfirmPassword = ref('')
const rememberMe = ref(rememberedLogin.remembered)
const loading = ref(false)

watch(rememberMe, (checked) => {
  if (!checked) {
    persistRememberedLogin(false, '', '')
  }
})

function resetLoginFormToInitial() {
  loginUsername.value = initialLoginState.username
  loginPassword.value = initialLoginState.password
  rememberMe.value = initialLoginState.remember
}

function resetRegisterFormToInitial() {
  registerUsername.value = ''
  registerPassword.value = ''
  registerConfirmPassword.value = ''
}

function switchMode() {
  if (mode.value === 'login') {
    resetRegisterFormToInitial()
    mode.value = 'register'
    return
  }
  resetLoginFormToInitial()
  mode.value = 'login'
}

async function onLogin() {
  if (!loginUsername.value.trim() || !loginPassword.value.trim()) {
    message.error('请完整填写登录信息')
    return
  }
  const res = await fetch(`${auth.apiBase}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: loginUsername.value.trim(), password: loginPassword.value }),
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(data.error || '登录失败')
  }
  persistRememberedLogin(rememberMe.value, loginUsername.value.trim(), loginPassword.value)
  auth.setSession(data.token, data.username, String(data.expire_at || ''))
  if (data.default_pwd) {
    message.warning('当前仍是默认密码，建议登录后尽快修改')
  }
  await router.push('/')
}

async function onRegister() {
  if (
    !registerUsername.value.trim() ||
    !registerPassword.value.trim() ||
    !registerConfirmPassword.value.trim()
  ) {
    message.error('请完整填写注册信息')
    return
  }
  if (registerPassword.value !== registerConfirmPassword.value) {
    message.error('两次输入密码不一致')
    return
  }
  if (registerPassword.value.trim().length < 8) {
    message.error('密码长度至少 8 位')
    return
  }
  const res = await fetch(`${auth.apiBase}/api/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      username: registerUsername.value.trim(),
      password: registerPassword.value,
    }),
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(data.error || '注册失败')
  }
  message.success('注册成功，请使用新账号登录')
  mode.value = 'login'
  registerUsername.value = ''
  registerPassword.value = ''
  registerConfirmPassword.value = ''
}

async function onPrimaryAction() {
  loading.value = true
  try {
    if (mode.value === 'login') {
      await onLogin()
    } else {
      await onRegister()
    }
  } catch (e: any) {
    message.error(e?.message || (mode.value === 'login' ? '登录失败' : '注册失败'))
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrap {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px 28px;
  position: relative;
  overflow: hidden;
  background:
    radial-gradient(780px 360px at 8% 10%, rgba(46, 141, 212, 0.22), transparent 62%),
    radial-gradient(720px 320px at 92% 14%, rgba(24, 169, 146, 0.2), transparent 60%),
    radial-gradient(560px 280px at 52% 92%, rgba(28, 115, 180, 0.16), transparent 64%),
    linear-gradient(145deg, #f3f9ff 0%, #eaf4ff 45%, #edf8f8 100%);
}

.login-wrap::before,
.login-wrap::after {
  content: '';
  position: absolute;
  border-radius: 999px;
  filter: blur(2px);
  pointer-events: none;
}

.login-wrap::before {
  width: 420px;
  height: 420px;
  left: -130px;
  top: -150px;
  background: radial-gradient(circle at 30% 30%, rgba(39, 122, 186, 0.22), rgba(39, 122, 186, 0));
}

.login-wrap::after {
  width: 460px;
  height: 460px;
  right: -180px;
  bottom: -170px;
  background: radial-gradient(circle at 35% 35%, rgba(16, 156, 133, 0.2), rgba(16, 156, 133, 0));
}

.auth-panel-wrap {
  width: 100%;
  max-width: 580px;
}

.login-card {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: none;
  border-radius: 22px !important;
  align-self: center;
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid rgba(207, 225, 239, 0.92);
  box-shadow: 0 28px 62px rgba(13, 55, 90, 0.16);
  backdrop-filter: blur(6px);
}

.login-card :deep(.n-card-header) {
  padding: 16px 22px 6px;
}

.login-card :deep(.n-card__content) {
  padding: 12px 34px 22px !important;
}

.login-form {
  width: 92%;
  margin: 0 auto;
}

.login-card :deep(.n-form-item) {
  margin-bottom: 16px;
}

.login-card :deep(.n-form-item .n-form-item-blank) {
  flex: 1;
  min-width: 0;
}

.login-card :deep(.n-form-item .n-input) {
  width: 100%;
}

.login-card :deep(.n-card-header__main) {
  font-weight: 700;
  font-size: 30px;
  line-height: 1.2;
  color: #0f3554;
  text-align: center;
}

.login-subtitle {
  color: #55748f;
  margin-bottom: 16px;
  font-size: 16px;
  text-align: center;
}

.login-options {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: 8px 0 14px;
  min-height: 28px;
}

.login-options-placeholder {
  height: 22px;
}

.is-register-options {
  justify-content: center;
}

.is-register-options .login-options-placeholder {
  display: none;
}

.switch-link {
  padding: 0 !important;
  background: transparent !important;
  box-shadow: none !important;
}

.switch-link:hover,
.switch-link:focus,
.switch-link:active {
  background: transparent !important;
}

.login-submit {
  margin-top: 18px;
}

.is-login .login-submit {
  margin-bottom: 10px;
}

.is-register .login-submit {
  margin-bottom: 18px;
}

.is-register-options {
  margin-bottom: 16px;
}

.auth-panel-enter-active,
.auth-panel-leave-active {
  transition:
    opacity 0.72s cubic-bezier(0.22, 0.61, 0.36, 1),
    transform 0.72s cubic-bezier(0.22, 0.61, 0.36, 1);
}

.auth-panel-enter-from {
  opacity: 0;
  transform: translateY(-38px);
}

.auth-panel-enter-to {
  opacity: 1;
  transform: translateY(0);
}

.auth-panel-leave-from {
  opacity: 1;
  transform: translateY(0);
}

.auth-panel-leave-to {
  opacity: 0;
  transform: translateY(14px);
}
</style>
