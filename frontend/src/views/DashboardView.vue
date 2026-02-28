<template>
  <n-layout :has-sider="!isMobile" class="page-layout">
    <n-layout-sider v-if="!isMobile" width="220" bordered class="main-sider">
      <div class="sider-title">运维控制台</div>
      <n-menu class="main-menu" :value="activeView" :options="menuOptions" @update:value="onMenuSelect" />
    </n-layout-sider>

    <n-layout class="main-shell">
      <n-layout-header bordered class="topbar">
        <div class="top-main">
          <n-button v-if="isMobile" quaternary size="small" class="mobile-menu-btn" @click="showMainMenuDrawer = true">
            菜单
          </n-button>
          <div class="top-title">运维管理后台系统</div>
        </div>
        <div class="top-actions">
          <span>管理员：{{ auth.username }}</span>
          <span class="cache-countdown">缓存倒计时：{{ formatCountdown(cacheCountdownSeconds) }}</span>
          <n-button size="small" @click="showPwd = true">修改密码</n-button>
          <n-button size="small" type="error" @click="logout">退出</n-button>
        </div>
      </n-layout-header>

      <n-layout-content class="page-content" content-style="padding: 16px;">
        <div class="view-banner" :class="`view-banner--${activeView}`">
          <div class="view-banner-main">
            <div class="view-banner-kicker">{{ currentViewMeta.kicker }}</div>
            <div class="view-banner-title">{{ currentViewMeta.title }}</div>
            <div class="view-banner-desc">{{ currentViewMeta.desc }}</div>
          </div>
          <div class="view-banner-tags">
            <span class="view-tag">{{ currentViewMeta.tag }}</span>
            <span v-if="isProjectView" class="view-tag view-tag--soft">缓存：{{ formatCountdown(cacheCountdownSeconds) }}</span>
          </div>
        </div>

        <n-spin :show="loadingProject">
          <template #description>正在执行项目自动登录，请稍候...</template>

          <n-card v-if="activeView === 'config'" title="项目管理账号配置" size="small">
            <n-grid :cols="credentialGridCols" :x-gap="12" :y-gap="12">
              <n-grid-item v-for="item in credentials" :key="item.project_type">
                <n-card :title="credentialTitle(item.project_type)" size="small">
                  <n-form>
                    <n-form-item label="账号">
                      <n-input v-model:value="item.account" />
                    </n-form-item>
                    <n-form-item label="密码">
                      <n-input v-model:value="item.password" type="password" show-password-on="click" />
                    </n-form-item>
                    <n-button type="primary" block @click="saveCredential(item)">确认</n-button>
                  </n-form>
                </n-card>
              </n-grid-item>
            </n-grid>
          </n-card>

          <n-layout v-if="isProjectView" :has-sider="!isMobile" class="project-layout">
            <n-layout-sider v-if="!isMobile" width="220" bordered class="project-sider">
              <div class="project-func-title">
                <span class="project-func-dot" :class="`project-func-dot--${activeView}`"></span>
                <span>功能列表</span>
              </div>
              <n-menu
                class="project-menu"
                :value="currentProjectAction"
                :options="projectFunctionMenuOptions"
                @update:value="onProjectFunctionSelect"
              />
            </n-layout-sider>

            <n-layout>
              <n-layout-content content-style="padding: 12px;">
                <div v-if="isMobile" class="mobile-action-switch">
                  <span class="mobile-action-switch__label">功能选择</span>
                  <n-select
                    :value="currentProjectAction"
                    :options="projectFunctionSelectOptions"
                    placeholder="请选择功能"
                    @update:value="onProjectFunctionSelect"
                  />
                </div>
                <n-card v-if="currentProjectForm" :title="currentProjectForm.title" size="small" :class="['project-action-card', `project-action-card--${activeView}`]">
                  <div class="func-panel">
                    <div class="func-input">
                      <n-form label-placement="top">
                        <n-form-item
                          v-for="field in visibleFields(currentProjectForm)"
                          :key="`${currentProjectForm.action}-${field.key}`"
                        >
                          <template #label>
                            <span>{{ field.label }}</span>
                            <span v-if="field.required" class="required-star">*</span>
                          </template>

                          <n-input
                            v-if="field.type === 'text'"
                            v-model:value="currentProjectForm.model[field.key]"
                            :readonly="field.readonly"
                            :placeholder="field.placeholder"
                            @focus="onProjectFieldFocus"
                            @blur="onProjectFieldBlur"
                          />

                          <n-input
                            v-else-if="field.type === 'password'"
                            v-model:value="currentProjectForm.model[field.key]"
                            :type="field.masked === false ? 'text' : 'password'"
                            show-password-on="click"
                            :readonly="field.readonly"
                            :placeholder="field.placeholder"
                            @focus="onProjectFieldFocus"
                            @blur="onProjectFieldBlur"
                          >
                            <template v-if="field.randomButton" #suffix>
                              <n-button text size="tiny" @click="fillRandomPassword(currentProjectForm, field.key)">
                                随机生成
                              </n-button>
                            </template>
                          </n-input>

                          <n-input
                            v-else-if="field.type === 'textarea'"
                            type="textarea"
                            v-model:value="currentProjectForm.model[field.key]"
                            :autosize="{ minRows: 3, maxRows: 6 }"
                            :placeholder="field.placeholder"
                            @focus="onProjectFieldFocus"
                            @blur="onProjectFieldBlur"
                          />

                          <n-select
                            v-else-if="field.type === 'select'"
                            v-model:value="currentProjectForm.model[field.key]"
                            :options="field.options || []"
                            :multiple="field.multiple"
                            :max-tag-count="field.multiple ? 'responsive' : undefined"
                            :placeholder="field.placeholder"
                            @focus="onProjectFieldFocus"
                            @blur="onProjectFieldBlur"
                          />

                          <div v-else-if="field.type === 'file'" class="upload-block">
                            <n-upload
                              accept=".xlsx,.xls"
                              :show-file-list="false"
                              :custom-request="(options: any) => handleAdBatchFileUpload(options, currentProjectForm, field)"
                            >
                              <n-upload-dragger>
                                <div class="upload-title">点击上传或拖拽 Excel 文件到此区域</div>
                                <div class="upload-tip">支持 .xlsx / .xls</div>
                              </n-upload-dragger>
                            </n-upload>
                            <div class="upload-actions">
                              <n-button size="small" @click="downloadAdBatchTemplate">下载模板</n-button>
                              <span v-if="currentProjectForm.model[field.key]" class="upload-file-name">
                                已上传：{{ currentProjectForm.model[field.key] }}
                              </span>
                            </div>
                          </div>

                          <n-switch v-else-if="field.type === 'switch'" v-model:value="currentProjectForm.model[field.key]" />
                        </n-form-item>

                        <template v-if="isPrintModifyForm(currentProjectForm)">
                          <n-button
                            v-if="!isPrintModifyEditMode(currentProjectForm)"
                            type="primary"
                            block
                            :loading="currentProjectForm.loading"
                            @click="confirmPrintModify(currentProjectForm)"
                          >
                            确认
                          </n-button>
                          <div v-else class="dual-action-buttons">
                            <n-button type="primary" block :loading="currentProjectForm.loading" @click="submitAction(currentProjectForm)">
                              执行
                            </n-button>
                            <n-button block @click="backPrintModify(currentProjectForm)">返回</n-button>
                          </div>
                        </template>
                        <n-button v-else type="primary" block :loading="currentProjectForm.loading" @click="submitAction(currentProjectForm)">
                          执行
                        </n-button>
                      </n-form>
                    </div>

                    <div class="func-output">
                      <div class="result-title">执行结果</div>
                      <n-progress
                        v-if="currentProjectForm.loading || currentProjectForm.progress > 0"
                        class="result-progress"
                        type="line"
                        :percentage="currentProjectForm.progress"
                        :processing="currentProjectForm.loading"
                        :indicator-placement="'inside'"
                      />
                      <n-table
                        v-if="currentProjectForm.action === 'batch_add_users' && currentProjectForm.resultItems.length > 0"
                        class="batch-result-table"
                        size="small"
                        striped
                      >
                        <thead>
                          <tr>
                            <th>用户名</th>
                            <th>密码</th>
                            <th>成功/失败</th>
                            <th>错误原因</th>
                          </tr>
                        </thead>
                        <tbody>
                          <tr v-for="(row, idx) in currentProjectForm.resultItems" :key="idx">
                            <td>{{ row.username }}</td>
                            <td>{{ row.password || '-' }}</td>
                            <td>{{ row.ok ? '成功' : '失败' }}</td>
                            <td class="error-reason-cell">
                              <n-tooltip trigger="hover">
                                <template #trigger>
                                  <span class="error-reason-ellipsis">{{ row.error_reason || '-' }}</span>
                                </template>
                                {{ row.error_reason || '-' }}
                              </n-tooltip>
                            </td>
                          </tr>
                        </tbody>
                      </n-table>
                      <pre v-else class="result-box">{{ currentProjectForm.result }}</pre>
                    </div>
                  </div>
                </n-card>
              </n-layout-content>
            </n-layout>
          </n-layout>

          <n-card v-if="activeView === 'logs'" title="操作日志" size="small">
            <div class="logs-toolbar">
              <n-button size="small" @click="loadLogs(logPage, logPageSize)">刷新日志</n-button>
            </div>
            <div class="logs-table-wrap">
              <n-table class="logs-table" size="small" striped style="margin-top: 12px">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>时间</th>
                  <th>用户</th>
                  <th>行为</th>
                  <th>项目</th>
                  <th>详情</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="log in logs" :key="log.id">
                  <td>{{ log.id }}</td>
                  <td>{{ log.created_at }}</td>
                  <td>{{ log.username }}</td>
                  <td>{{ log.action }}</td>
                  <td>{{ log.project_type }}</td>
                  <td>{{ log.detail }}</td>
                </tr>
              </tbody>
              </n-table>
            </div>
            <n-pagination
              v-model:page="logPage"
              v-model:page-size="logPageSize"
              style="margin-top: 12px; justify-content: flex-end"
              :item-count="logTotal"
              :page-sizes="logPageSizeOptions"
              show-size-picker
              @update:page="handleLogPageChange"
              @update:page-size="handleLogPageSizeChange"
            />
          </n-card>
        </n-spin>
      </n-layout-content>
    </n-layout>
  </n-layout>

  <n-drawer v-model:show="showMainMenuDrawer" placement="left" :width="260">
    <n-drawer-content title="导航菜单" closable>
      <n-menu class="mobile-drawer-menu" :value="activeView" :options="menuOptions" @update:value="onMobileMenuSelect" />
    </n-drawer-content>
  </n-drawer>

  <n-modal
    v-model:show="showPwd"
    preset="dialog"
    title="修改管理员密码"
    positive-text="确认"
    negative-text="取消"
    @positive-click="changePassword"
  >
    <n-form>
      <n-form-item label="原密码">
        <n-input v-model:value="oldPwd" type="password" show-password-on="click" />
      </n-form-item>
      <n-form-item label="新密码">
        <n-input v-model:value="newPwd" type="password" show-password-on="click" />
      </n-form-item>
    </n-form>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  NLayout,
  NLayoutSider,
  NMenu,
  NLayoutHeader,
  NLayoutContent,
  NSpin,
  NCard,
  NGrid,
  NGridItem,
  NForm,
  NFormItem,
  NInput,
  NButton,
  NTable,
  NTooltip,
  NModal,
  NSelect,
  NPagination,
  NProgress,
  NSwitch,
  NUpload,
  NUploadDragger,
  NDrawer,
  NDrawerContent,
  useMessage,
} from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { apiRequest, isAuthExpiredError } from '@/api/client'
import { AD_ORG_UNIT_VALUES } from '@/config/ad'
import {
  PRINT_GENDER_OPTIONS_ADD,
  PRINT_GENDER_OPTIONS_MODIFY,
  PRINT_ROLE_NAME_TO_ID,
  PRINT_ROLE_OPTIONS,
  PRINT_SEARCH_KEY_OPTIONS,
  PRINT_SECTION_VALUES,
  PRINT_STATUS_OPTIONS,
} from '@/config/print'
import { VPN_SECTION_VALUES } from '@/config/vpn'

