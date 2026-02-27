<template>
  <div class="page-container">
    <n-card class="page-layout">
      <div class="search-form">
        <n-form inline :model="searchForm" label-placement="left">
          <n-form-item label="任务名称">
            <n-input v-model:value="searchForm.jobName" placeholder="请输入任务名称" clearable />
          </n-form-item>
          <n-form-item label="任务组名">
            <n-input v-model:value="searchForm.jobGroup" placeholder="请输入任务组名" clearable />
          </n-form-item>
          <n-form-item label="状态">
            <n-select v-model:value="searchForm.status" placeholder="请选择状态" :options="statusOptions" clearable style="width: 120px" />
          </n-form-item>
          <n-form-item>
            <n-space>
              <n-button type="primary" @click="handleSearch">
                <template #icon><n-icon><SearchOutline /></n-icon></template>
                搜索
              </n-button>
              <n-button @click="handleReset">
                <template #icon><n-icon><RefreshOutline /></n-icon></template>
                重置
              </n-button>
            </n-space>
          </n-form-item>
        </n-form>
      </div>

      <div class="table-toolbar">
        <n-space>
          <n-button v-if="hasPermission('monitor:job:add')" type="primary" @click="handleAdd">
            <template #icon><n-icon><AddOutline /></n-icon></template>
            新增任务
          </n-button>
          <n-button @click="showLogModal = true">
            <template #icon><n-icon><ListOutline /></n-icon></template>
            调度日志
          </n-button>
        </n-space>
      </div>

      <n-data-table
        :columns="columns"
        :data="tableData"
        :loading="loading"
        :row-key="(row: SysJob) => row.id"
        remote
      />

      <div class="pagination-container" style="display: flex; justify-content: flex-end; margin-top: 12px">
        <n-pagination
          v-model:page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :item-count="pagination.itemCount"
          :page-sizes="[10, 20, 50, 100]"
          show-size-picker
          show-quick-jumper
          @update:page="handlePageChange"
          @update:page-size="handlePageSizeChange"
        >
          <template #prefix>
            共 {{ pagination.itemCount }} 条
          </template>
        </n-pagination>
      </div>
    </n-card>

    <n-modal v-model:show="modalVisible" :title="modalTitle" preset="card" style="width: 600px" :mask-closable="false">
      <n-form ref="formRef" :model="formData" :rules="rules" label-placement="left" label-width="100">
        <n-form-item label="任务名称" path="jobName">
          <n-input v-model:value="formData.jobName" placeholder="请输入任务名称" />
        </n-form-item>
        <n-form-item label="任务组名" path="jobGroup">
          <n-input v-model:value="formData.jobGroup" placeholder="请输入任务组名" />
        </n-form-item>
        <n-form-item label="调用目标" path="invokeTarget">
          <n-input v-model:value="formData.invokeTarget" placeholder="请输入调用目标字符串 如: sampleTask.noParams" />
        </n-form-item>
        <n-form-item label="cron表达式" path="cronExpression">
          <n-input v-model:value="formData.cronExpression" placeholder="请输入cron表达式 如: 0/10 * * * * ?" />
        </n-form-item>
        <n-form-item label="错误策略" path="misfirePolicy">
          <n-select v-model:value="formData.misfirePolicy" :options="misfireOptions" />
        </n-form-item>
        <n-form-item label="并发执行" path="concurrent">
          <n-radio-group v-model:value="formData.concurrent">
            <n-radio :value="0">允许</n-radio>
            <n-radio :value="1">禁止</n-radio>
          </n-radio-group>
        </n-form-item>
        <n-form-item label="状态" path="status">
          <n-switch v-model:value="formData.status" :checked-value="1" :unchecked-value="0">
            <template #checked>正常</template>
            <template #unchecked>暂停</template>
          </n-switch>
        </n-form-item>
        <n-form-item label="备注" path="remark">
          <n-input v-model:value="formData.remark" type="textarea" placeholder="请输入备注" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="modalVisible = false">取消</n-button>
          <n-button type="primary" :loading="submitLoading" @click="handleSubmit">确定</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal v-model:show="showLogModal" title="调度日志" preset="card" style="width: 1000px">
      <n-data-table
        :columns="logColumns"
        :data="logData"
        :loading="logLoading"
        :row-key="(row: SysJobLog) => row.id"
        size="small"
        remote
      />
      <div class="pagination-container" style="display: flex; justify-content: flex-end; margin-top: 12px">
        <n-pagination
          v-model:page="logPagination.page"
          v-model:page-size="logPagination.pageSize"
          :item-count="logPagination.itemCount"
          :page-sizes="[10, 20, 50, 100]"
          show-size-picker
          show-quick-jumper
          @update:page="handleLogPageChange"
          @update:page-size="handleLogPageSizeChange"
        >
          <template #prefix>
            共 {{ logPagination.itemCount }} 条
          </template>
        </n-pagination>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, h, onMounted, watch, computed } from 'vue'
