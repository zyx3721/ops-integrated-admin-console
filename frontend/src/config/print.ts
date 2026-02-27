export const PRINT_SECTION_VALUES = [
  '数据总部',
  '数金总部',
  '长亮合度',
  '长亮控股',
  '长亮金服',
  '长亮科技\\人力资源中心',
  '长亮科技\\运营中心',
  '长亮科技\\共享服务中心',
  '长亮科技\\信息服务中心',
  '长亮科技\\财务中心',
  '长亮科技\\集团解决方案部',
  '长亮科技\\集团项目管理部',
  '长亮科技\\内部审计部',
  '长亮科技\\市场部',
  '长亮科技\\干部部',
  '长亮科技\\战略规划部',
  '长亮科技\\研发中心',
  '长亮科技\\研发体系',
  '长亮科技\\税务部',
  '长亮科技\\销售总部',
  '长亮科技\\集团产品发展部',
  '长亮科技\\董事会办公室',
  '长亮科技\\公共关系部',
  '长亮科技\\健康督导办公室',
  '长亮科技\\集团总裁办',
  '长亮科技\\总裁办公室',
  '长亮科技\\北京运营中心',
  '外部人员',
]

export const PRINT_SEARCH_KEY_OPTIONS = [
  { label: '用户名', value: 'username' },
  { label: '姓名', value: 'fullname' },
  { label: '邮箱', value: 'email' },
]

export const PRINT_GENDER_OPTIONS_ADD = [
  { label: '男', value: 'male' },
  { label: '女', value: 'female' },
]

export const PRINT_GENDER_OPTIONS_MODIFY = [
  { label: '男', value: 'male' },
  { label: '女', value: 'female' },
  { label: '未知', value: 'unknown' },
]

export const PRINT_STATUS_OPTIONS = [
  { label: '正常', value: 'enabled' },
  { label: '禁用', value: 'disabled' },
]

export const PRINT_ROLE_OPTIONS = [
  { label: '彩色权限', value: '13a8c61c6888a4c' },
  { label: '报表', value: '36b238261872cd10208' },
  { label: '管理员', value: '7d9bfe7cd65a29' },
  { label: '黑白权限', value: '12483a1e79473e4' },
]

export const PRINT_ROLE_NAME_TO_ID: Record<string, string> = Object.fromEntries(
  PRINT_ROLE_OPTIONS.map((item) => [item.label, item.value]),
)