type FieldPhase = 'query' | 'edit' | 'all'
type Field = {
  key: string
  label: string
  type: 'text' | 'password' | 'select' | 'switch' | 'textarea' | 'file'
  options?: { label: string; value: any }[]
  required?: boolean
  readonly?: boolean
  masked?: boolean
  placeholder?: string
  randomButton?: boolean
  multiple?: boolean
  phase?: FieldPhase
}

type ActionForm = {
  title: string
  action: string
  fields: Field[]
  model: Record<string, any>
  defaults: Record<string, any>
  loading: boolean
  progress: number
  result: string
  resultItems: any[]
}

const router = useRouter()
const message = useMessage()
const auth = useAuthStore()

const loadingProject = ref(false)
const activeView = ref('config')
const showPwd = ref(false)
const oldPwd = ref('')
const newPwd = ref('')
const showMainMenuDrawer = ref(false)
const viewportWidth = ref(typeof window === 'undefined' ? 1280 : window.innerWidth)
const isMobile = computed(() => viewportWidth.value <= 900)
const credentialGridCols = computed(() => {
  if (viewportWidth.value <= 760) return 1
  if (viewportWidth.value <= 1260) return 2
  return 3
})

const credentials = ref<any[]>([])
const logs = ref<any[]>([])
const logPage = ref(1)
const logPageSize = ref(20)
const logTotal = ref(0)
const logPageSizeOptions = [20, 30, 50, 100, 200]
const cacheTTLSeconds = ref(600)
const cacheCountdownSeconds = ref(600)
let cacheCountdownTimer: number | null = null
let reloginInProgress = false
const projectFieldFocused = ref(false)
const selectedAction = reactive<Record<string, string>>({
  ad: '',
  print: '',
  vpn: '',
})

const menuOptions = [
  { label: '项目凭据', key: 'config' },
  { label: 'AD管理', key: 'ad' },
  { label: '打印管理', key: 'print' },
  { label: 'VPN管理', key: 'vpn' },
  { label: '操作日志', key: 'logs' },
]

const viewMetaMap: Record<string, { kicker: string; title: string; desc: string; tag: string }> = {
  config: {
    kicker: 'CREDENTIAL CENTER',
    title: '项目凭据配置中心',
    desc: '为当前管理员独立维护 AD、打印、VPN 与防火墙凭据。',
    tag: '安全配置',
  },
  ad: {
    kicker: 'AD MANAGEMENT',
    title: 'AD 域账号管理',
    desc: '新增、查询、改密、解锁与批量任务统一在此执行。',
    tag: '身份目录',
  },
  print: {
    kicker: 'PRINT HUB',
    title: '打印权限管理',
    desc: '管理打印用户信息、角色权限与账户维护操作。',
    tag: '打印系统',
  },
  vpn: {
    kicker: 'VPN CONTROL',
    title: 'VPN 访问管理',
    desc: '统一处理 VPN 账户新增、查询、改密、改状态与删除。',
    tag: '远程接入',
  },
  logs: {
    kicker: 'AUDIT LOGS',
    title: '操作日志审计',
    desc: '集中查看管理员行为与项目操作轨迹，支持分页检索。',
    tag: '行为审计',
  },
}