import { NButton, NTag, NSpace, NSwitch, NPagination, useMessage, useDialog, type DataTableColumns, type FormInst, type FormRules } from 'naive-ui'
import { SearchOutline, RefreshOutline, AddOutline, ListOutline } from '@vicons/ionicons5'
import { jobApi, type SysJob, type SysJobLog } from '@/api/monitor'
import { useUserStore } from '@/stores/user'

const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()
const hasPermission = (permission: string) => userStore.hasPermission(permission)

const searchForm = reactive({ jobName: '', jobGroup: '', status: null as number | null })
const statusOptions = [{ label: '正常', value: 1 }, { label: '暂停', value: 0 }]
const misfireOptions = [{ label: '立即执行', value: 1 }, { label: '执行一次', value: 2 }, { label: '放弃执行', value: 3 }]
const tableData = ref<SysJob[]>([])
const loading = ref(false)
const pagination = reactive({ page: 1, pageSize: 10, itemCount: 0, showSizePicker: true, pageSizes: [10, 20, 50] })

const columns: DataTableColumns<SysJob> = [
  { title: 'ID', key: 'id', width: 80 },
  { title: '任务名称', key: 'jobName', width: 150 },
  { title: '任务组名', key: 'jobGroup', width: 100 },
  { title: '调用目标', key: 'invokeTarget', ellipsis: { tooltip: true } },
  { title: 'cron表达式', key: 'cronExpression', width: 150 },
  { title: '状态', key: 'status', width: 100, render(row) {
    return h(NSwitch, { value: row.status === 1, size: 'small', onChange: (val: boolean) => handleChangeStatus(row, val ? 1 : 0) },
      { checked: () => '正常', unchecked: () => '暂停' })
  }},
  { title: '操作', key: 'actions', width: 200, fixed: 'right', render(row) {
    const buttons = []
    if (hasPermission('monitor:job:edit')) {
      buttons.push(h(NButton, { size: 'small', onClick: () => handleRun(row) }, { default: () => '执行' }))
      buttons.push(h(NButton, { size: 'small', onClick: () => handleEdit(row) }, { default: () => '编辑' }))
    }
    if (hasPermission('monitor:job:delete')) {
      buttons.push(h(NButton, { size: 'small', type: 'error', onClick: () => handleDelete(row) }, { default: () => '删除' }))
    }
    return buttons.length > 0 ? h(NSpace, null, { default: () => buttons }) : '-'
  }}
]

const modalVisible = ref(false)
const modalTitle = ref('新增任务')
const formRef = ref<FormInst | null>(null)
const submitLoading = ref(false)
const formData = reactive<SysJob>({ id: undefined, jobName: '', jobGroup: 'DEFAULT', invokeTarget: '', cronExpression: '', misfirePolicy: 3, concurrent: 1, status: 0, remark: '' })
const rules: FormRules = {
  jobName: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
  invokeTarget: [{ required: true, message: '请输入调用目标', trigger: 'blur' }],
  cronExpression: [{ required: true, message: '请输入cron表达式', trigger: 'blur' }]
}

