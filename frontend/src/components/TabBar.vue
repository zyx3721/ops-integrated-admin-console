<template>
  <div class="tab-bar">
    <div class="tab-bar-scroll" ref="scrollRef">
      <div class="tab-bar-inner">
        <div
          v-for="tab in tabsStore.tabs"
          :key="tab.path"
          class="tab-item"
          :class="{ active: tab.path === tabsStore.activeTab }"
          :style="tab.path === tabsStore.activeTab ? activeTabStyle : {}"
          @click="handleClick(tab)"
          @contextmenu.prevent="handleContextMenu($event, tab)"
        >
          <span class="tab-title">{{ tab.title }}</span>
          <n-icon
            v-if="!tab.affix"
            size="14"
            class="tab-close"
            @click.stop="handleClose(tab)"
          >
            <CloseOutline />
          </n-icon>
        </div>
      </div>
    </div>

    <!-- 右键菜单 -->
    <n-dropdown
      :show="contextMenuVisible"
      :options="contextMenuOptions"
      :x="contextMenuX"
      :y="contextMenuY"
      placement="bottom-start"
      @clickoutside="contextMenuVisible = false"
      @select="handleContextMenuSelect"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { CloseOutline } from '@vicons/ionicons5'
import { useTabsStore, type TabItem } from '@/stores/tabs'
import { useThemeStore } from '@/stores/theme'

const router = useRouter()
const route = useRoute()
const tabsStore = useTabsStore()
const themeStore = useThemeStore()
const scrollRef = ref<HTMLElement | null>(null)

// 激活页签的动态样式（跟随主题色）
const activeTabStyle = computed(() => {
  const color = themeStore.primaryColor
  return {
    color: color,
    borderColor: color,
    background: `${color}15`
  }
})

// 右键菜单
const contextMenuVisible = ref(false)
const contextMenuX = ref(0)
const contextMenuY = ref(0)
const contextMenuTab = ref<TabItem | null>(null)

const contextMenuOptions = computed(() => {
  const tab = contextMenuTab.value
  if (!tab) return []
  return [
    { label: '刷新当前', key: 'refresh' },
    { label: '关闭当前', key: 'close', disabled: tab.affix },
    { type: 'divider', key: 'd1' },
    { label: '关闭左侧', key: 'closeLeft' },
    { label: '关闭右侧', key: 'closeRight' },
    { label: '关闭其他', key: 'closeOther' },
    { label: '关闭所有', key: 'closeAll' }
  ]
})

// 监听路由变化，自动添加页签
watch(
  () => route.path,
  () => {
    tabsStore.addTab(route)
    nextTick(scrollToActiveTab)
  },
  { immediate: true }
)

function handleClick(tab: TabItem) {
  if (tab.path !== route.path) {
    router.push(tab.path)
  }
}

function handleClose(tab: TabItem) {
  const nextPath = tabsStore.closeTab(tab.path)
  if (nextPath) {
    router.push(nextPath)
  }
}

function handleContextMenu(e: MouseEvent, tab: TabItem) {
  contextMenuTab.value = tab
  contextMenuX.value = e.clientX
  contextMenuY.value = e.clientY
  contextMenuVisible.value = true
}

function handleContextMenuSelect(key: string) {
  contextMenuVisible.value = false
  const tab = contextMenuTab.value
  if (!tab) return

  switch (key) {
    case 'refresh':
      // 通过跳转空路由再跳回来实现刷新
      router.replace({ path: '/redirect' + tab.path })
      break
    case 'close': {
      const nextPath = tabsStore.closeTab(tab.path)
      if (nextPath) router.push(nextPath)
      break
    }
    case 'closeLeft':
      tabsStore.closeLeftTabs(tab.path)
      if (!tabsStore.tabs.find(t => t.path === route.path)) {
        router.push(tab.path)
      }
      break
    case 'closeRight':
      tabsStore.closeRightTabs(tab.path)
      if (!tabsStore.tabs.find(t => t.path === route.path)) {
        router.push(tab.path)
      }
      break
    case 'closeOther':
      tabsStore.closeOtherTabs(tab.path)
      if (route.path !== tab.path) {
        router.push(tab.path)
      }
      break
    case 'closeAll': {
      const homePath = tabsStore.closeAllTabs()
      router.push(homePath)
      break
    }
  }
}

function scrollToActiveTab() {
  if (!scrollRef.value) return
  const activeEl = scrollRef.value.querySelector('.tab-item.active') as HTMLElement
  if (activeEl) {
    activeEl.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' })
  }
}
</script>

<style lang="scss" scoped>
.tab-bar {
  display: flex;
  align-items: center;
  background: #fff;
  border-bottom: 1px solid #e8e8e8;
  padding: 0 12px;
  height: 38px;
  user-select: none;
}

body.dark-theme .tab-bar {
  background: #1e1e20;
  border-bottom-color: #3f3f46;
}

.tab-bar-scroll {
  flex: 1;
  overflow-x: auto;
  overflow-y: hidden;

  /* 隐藏滚动条 */
  &::-webkit-scrollbar {
    display: none;
  }
  scrollbar-width: none;
}

.tab-bar-inner {
  display: flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}

.tab-item {
  display: inline-flex;
  align-items: center;
  height: 28px;
  padding: 0 12px;
  border-radius: 4px;
  font-size: 13px;
  color: #666;
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
  gap: 4px;
  background: transparent;
  border: 1px solid transparent;

  &:hover {
    color: #333;
    background: #f0f0f0;
  }

  &.active {
    color: #18a058;
    background: #e8f5e9;
    border-color: #18a058;
  }
}

body.dark-theme .tab-item {
  color: #ffffffa6;

  &:hover {
    color: #ffffffd1;
    background: #2d2d30;
  }

  &.active {
    color: #63e2b7;
    background: rgba(99, 226, 183, 0.1);
    border-color: #63e2b7;
  }
}

.tab-title {
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tab-close {
  border-radius: 50%;
  padding: 1px;
  transition: all 0.2s;

  &:hover {
    background: rgba(0, 0, 0, 0.1);
  }
}

body.dark-theme .tab-close:hover {
  background: rgba(255, 255, 255, 0.1);
}
</style>
