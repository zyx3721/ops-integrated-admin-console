<template>
  <div class="page-container">
    <n-grid :cols="2" :x-gap="16" :y-gap="16">
      <n-gi>
        <n-card title="基本信息">
          <n-descriptions :column="1" label-placement="left">
            <n-descriptions-item label="Redis版本">{{ cacheInfo.stats?.redis_version }}</n-descriptions-item>
            <n-descriptions-item label="运行天数">{{ cacheInfo.stats?.uptime_in_days }} 天</n-descriptions-item>
            <n-descriptions-item label="连接客户端数">{{ cacheInfo.stats?.connected_clients }}</n-descriptions-item>
            <n-descriptions-item label="已处理命令数">{{ cacheInfo.stats?.total_commands_processed }}</n-descriptions-item>
            <n-descriptions-item label="每秒处理命令">{{ cacheInfo.stats?.instantaneous_ops_per_sec }}</n-descriptions-item>
          </n-descriptions>
        </n-card>
      </n-gi>
      <n-gi>
        <n-card title="内存信息">
          <n-descriptions :column="1" label-placement="left">
            <n-descriptions-item label="已用内存">{{ cacheInfo.memory?.used_memory_human }}</n-descriptions-item>
            <n-descriptions-item label="内存峰值">{{ cacheInfo.memory?.used_memory_peak_human }}</n-descriptions-item>
            <n-descriptions-item label="Key数量">{{ cacheInfo.dbSize }}</n-descriptions-item>
          </n-descriptions>
        </n-card>
      </n-gi>
    </n-grid>

    <n-card title="缓存键列表" style="margin-top: 16px">
      <div class="search-form">
        <n-form inline label-placement="left">
          <n-form-item label="键名模式">
            <n-input v-model:value="searchPattern" placeholder="请输入键名模式" clearable style="width: 300px" />
          </n-form-item>
          <n-form-item>
            <n-button type="primary" @click="loadKeys">
              <template #icon><n-icon><SearchOutline /></n-icon></template>
              搜索
            </n-button>
          </n-form-item>
        </n-form>
      </div>
      
      <n-data-table :columns="columns" :data="keys" :loading="keysLoading" :row-key="(row: string) => row" />
    </n-card>

    <n-modal v-model:show="detailVisible" preset="card" title="缓存详情" style="width: 800px">
      <n-descriptions :column="1" label-placement="left" bordered>
        <n-descriptions-item label="键名">
          {{ cacheDetail.key }}
        </n-descriptions-item>
        <n-descriptions-item label="类型">
          <n-tag type="info">{{ cacheDetail.type }}</n-tag>
        </n-descriptions-item>
        <n-descriptions-item label="有效期">
          {{ formatTTL(cacheDetail.ttl) }}
        </n-descriptions-item>
        <n-descriptions-item label="值">
          <n-scrollbar style="max-height: 400px">
            <n-code :code="formatValue(cacheDetail.value)" language="json" word-wrap />
          </n-scrollbar>
        </n-descriptions-item>
      </n-descriptions>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, h, onMounted, computed } from 'vue'
import { NButton, NSpace, NTag, NInput, NForm, NFormItem, NIcon, NCard, NGrid, NGi, NDescriptions, NDescriptionsItem, NDataTable, NPagination, NModal, NScrollbar, NCode, useMessage, useDialog, type DataTableColumns } from 'naive-ui'
import { SearchOutline, EyeOutline, TrashOutline } from '@vicons/ionicons5'
import { cacheApi } from '@/api/monitor'

const message = useMessage()
const dialog = useDialog()

const cacheInfo = ref<any>({})
const keys = ref<string[]>([])
const keysLoading = ref(false)
const searchPattern = ref('*')

// 分页相关
const pagination = reactive({
  page: 1,
  pageSize: 10,
  itemCount: 0
})

// 计算当前页展示的数据
const pagedKeys = computed(() => {
  const start = (pagination.page - 1) * pagination.pageSize
  const end = start + pagination.pageSize
  return keys.value.slice(start, end)
})

// 详情相关
const detailVisible = ref(false)
const detailLoading = ref(false)
const cacheDetail = ref<{
  key: string,
  type: string,
  value: string,
  ttl: number
}>({
  key: '',
  type: '',
  value: '',
  ttl: -1
})

const columns: DataTableColumns<string> = [
  { title: '键名', key: 'key', render(row) { return h('span', { style: 'word-break: break-all;' }, row) }},
  { title: '操作', key: 'actions', width: 180, render(row) {
    return h(NSpace, null, {
      default: () => [
        h(NButton, { size: 'small', quaternary: true, type: 'primary', onClick: () => handleView(row) }, {
          default: () => [h(NIcon, null, { default: () => h(EyeOutline) }), ' 查看']
        }),
        h(NButton, { size: 'small', quaternary: true, type: 'error', onClick: () => handleDelete(row) }, {
          default: () => [h(NIcon, null, { default: () => h(TrashOutline) }), ' 删除']
        })
      ]
    })
  }}
]

async function loadCacheInfo() {
  try { cacheInfo.value = await cacheApi.info() }
  catch { /* ignore */ }
}

async function loadKeys() {
  keysLoading.value = true
  try { 
    const res = await cacheApi.keys(searchPattern.value) || []
    keys.value = res
    pagination.itemCount = res.length
    pagination.page = 1
  }
  finally { keysLoading.value = false }
}

async function handleView(key: string) {
  detailVisible.value = true
  detailLoading.value = true
  try {
    const res = await cacheApi.getValue(key)
    cacheDetail.value = {
      key: key,
      type: res.type || 'unknown',
      value: res.value,
      ttl: res.ttl ?? -1
    }
  } catch (error) {
    message.error('获取详情失败')
    detailVisible.value = false
  } finally {
    detailLoading.value = false
  }
}

function handleDelete(key: string) {
  dialog.warning({
    title: '提示', content: `确定要删除缓存"${key}"吗？`, positiveText: '确定', negativeText: '取消',
    onPositiveClick: async () => { await cacheApi.delete(key); message.success('删除成功'); loadKeys() }
  })
}

// 格式化展示 TTL
function formatTTL(ttl: number) {
  if (ttl === -1) return '永久有效'
  if (ttl === -2) return '已过期'
  return `${ttl} 秒`
}

// 格式化 Value
function formatValue(value: any) {
  if (!value) return ''
  if (typeof value === 'string') {
    try {
      const obj = JSON.parse(value)
      return JSON.stringify(obj, null, 2)
    } catch {
      return value
    }
  }
  return JSON.stringify(value, null, 2)
}

onMounted(() => { loadCacheInfo(); loadKeys() })
</script>