// 日志
const showLogModal = ref(false)
const logData = ref<SysJobLog[]>([])
const logLoading = ref(false)
const logPagination = reactive({ page: 1, pageSize: 10, itemCount: 0, showSizePicker: true, pageSizes: [10, 20, 50] })
const logColumns: DataTableColumns<SysJobLog> = [
  { title: 'ID', key: 'id', width: 80 },
  { title: '任务名称', key: 'jobName', width: 120 },
  { title: '调用目标', key: 'invokeTarget', ellipsis: { tooltip: true } },
  { title: '日志信息', key: 'jobMessage', width: 100 },
  { title: '状态', key: 'status', width: 80, render(row) {
    return h(NTag, { type: row.status === 0 ? 'success' : 'error', size: 'small' }, { default: () => row.status === 0 ? '成功' : '失败' })
  }},
  { title: '开始时间', key: 'startTime', width: 180 },
  { title: '结束时间', key: 'stopTime', width: 180 }
]

watch(showLogModal, (val) => { if (val) loadLogData() })

async function loadData() {
  loading.value = true
  try {
    const res = await jobApi.page({ page: pagination.page, pageSize: pagination.pageSize, ...searchForm })
    tableData.value = res.list
    pagination.itemCount = res.total
  } finally { loading.value = false }
}

async function loadLogData() {
  logLoading.value = true
  try {
    const res = await jobApi.logPage({ page: logPagination.page, pageSize: logPagination.pageSize })
    logData.value = res.list
    logPagination.itemCount = res.total
  } finally { logLoading.value = false }
}

function handleSearch() { pagination.page = 1; loadData() }
function handleReset() { searchForm.jobName = ''; searchForm.jobGroup = ''; searchForm.status = null; handleSearch() }
function handlePageChange(page: number) { pagination.page = page; loadData() }
function handlePageSizeChange(pageSize: number) { pagination.pageSize = pageSize; pagination.page = 1; loadData() }
function handleLogPageChange(page: number) { logPagination.page = page; loadLogData() }
function handleLogPageSizeChange(pageSize: number) {
  logPagination.pageSize = pageSize
  logPagination.page = 1
  loadLogData()
}

function handleAdd() {
  modalTitle.value = '新增任务'
  Object.assign(formData, { id: undefined, jobName: '', jobGroup: 'DEFAULT', invokeTarget: '', cronExpression: '', misfirePolicy: 3, concurrent: 1, status: 0, remark: '' })
  modalVisible.value = true
}

function handleEdit(row: SysJob) {
  modalTitle.value = '编辑任务'
  Object.assign(formData, row)
  modalVisible.value = true
}

async function handleSubmit() {
  try {
    await formRef.value?.validate()
    submitLoading.value = true
    if (formData.id) { await jobApi.update(formData); message.success('更新成功') }
    else { await jobApi.create(formData); message.success('创建成功') }
    modalVisible.value = false
    loadData()
  } finally { submitLoading.value = false }
}

function handleDelete(row: SysJob) {
  dialog.warning({
    title: '提示', content: `确定要删除任务"${row.jobName}"吗？`, positiveText: '确定', negativeText: '取消',
    onPositiveClick: async () => { await jobApi.delete(row.id!); message.success('删除成功'); loadData() }
  })
}

async function handleChangeStatus(row: SysJob, status: number) {
  await jobApi.changeStatus(row.id!, status)
  message.success('操作成功')
  loadData()
}

async function handleRun(row: SysJob) {
  await jobApi.run(row.id!)
  message.success('执行成功')
}

onMounted(() => loadData())
</script>
<style lang="scss" scoped>
.page-layout{
  height: calc(100vh - 160px);
}
</style>