const currentViewMeta = computed(() => viewMetaMap[activeView.value] || viewMetaMap.config)

const adOrgUnitOptions = AD_ORG_UNIT_VALUES.map((value) => ({ label: value, value }))
const defaultAdOrgUnit = AD_ORG_UNIT_VALUES.includes('CLHD') ? 'CLHD' : (AD_ORG_UNIT_VALUES[0] ?? '')
const printSectionOptions = PRINT_SECTION_VALUES.map((value) => ({ label: value, value }))
const vpnSectionOptions = VPN_SECTION_VALUES.map((value) => ({ label: value, value }))
const credentialTypeTitleMap: Record<string, string> = {
  ad: 'AD',
  print: '打印',
  vpn: 'VPN',
  vpn_firewall: '防火墙',
}

function credentialTitle(projectType: string): string {
  const key = String(projectType || '').trim()
  if (!key) return ''
  return credentialTypeTitleMap[key] || key.toUpperCase()
}

function secureRandomInt(max: number): number {
  if (max <= 0) return 0
  const arr = new Uint32Array(1)
  crypto.getRandomValues(arr)
  return arr[0] % max
}

function generateAdPassword(length = 8): string {
  const uppers = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'
  const lowers = 'abcdefghijklmnopqrstuvwxyz'
  const digits = '0123456789'
  const all = `${uppers}${lowers}${digits}`
  const chars = [
    uppers[secureRandomInt(uppers.length)],
    lowers[secureRandomInt(lowers.length)],
    digits[secureRandomInt(digits.length)],
  ]
  while (chars.length < length) {
    chars.push(all[secureRandomInt(all.length)])
  }
  for (let i = chars.length - 1; i > 0; i -= 1) {
    const j = secureRandomInt(i + 1)
    ;[chars[i], chars[j]] = [chars[j], chars[i]]
  }
  return chars.join('')
}

const formMap = reactive<Record<string, ActionForm[]>>({
  ad: [
    form(
      '新增用户',
      'add_user',
      [
        t('sn', '姓'),
        t('given_name', '名'),
        t('cn', '姓名', { required: true }),
        t('username', '用户名', { required: true }),
        p('password', '密码', { required: true, masked: false, randomButton: true }),
        t('email', '邮箱', { required: true }),
        t('description', '描述'),
        sel('ou', '组织单位', adOrgUnitOptions, { required: true }),
      ],
      { ou: defaultAdOrgUnit },
    ),
    form('批量新增用户', 'batch_add_users', [fileField('excel_file', 'Excel 文件', { required: true })]),
    form('查询用户', 'search_user', [t('search_name', '搜索关键词', { required: true, placeholder: '可搜索姓名，工号或GID' })]),
    form(
      '重置密码',
      'reset_password',
      [
        t('name', '用户名', { required: true }),
        p('password', '新密码', { masked: false, randomButton: true }),
        sw('pwd_last_set', '用户下次登陆时须更改密码(U)'),
      ],
      { pwd_last_set: true },
    ),
    form('解锁用户', 'unlock_user', [t('name', '用户名', { required: true })]),
    form('修改描述', 'modify_description', [t('name', '用户名', { required: true }), t('description', '新描述')]),
    form('修改姓名', 'modify_name', [t('name', '用户名', { required: true }), t('sn', '姓'), t('given_name', '名'), t('cn', '姓名', { required: true })]),
    form('删除用户', 'delete_user', [t('name', '用户名', { required: true })]),
  ],
  print: [
    form(
      '新增用户',
      'add_user',
      [
        t('name', '用户名', { required: true }),
        t('fullname', '姓名', { required: true }),
        sel('sex', '性别', PRINT_GENDER_OPTIONS_ADD, { required: true, placeholder: '请选择性别' }),
        p('password', '密码', { required: true }),
        t('email', '邮箱', { required: true }),
        sel('section', '部门', printSectionOptions, { required: true, placeholder: '请选择部门' }),
      ],
      { name: '', fullname: '', sex: '', email: '', section: '', password: '123' },
    ),
    form(
      '查询用户',
      'search_user',
      [
        sel('search_key', '查询字段', PRINT_SEARCH_KEY_OPTIONS, { required: true }),
        t('search_content', '查询值', { required: true }),
      ],
      { search_key: 'fullname' },
    ),
    form(
      '重置密码',
      'reset_password',
      [
        sel('search_key', '查询字段', PRINT_SEARCH_KEY_OPTIONS, { required: true }),
        t('search_content', '查询值', { required: true }),
        p('password', '新密码', { required: true }),
      ],
      { search_key: 'fullname', password: '123' },
    ),
    form(
      '修改用户',
      'modify_user',
      [
        sel('search_key', '查询字段', PRINT_SEARCH_KEY_OPTIONS, { required: true, phase: 'query' }),
        t('search_content', '查询值', { required: true, phase: 'query' }),
        t('name', '用户名', { required: true, readonly: true, phase: 'edit' }),
        t('fullname', '姓名', { required: true, phase: 'edit' }),
        sel('sex', '性别', PRINT_GENDER_OPTIONS_MODIFY, { required: true, phase: 'edit' }),
        sel('status', '状态', PRINT_STATUS_OPTIONS, { required: true, phase: 'edit' }),
        t('email', '邮箱', { phase: 'edit' }),
        sel('section', '部门', printSectionOptions, { required: true, phase: 'edit', placeholder: '请选择部门' }),
        sel('roles', '角色', PRINT_ROLE_OPTIONS, { required: true, phase: 'edit', multiple: true, placeholder: '请选择角色' }),
      ],
      { search_key: 'fullname', status: 'enabled', roles: [], __mode: 'query' },
    ),
    form(
      '删除用户',
      'delete_user',
      [
        sel('search_key', '查询字段', PRINT_SEARCH_KEY_OPTIONS, { required: true }),
        t('search_content', '查询值', { required: true }),
      ],
      { search_key: 'fullname' },
    ),
  ],
  vpn: [
    form(
      '新增用户',
      'add_user',
      [
        t('vpn_user', '用户名', { required: true }),
        p('passwd', '新密码', { masked: false, randomButton: true }),
        t('description', '描述', { required: true }),
        t('mail', '邮箱', { required: true }),
        sel('section', '所属父组', vpnSectionOptions, { required: true, placeholder: '请选择所属父组' }),
        sel('status', '状态', [
          { label: '启用', value: 'enabled' },
          { label: '禁用', value: 'disabled' },
        ], { required: true }),
      ],
      { status: 'enabled', section: 'default^root' },
    ),
    form('查询用户', 'search_user', [t('search_description', '描述', { required: true })]),
    form(
      '修改密码',
      'modify_password',
      [t('modify_password_description', '描述', { required: true }), p('passwd', '新密码', { masked: false, randomButton: true })],
    ),
    form(
      '修改状态',
      'modify_status',
      [
        t('modify_status_description', '描述', { required: true }),
        sel('status', '状态', [
          { label: '启用', value: 'enabled' },
          { label: '禁用', value: 'disabled' },
        ], { required: true }),
      ],
      { status: 'enabled' },
    ),
    form(
      '删除用户',
      'delete_users',
      [
        t('vpn_user', '用户名', { required: true, placeholder: '多用户可用 , ; / 三种符号隔开' }),
        sw('remote_firewall', '同步删除防火墙上的VPN账户'),
      ],
      { remote_firewall: false },
    ),
  ],
})
const isProjectView = computed(() => ['ad', 'print', 'vpn'].includes(activeView.value))

const currentProjectForms = computed(() => {
  if (!isProjectView.value) return []
  return formMap[activeView.value] || []
})

const currentProjectAction = computed(() => {
  if (!isProjectView.value) return ''
  const key = activeView.value
  const saved = selectedAction[key]
  if (saved && currentProjectForms.value.some((f) => f.action === saved)) {
    return saved
  }
  return currentProjectForms.value[0]?.action || ''
})

const projectFunctionMenuOptions = computed(() => currentProjectForms.value.map((f) => ({ label: f.title, key: f.action })))
const projectFunctionSelectOptions = computed(() =>
  currentProjectForms.value.map((f) => ({
    label: f.title,
    value: f.action,
  })),
)

const currentProjectForm = computed(() => {
  return currentProjectForms.value.find((f) => f.action === currentProjectAction.value) || currentProjectForms.value[0]
})

const adAddUserForm = computed(() => formMap.ad.find((x) => x.action === 'add_user'))
const adResetPasswordForm = computed(() => formMap.ad.find((x) => x.action === 'reset_password'))
const adModifyNameForm = computed(() => formMap.ad.find((x) => x.action === 'modify_name'))
const vpnAddUserForm = computed(() => formMap.vpn.find((x) => x.action === 'add_user'))
const vpnModifyPasswordForm = computed(() => formMap.vpn.find((x) => x.action === 'modify_password'))
const printModifyUserForm = computed(() => formMap.print.find((x) => x.action === 'modify_user'))

function isPrintModifyForm(form?: ActionForm): boolean {
  return activeView.value === 'print' && !!form && form.action === 'modify_user'
}

function isPrintModifyEditMode(form?: ActionForm): boolean {
  return isPrintModifyForm(form) && String(form?.model?.__mode || 'query') === 'edit'
}

function visibleFields(form?: ActionForm): Field[] {
  if (!form) return []
  if (!isPrintModifyForm(form)) return form.fields
  const mode: FieldPhase = isPrintModifyEditMode(form) ? 'edit' : 'query'
  return form.fields.filter((field) => !field.phase || field.phase === 'all' || field.phase === mode)
}

function resetPrintModifyModel(form?: ActionForm) {
  if (!form) return
  form.model.__mode = 'query'
  form.model.search_key = form.model.search_key || 'fullname'
  form.model.search_content = ''
  form.model.name = ''
  form.model.fullname = ''
  form.model.sex = ''
  form.model.status = 'enabled'
  form.model.email = ''
  form.model.section = ''
  form.model.roles = []
  form.model.__user_id = ''
  form.model.__ori_email = ''
}

function resetActionFormModel(form?: ActionForm) {
  if (!form) return
  form.model = { ...form.defaults }
}

function setProjectDefaultAction(project: string) {
  const forms = formMap[project] || []
  if (!forms.length) return
  selectedAction[project] = forms[0].action
}

function onProjectFunctionSelect(action: string) {
  if (!isProjectView.value) return
  selectedAction[activeView.value] = action
  if (activeView.value === 'ad' && action === 'add_user') {
    initAdAddUserDefaults()
  }
  if (activeView.value === 'ad' && action === 'reset_password') {
    initAdResetPasswordDefaults()
  }
  if (activeView.value === 'vpn' && action === 'add_user') {
    initVpnAddUserDefaults()
  }
  if (activeView.value === 'vpn' && action === 'modify_password') {
    initVpnModifyPasswordDefaults()
  }
  if (activeView.value === 'print' && action === 'modify_user') {
    resetPrintModifyModel(printModifyUserForm.value)
  }
}

function syncAdFullName() {
  const f = adAddUserForm.value
  if (!f) return
  const sn = String(f.model.sn || '').trim()
  const givenName = String(f.model.given_name || '').trim()
  f.model.cn = `${sn}${givenName}`
}

function initAdAddUserDefaults() {
  const f = adAddUserForm.value
  if (!f) return
  f.model.password = generateAdPassword()
  if (!f.model.ou) {
    f.model.ou = defaultAdOrgUnit
  }
  syncAdFullName()
}

function initAdResetPasswordDefaults() {
  const f = adResetPasswordForm.value
  if (!f) return
  f.model.password = generateAdPassword()
}

function initVpnAddUserDefaults() {
  const f = vpnAddUserForm.value
  if (!f) return
  f.model.passwd = generateAdPassword()
}

function initVpnModifyPasswordDefaults() {
  const f = vpnModifyPasswordForm.value
  if (!f) return
  f.model.passwd = generateAdPassword()
}

watch(
  () => [adAddUserForm.value?.model.sn, adAddUserForm.value?.model.given_name],
  () => syncAdFullName(),
  { immediate: true },
)

function syncAdModifyNameFullName() {
  const f = adModifyNameForm.value
  if (!f) return
  const sn = String(f.model.sn || '').trim()
  const givenName = String(f.model.given_name || '').trim()
  if (sn || givenName) {
    f.model.cn = `${sn}${givenName}`
  }
}

watch(
  () => [adModifyNameForm.value?.model.sn, adModifyNameForm.value?.model.given_name],
  () => syncAdModifyNameFullName(),
  { immediate: true },
)

function parsePrintRoleIDsFromNames(roleNames: string): string[] {
  const ids: string[] = []
  for (const one of String(roleNames || '').split('|')) {
    const key = one.trim()
    if (!key) continue
    const id = PRINT_ROLE_NAME_TO_ID[key]
    if (id && !ids.includes(id)) {
      ids.push(id)
    }
  }
  return ids
}

function normalizePrintRoleIDs(value: any, roleNames = ''): string[] {
  const raw: string[] = []
  if (Array.isArray(value)) {
    for (const one of value) {
      const id = String(one || '').trim()
      if (id) raw.push(id)
    }
  } else {
    const s = String(value || '').trim()
    if (s) {
      for (const one of s.split(',')) {
        const id = one.trim()
        if (id) raw.push(id)
      }
    }
  }
  if (!raw.length && roleNames) {
    raw.push(...parsePrintRoleIDsFromNames(roleNames))
  }
  return Array.from(new Set(raw))
}

function normalizePathText(value: any): string {
  let s = String(value ?? '')
  while (s.includes('\\\\')) {
    s = s.replace(/\\\\/g, '\\')
  }
  return s
}

function normalizePrintItems(items: any[]): any[] {
  return items.map((item) => {
    if (!item || typeof item !== 'object') return item
    return {
      ...item,
      dept: normalizePathText((item as any).dept),
      section: normalizePathText((item as any).section),
    }
  })
}

function stringifyResult(data: any, normalizePath = false): string {
  const text = JSON.stringify(data, null, 2)
  if (!normalizePath) return text
  return text.replace(/\\\\/g, '\\')
}

async function confirmPrintModify(form: ActionForm) {
  form.loading = true
  try {
    const searchKey = String(form.model.search_key || 'fullname').trim() || 'fullname'
    const searchContent = String(form.model.search_content || '').trim()
    if (!searchContent) {
      throw new Error('查询值不能为空')
    }
    const res = await apiRequest('/api/projects/print/operate', 'POST', {
      action: 'get_user',
      params: {
        search_key: searchKey,
        search_content: searchContent,
      },
    })
    const item = res?.data?.item || {}
    form.model.name = String(item.name || '')
    form.model.fullname = String(item.fullname || '')
    form.model.sex = String(item.sex || '')
    form.model.status = String(item.status || 'enabled')
    form.model.email = String(item.email || '')
    form.model.section = normalizePathText(item.section || '')
    form.model.roles = normalizePrintRoleIDs(item.role_ids, String(item.role_names || ''))
    form.model.__user_id = String(item.id || '')
    form.model.__ori_email = String(item.email || '')
    form.model.__mode = 'edit'
    form.resultItems = []
    form.result = stringifyResult(res, true)
    message.success('user info loaded')
  } catch (e: any) {
    form.resultItems = []
    form.result = '错误：' + e.message
    handleRequestError(e)
  } finally {
    form.loading = false
  }
}

function backPrintModify(form: ActionForm) {
  resetPrintModifyModel(form)
}

type FieldMeta = Pick<Field, 'required' | 'readonly' | 'masked' | 'placeholder' | 'randomButton' | 'multiple' | 'phase'>

function t(key: string, label: string, meta: FieldMeta = {}): Field {
  return { key, label, type: 'text', ...meta }
}

function p(key: string, label: string, meta: FieldMeta = {}): Field {
  return { key, label, type: 'password', ...meta }
}

function ta(key: string, label: string, meta: FieldMeta = {}): Field {
  return { key, label, type: 'textarea', ...meta }
}

function sw(key: string, label: string, meta: FieldMeta = {}): Field {
  return { key, label, type: 'switch', ...meta }
}

function sel(key: string, label: string, options: any[], meta: FieldMeta = {}): Field {
  return { key, label, type: 'select', options, ...meta }
}

function fileField(key: string, label: string, meta: FieldMeta = {}): Field {
  return { key, label, type: 'file', ...meta }
}

function form(title: string, action: string, fields: Field[], defaults: Record<string, any> = {}): ActionForm {
  const initial: Record<string, any> = {}
  for (const field of fields) {
    initial[field.key] = field.type === 'switch' ? false : ''
  }
  const merged = { ...initial, ...defaults }
  return {
    title,
    action,
    fields,
    model: { ...merged },
    defaults: { ...merged },
    loading: false,
    progress: 0,
    result: '',
    resultItems: [],
  }
}

function fillRandomPassword(form: ActionForm, key: string) {
  form.model[key] = generateAdPassword()
}

function handleRequestError(err: any, fallback = '请求失败') {
  if (isAuthExpiredError(err)) {
    return
  }
  const msg = String(err?.message || '').trim()
  message.error(msg || fallback)
}

async function downloadAdBatchTemplate() {
  try {
    const headers: Record<string, string> = {}
    if (auth.token) {
      headers.Authorization = `Bearer ${auth.token}`
    }
    const res = await fetch(`${auth.apiBase}/api/projects/ad/batch-template`, {
      method: 'GET',
      headers,
    })
    if (res.status === 401) {
      auth.clearSession()
      await router.replace('/login')
      return
    }
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.error || '下载模板失败')
    }
    const blob = await res.blob()
    let filename = '创建AD用户模板.xlsx'
    const disposition = res.headers.get('content-disposition') || ''
    const m = disposition.match(/filename\*=UTF-8''([^;]+)/i)
    if (m && m[1]) {
      filename = decodeURIComponent(m[1])
    }
    const link = document.createElement('a')
    const url = URL.createObjectURL(blob)
    link.href = url
    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  } catch (e: any) {
    handleRequestError(e, '下载模板失败')
  }
}

async function handleAdBatchFileUpload(options: any, form: ActionForm, field: Field) {
  try {
    const rawFile: File | undefined =
      options?.file?.file ||
      options?.file?.blobFile ||
      options?.file
    if (!rawFile) {
      throw new Error('未获取到上传文件')
    }
    const formData = new FormData()
    formData.append('file', rawFile)
    formData.append('old_file', String(form.model[field.key] || ''))

    const headers: Record<string, string> = {}
    if (auth.token) {
      headers.Authorization = `Bearer ${auth.token}`
    }
    const res = await fetch(`${auth.apiBase}/api/projects/ad/batch-upload`, {
      method: 'POST',
      headers,
      body: formData,
    })
    if (res.status === 401) {
      auth.clearSession()
      await router.replace('/login')
      return
    }
    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      throw new Error(data.error || '上传失败')
    }
    form.model[field.key] = String(data.name || '')
    form.resultItems = []
    form.result = ''
    options?.onFinish?.()
    message.success(`上传成功：${form.model[field.key]}`)
  } catch (e: any) {
    options?.onError?.()
    handleRequestError(e, '上传失败')
  }
}

function formatCountdown(value: number): string {
  const safe = Math.max(0, Math.floor(value))
  const mm = String(Math.floor(safe / 60)).padStart(2, '0')
  const ss = String(safe % 60).padStart(2, '0')
  return `${mm}:${ss}`
}

function onProjectFieldFocus() {
  projectFieldFocused.value = true
}

function onProjectFieldBlur() {
  window.setTimeout(() => {
    projectFieldFocused.value = false
  }, 0)
}

function isAnyProjectActionRunning(): boolean {
  return ['ad', 'print', 'vpn'].some((projectKey) => {
    const forms = formMap[projectKey] || []
    return forms.some((form) => form.loading)
  })
}

function resetCacheCountdown() {
  cacheCountdownSeconds.value = cacheTTLSeconds.value
}

async function loadAuthMeta() {
  try {
    const data = await apiRequest('/api/auth/me')
    const ttl = Number(data?.project_cache_ttl_seconds || 0)
    if (Number.isFinite(ttl) && ttl > 0) {
      cacheTTLSeconds.value = ttl
      resetCacheCountdown()
    }
  } catch (e) {
    // keep default TTL
  }
}

async function reloginProjectsSilently() {
  if (reloginInProgress) return
  reloginInProgress = true
  try {
    const data = await apiRequest('/api/projects/relogin', 'POST', {})
    auth.clearLoadedProjects()
    const items = Array.isArray(data?.items) ? data.items : []
    for (const item of items) {
      const key = String(item?.project_type || '').trim()
      if (!key) continue
      if (item?.ok) {
        auth.markProjectLoaded(key)
      }
    }
  } catch (e) {
    // silent refresh
  } finally {
    reloginInProgress = false
    resetCacheCountdown()
  }
}

function startCacheCountdownTimer() {
  if (cacheCountdownTimer !== null) {
    window.clearInterval(cacheCountdownTimer)
  }
  cacheCountdownTimer = window.setInterval(async () => {
    if (!auth.token) return
    if (projectFieldFocused.value) return
    if (loadingProject.value || isAnyProjectActionRunning()) return
    if (cacheCountdownSeconds.value > 0) {
      cacheCountdownSeconds.value -= 1
      return
    }
    await reloginProjectsSilently()
  }, 1000)
}

async function loadCredentials() {
  const data = await apiRequest('/api/projects/credentials')
  credentials.value = data.items || []
}

async function saveCredential(item: any) {
  try {
    await apiRequest(`/api/projects/credentials/${item.project_type}`, 'PUT', {
      account: item.account,
      password: item.password,
    })
    auth.resetProjectLoaded(item.project_type)
    message.success(`${credentialTitle(item.project_type)}凭据保存成功`)
  } catch (e: any) {
    handleRequestError(e)
  }
}

async function ensureLoaded(project: string) {
  if (auth.loadedProjects[project]) return
  loadingProject.value = true
  try {
    await apiRequest(`/api/projects/${project}/load`, 'POST', {})
    auth.markProjectLoaded(project)
  } finally {
    loadingProject.value = false
  }
}

function isEmptyRequiredValue(value: any): boolean {
  if (value === null || value === undefined) return true
  if (Array.isArray(value)) return value.length === 0
  if (typeof value === 'string') return value.trim() === ''
  return false
}

type AsyncOperateJobResp = {
  job_id: string
  status: string
  ok: boolean
  done: boolean
  message: string
  error: string
  progress: number
  processed: number
  total: number
  log_lines: string[]
  result_text: string
  result_items: any[]
}

function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

function updateFormProgressState(f: ActionForm, job: AsyncOperateJobResp) {
  const progress = Number(job?.progress || 0)
  f.progress = Number.isFinite(progress) ? Math.max(0, Math.min(100, Math.floor(progress))) : 0

  const logLines = Array.isArray(job?.log_lines) ? job.log_lines.filter((x) => String(x || '').trim() !== '') : []
  if (logLines.length > 0) {
    f.result = logLines.join('\n')
  }
  if (Array.isArray(job?.result_items)) {
    f.resultItems = job.result_items
  }
  if (String(job?.result_text || '').trim()) {
    f.result = String(job.result_text).trim()
  }
}

async function runAsyncProjectAction(f: ActionForm, projectType: string, action: string, params: Record<string, any>) {
  const start = await apiRequest('/api/projects/operate-async', 'POST', {
    project_type: projectType,
    action,
    params,
  })
  const jobID = String(start?.job_id || '').trim()
  if (!jobID) {
    throw new Error('创建异步任务失败')
  }

  const timeoutAt = Date.now() + 30 * 60 * 1000
  while (true) {
    const job = (await apiRequest(`/api/projects/operate-async/${encodeURIComponent(jobID)}`)) as AsyncOperateJobResp
    updateFormProgressState(f, job)
    if (job?.done) {
      return job
    }
    if (Date.now() >= timeoutAt) {
      throw new Error('任务执行超时，请稍后重试')
    }
    await sleep(500)
  }
}

async function submitAction(f: ActionForm) {
  f.loading = true
  f.progress = 1
  f.resultItems = []
  f.result = '任务已提交，正在执行...'
  try {
    const fields = visibleFields(f)
    const missing = fields
      .filter((field) => field.required && isEmptyRequiredValue(f.model[field.key]))
      .map((field) => field.label)
    if (missing.length) {
      throw new Error('必填项不能为空：' + missing.join('、'))
    }

    const params = { ...f.model }
    delete params.__mode
    delete params.__user_id
    delete params.__ori_email

    if (f.action === 'batch_add_users') {
      params.excel_file = String(params.excel_file || '').trim()
    }
    if (f.action === 'delete_users') {
      params.vpn_users = String(params.vpn_user || '')
        .split(/[,/;]+/)
        .map((x: string) => x.trim())
        .filter(Boolean)
      delete params.vpn_user
    }

    const emailRegex = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/
    const strongPasswordRegex = /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,}$/

    if (activeView.value === 'ad' && f.action === 'add_user') {
      params.password = String(params.password || '').trim()
      const email = String(params.email || '').trim()
      if (!strongPasswordRegex.test(params.password)) {
        throw new Error('密码需至少8位，且包含大小写字母和数字')
      }
      if (!emailRegex.test(email)) {
        throw new Error('邮箱格式不正确')
      }
    }
    if (activeView.value === 'ad' && f.action === 'reset_password') {
      params.password = String(params.password || '').trim()
      if (params.password && !strongPasswordRegex.test(params.password)) {
        throw new Error('密码需至少8位，且包含大小写字母和数字')
      }
    }

    if (activeView.value === 'print' && f.action === 'add_user') {
      params.name = String(params.name || '').trim()
      params.fullname = String(params.fullname || '').trim()
      params.sex = String(params.sex || '').trim()
      params.password = String(params.password || '').trim()
      params.email = String(params.email || '').trim()
      params.section = String(params.section || '').trim()
      if (!emailRegex.test(params.email)) {
        throw new Error('邮箱格式不正确')
      }
    }

    if (activeView.value === 'print' && (f.action === 'search_user' || f.action === 'reset_password' || f.action === 'delete_user')) {
      params.search_content = String(params.search_content || '').trim()
      if (!params.search_content) {
        throw new Error('查询值不能为空')
      }
    }

    if (activeView.value === 'print' && f.action === 'modify_user') {
      if (!isPrintModifyEditMode(f)) {
        throw new Error('请先点击确认查询')
      }
      params.name = String(params.name || '').trim()
      params.fullname = String(params.fullname || '').trim()
      params.sex = String(params.sex || '').trim()
      params.status = String(params.status || '').trim()
      params.email = String(params.email || '').trim()
      params.section = String(params.section || '').trim()
      params.user_id = String(f.model.__user_id || '').trim()
      params.ori_email = String(f.model.__ori_email || '').trim()
      const roleIDs = normalizePrintRoleIDs(params.roles)
      if (!roleIDs.length) {
        throw new Error('角色不能为空')
      }
      params.roles = roleIDs.join(',')
      if (params.email && !emailRegex.test(params.email)) {
        throw new Error('邮箱格式不正确')
      }
    }
    if (activeView.value === 'vpn' && f.action === 'add_user') {
      params.vpn_user = String(params.vpn_user || '').trim()
      params.passwd = String(params.passwd || '').trim()
      params.description = String(params.description || '').trim()
      params.mail = String(params.mail || '').trim()
      params.section = String(params.section || '').trim()
      params.status = String(params.status || '').trim()
      if (params.passwd && !strongPasswordRegex.test(params.passwd)) {
        throw new Error('密码需至少8位，且包含大小写字母和数字')
      }
      if (!emailRegex.test(params.mail)) {
        throw new Error('邮箱格式不正确')
      }
    }
    if (activeView.value === 'vpn' && f.action === 'search_user') {
      params.description = String(params.search_description || '').trim()
      if (!params.description) {
        throw new Error('描述不能为空')
      }
      delete params.search_description
    }
    if (activeView.value === 'vpn' && f.action === 'modify_password') {
      params.description = String(params.modify_password_description || '').trim()
      params.passwd = String(params.passwd || '').trim()
      if (!params.description) {
        throw new Error('描述不能为空')
      }
      if (params.passwd && !strongPasswordRegex.test(params.passwd)) {
        throw new Error('密码需至少8位，且包含大小写字母和数字')
      }
      delete params.modify_password_description
    }
    if (activeView.value === 'vpn' && f.action === 'modify_status') {
      params.description = String(params.modify_status_description || '').trim()
      params.status = String(params.status || '').trim()
      if (!params.description) {
        throw new Error('描述不能为空')
      }
      delete params.modify_status_description
    }
    if (activeView.value === 'vpn' && f.action === 'delete_users') {
      if (!Array.isArray(params.vpn_users) || params.vpn_users.length === 0) {
        throw new Error('用户名不能为空')
      }
    }

    const job = await runAsyncProjectAction(f, activeView.value, f.action, params)
    let resultItems = Array.isArray(job?.result_items) ? job.result_items : []
    if (activeView.value === 'print') {
      resultItems = normalizePrintItems(resultItems)
    }
    f.resultItems = resultItems
    const resultText = String(job?.result_text || '').trim()
    if (resultText) {
      f.result = resultText
    }
    if (!job?.ok) {
      throw new Error(String(job?.error || job?.message || '执行失败'))
    }

    f.progress = 100
    if (activeView.value === 'print' && f.action === 'add_user') {
      resetActionFormModel(f)
    }
    if (activeView.value === 'ad' && f.action === 'add_user') {
      resetActionFormModel(f)
      initAdAddUserDefaults()
    }
    if (activeView.value === 'vpn') {
      resetActionFormModel(f)
      if (f.action === 'add_user') {
        initVpnAddUserDefaults()
      }
      if (f.action === 'modify_password') {
        initVpnModifyPasswordDefaults()
      }
    }
    if (activeView.value === 'print' && f.action === 'modify_user') {
      backPrintModify(f)
    }
    // AD 管理：除已单独处理的功能外，执行成功后恢复初始值
    if (activeView.value === 'ad' && f.action !== 'add_user') {
      resetActionFormModel(f)
      if (f.action === 'reset_password') {
        initAdResetPasswordDefaults()
      }
    }
    // 打印管理：除已单独处理的功能外，执行成功后恢复初始值
    if (activeView.value === 'print' && f.action !== 'add_user' && f.action !== 'modify_user') {
      resetActionFormModel(f)
    }
    message.success(job?.message || 'success')
  } catch (e: any) {
    const errMsg = String(e?.message || '执行失败')
    if (!String(f.result || '').trim()) {
      f.result = '错误：' + errMsg
    } else if (!String(f.result).includes(errMsg)) {
      f.result = `${f.result}\n错误：${errMsg}`
    }
    handleRequestError(e)
  } finally {
    f.loading = false
  }
}

async function loadLogs(page = logPage.value, pageSize = logPageSize.value) {
  const data = await apiRequest(`/api/logs?page=${page}&page_size=${pageSize}`)
  logs.value = data.items || []
  logTotal.value = Number(data.total || 0)
  logPage.value = Number(data.page || page || 1)
  logPageSize.value = Number(data.page_size || pageSize || 20)
}

async function handleLogPageChange(page: number) {
  logPage.value = page
  await loadLogs(page, logPageSize.value)
}

async function handleLogPageSizeChange(size: number) {
  logPageSize.value = size
  logPage.value = 1
  await loadLogs(1, size)
}

function syncViewportState() {
  viewportWidth.value = window.innerWidth
  if (!isMobile.value) {
    showMainMenuDrawer.value = false
  }
}

function onMobileMenuSelect(key: string) {
  showMainMenuDrawer.value = false
  void onMenuSelect(key)
}
async function onMenuSelect(key: string) {
  if (key === 'ad' || key === 'print' || key === 'vpn') {
    try {
      await ensureLoaded(key)
    } catch (e: any) {
      handleRequestError(e)
      return
    }
  }

  activeView.value = key

  if (key === 'ad' || key === 'print' || key === 'vpn') {
    setProjectDefaultAction(key)
  }
  if (key === 'ad') {
    initAdAddUserDefaults()
    initAdResetPasswordDefaults()
  }
  if (key === 'vpn') {
    initVpnAddUserDefaults()
    initVpnModifyPasswordDefaults()
  }
  if (key === 'print') {
    resetPrintModifyModel(printModifyUserForm.value)
  }
  if (key === 'config') await loadCredentials()
  if (key === 'logs') await loadLogs(logPage.value, logPageSize.value)
}

async function changePassword() {
  try {
    await apiRequest('/api/auth/change-password', 'POST', {
      old_password: oldPwd.value,
      new_password: newPwd.value,
    })
    message.success('密码修改成功')
    oldPwd.value = ''
    newPwd.value = ''
    showPwd.value = false
  } catch (e: any) {
    handleRequestError(e)
  }
}

function logout() {
  if (cacheCountdownTimer !== null) {
    window.clearInterval(cacheCountdownTimer)
    cacheCountdownTimer = null
  }
  auth.clearSession()
  router.push('/login')
}

function ensureSessionAliveBeforeAction() {
  if (!auth.token) {
    return false
  }
  if (auth.isSessionValid()) {
    return true
  }
  if (cacheCountdownTimer !== null) {
    window.clearInterval(cacheCountdownTimer)
    cacheCountdownTimer = null
  }
  auth.clearSession()
  void router.replace('/login')
  return false
}

function handleVisibilityOrFocus() {
  if (document.visibilityState === 'hidden') {
    return
  }
  ensureSessionAliveBeforeAction()
}

onMounted(async () => {
  document.addEventListener('visibilitychange', handleVisibilityOrFocus)
  window.addEventListener('focus', handleVisibilityOrFocus)
  window.addEventListener('resize', syncViewportState)
  syncViewportState()
  if (!ensureSessionAliveBeforeAction()) {
    return
  }
  await loadAuthMeta()
  startCacheCountdownTimer()
  loadCredentials().catch((e) => handleRequestError(e))
})

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', handleVisibilityOrFocus)
  window.removeEventListener('focus', handleVisibilityOrFocus)
  window.removeEventListener('resize', syncViewportState)
  if (cacheCountdownTimer !== null) {
    window.clearInterval(cacheCountdownTimer)
    cacheCountdownTimer = null
  }
})
</script>

<style scoped>
.required-star {
  margin-left: 4px;
  color: var(--ops-danger);
  font-weight: 700;
}

.page-layout {
  height: 100vh;
  height: 100dvh;
  position: relative;
  background: transparent;
}

.page-layout::before {
  content: '';
  position: fixed;
  inset: 0;
  pointer-events: none;
  background:
    radial-gradient(820px 320px at -10% 10%, rgba(72, 141, 196, 0.2), transparent 55%),
    radial-gradient(840px 360px at 110% -12%, rgba(43, 170, 156, 0.18), transparent 58%);
  z-index: 0;
}

.main-sider {
  z-index: 1;
  background: linear-gradient(180deg, #1a5b88 0%, #1a5078 52%, #174664 100%);
}

.sider-title {
  height: 58px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 17px;
  font-weight: 700;
  color: #f0f8ff;
  border-bottom: 1px solid rgba(255, 255, 255, 0.12);
  letter-spacing: 0.05em;
}

.main-menu {
  padding: 10px 8px;
}

.main-menu :deep(.n-menu-item-content) {
  margin: 2px 4px;
  border-radius: 10px;
  color: #f4f9ff;
}

.main-menu :deep(.n-menu-item-content .n-menu-item-content-header),
.main-menu :deep(.n-menu-item-content .n-menu-item-content__icon),
.main-menu :deep(.n-menu-item-content .n-menu-item-content__arrow) {
  color: #f4f9ff;
}

.main-menu :deep(.n-menu-item-content:hover) {
  background: rgba(255, 255, 255, 0.16);
}

.main-menu :deep(.n-menu-item-content--selected) {
  color: #0f4068 !important;
  background: linear-gradient(120deg, #d8ecfa, #f0f8ff) !important;
  font-weight: 700;
}

.page-content {
  z-index: 1;
  flex: 1;
  min-height: 0;
  overflow: auto;
}

.main-shell {
  height: 100vh;
  height: 100dvh;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.view-banner {
  position: relative;
  margin-bottom: 14px;
  border: 1px solid #d3e2ef;
  border-radius: 14px;
  padding: 14px 16px;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  background: linear-gradient(120deg, rgba(255, 255, 255, 0.86), rgba(243, 249, 255, 0.82));
  box-shadow: 0 14px 34px rgba(16, 67, 108, 0.1);
}

.view-banner-main {
  min-width: 0;
}

.view-banner-kicker {
  font-size: 11px;
  letter-spacing: 0.12em;
  color: #4f7da0;
  font-weight: 700;
}

.view-banner-title {
  margin-top: 4px;
  font-size: 22px;
  line-height: 1.2;
  font-weight: 800;
  color: #0f385a;
}

.view-banner-desc {
  margin-top: 6px;
  font-size: 13px;
  color: #4f708d;
  line-height: 1.6;
}

.view-banner-tags {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.view-tag {
  font-size: 12px;
  border-radius: 999px;
  border: 1px solid #bfd6e8;
  color: #1b5a87;
  background: #e9f4fc;
  padding: 5px 11px;
}

.view-tag--soft {
  color: #375c79;
  background: #f2f8fe;
}

.view-banner--ad {
  border-color: #cce1f1;
  background: linear-gradient(120deg, rgba(241, 249, 255, 0.92), rgba(230, 245, 255, 0.82));
}

.view-banner--print {
  border-color: #cfe2f5;
  background: linear-gradient(120deg, rgba(244, 250, 255, 0.93), rgba(235, 246, 255, 0.84));
}

.view-banner--vpn {
  border-color: #c7e6df;
  background: linear-gradient(120deg, rgba(242, 252, 249, 0.94), rgba(233, 248, 243, 0.84));
}

.view-banner--logs {
  border-color: #d8dfec;
  background: linear-gradient(120deg, rgba(249, 251, 255, 0.93), rgba(241, 246, 255, 0.84));
}

.view-banner--config {
  border-color: #d6e2ee;
}

.topbar {
  height: 58px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 18px;
  background: rgba(255, 255, 255, 0.72);
  backdrop-filter: blur(10px);
}

.top-main {
  display: flex;
  align-items: center;
  gap: 8px;
}

.mobile-menu-btn {
  border-radius: 8px;
  border: 1px solid #c8dced;
  background: rgba(255, 255, 255, 0.92);
}

.mobile-action-switch {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 12px;
  padding: 10px;
  border: 1px solid #d6e5f1;
  border-radius: 12px;
  background: #f7fbff;
}

.mobile-action-switch__label {
  flex-shrink: 0;
  font-size: 13px;
  color: #355c7a;
  font-weight: 600;
}
.top-title {
  font-size: 19px;
  font-weight: 700;
  color: #0f385a;
}

.top-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #295070;
  font-size: 13px;
}

.cache-countdown {
  color: #476f91;
  font-size: 12px;
  border: 1px solid #c9dded;
  background: #f2f8fe;
  border-radius: 999px;
  padding: 4px 10px;
}

.logs-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.project-layout {
  min-height: 0;
  border: 1px solid #d3e2ef;
  border-radius: 14px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.84);
  backdrop-filter: blur(6px);
  box-shadow: 0 18px 42px rgba(18, 66, 106, 0.12);
}

.project-sider {
  background: linear-gradient(180deg, #f7fbff 0%, #eef5fb 100%);
}

.project-menu {
  padding: 8px;
}

.project-menu :deep(.n-menu-item-content) {
  margin: 2px 3px;
  border-radius: 8px;
}

.project-menu :deep(.n-menu-item-content--selected) {
  background: linear-gradient(120deg, #dceefd, #f4faff) !important;
  color: #15517c !important;
  font-weight: 700;
}

.project-func-title {
  height: 50px;
  display: flex;
  align-items: center;
  padding: 0 14px;
  border-bottom: 1px solid #dce9f3;
  font-weight: 600;
  color: #204866;
  background: #f2f8fe;
}

.project-func-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  margin-right: 8px;
  background: #89abc6;
}

.project-func-dot--ad {
  background: #1d74b3;
}

.project-func-dot--print {
  background: #2f82bf;
}

.project-func-dot--vpn {
  background: #159d83;
}

.func-panel {
  display: grid;
  grid-template-columns: minmax(340px, 440px) 1fr;
  gap: 18px;
}

.func-input {
  min-width: 0;
  background: #ffffff;
  border: 1px solid #dbe8f2;
  border-radius: 12px;
  padding: 12px 12px 8px;
}

.func-output {
  min-width: 0;
  background: #ffffff;
  border: 1px solid #dbe8f2;
  border-radius: 12px;
  padding: 12px;
}

.result-title {
  font-size: 14px;
  font-weight: 700;
  margin-bottom: 10px;
  color: #204866;
}

.result-progress {
  margin-bottom: 10px;
}

.upload-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.upload-title {
  font-size: 14px;
  font-weight: 700;
  color: #1d4f76;
}

.upload-tip {
  font-size: 12px;
  color: #4f7698;
}

.upload-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.upload-file-name {
  font-size: 12px;
  color: #234e73;
}

.dual-action-buttons {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.upload-block :deep(.n-upload),
.upload-block :deep(.n-upload-trigger),
.upload-block :deep(.n-upload-dragger) {
  width: 100%;
}

.result-box {
  margin: 0;
  padding: 12px;
  min-height: clamp(180px, 24vh, 300px);
  max-height: 56vh;
  overflow: auto;
  background: #f6fbff;
  border: 1px solid #d6e5f1;
  border-radius: 10px;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.6;
  color: #264f71;
}

.batch-result-table :deep(table) {
  table-layout: fixed;
  width: 100%;
}

.batch-result-table :deep(th),
.batch-result-table :deep(td) {
  text-align: center;
  vertical-align: middle;
}

.error-reason-cell {
  width: 320px;
}

.error-reason-ellipsis {
  display: inline-block;
  width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.logs-table-wrap {
  overflow-x: auto;
}

.logs-table {
  min-width: 760px;
}

.mobile-drawer-menu {
  margin-top: 4px;
}

.mobile-drawer-menu :deep(.n-menu-item-content) {
  border-radius: 10px;
}
.logs-table :deep(th),
.logs-table :deep(td) {
  text-align: center;
  vertical-align: middle;
}

.logs-table :deep(th) {
  background: #eff7ff;
  color: #1e4f78;
}

.page-content :deep(.n-card-header__main) {
  font-weight: 700;
  color: #1a466d;
}

.project-action-card {
  position: relative;
}

.project-action-card::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 4px;
  border-radius: 16px 0 0 16px;
  background: #8cb4d4;
}

.project-action-card--ad::before {
  background: linear-gradient(180deg, #2e8ccd, #1264a1);
}

.project-action-card--print::before {
  background: linear-gradient(180deg, #4a95cf, #246ea8);
}

.project-action-card--vpn::before {
  background: linear-gradient(180deg, #36b49b, #138d75);
}

.page-content :deep(.n-upload-dragger) {
  border-radius: 10px;
  background: linear-gradient(180deg, #f8fcff, #f1f8ff);
}

@media (max-width: 1100px) {
  .func-panel {
    grid-template-columns: 1fr;
  }

  .func-output {
    border-top: 1px solid #dce9f3;
  }

  .result-box {
    min-height: 180px;
  }
}

@media (max-width: 900px) {
  .view-banner {
    flex-direction: column;
    gap: 10px;
  }

  .view-banner-title {
    font-size: 20px;
  }

  .view-banner-tags {
    width: 100%;
    flex-wrap: wrap;
  }

  .topbar {
    height: auto;
    padding: 10px 12px;
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .top-main {
    width: 100%;
  }

  .top-actions {
    width: 100%;
    flex-wrap: wrap;
    gap: 6px;
  }

  .cache-countdown {
    padding: 3px 8px;
  }

  .mobile-action-switch {
    flex-direction: column;
    align-items: stretch;
  }

  .upload-actions {
    flex-wrap: wrap;
  }

  .logs-table {
    min-width: 680px;
  }

  .error-reason-cell {
    width: 220px;
  }
}
</style>

