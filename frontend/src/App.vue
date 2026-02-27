<template>
  <n-config-provider :locale="zhCN" :date-locale="dateZhCN" :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <router-view />
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import type { GlobalThemeOverrides } from 'naive-ui'
import { NConfigProvider, NMessageProvider, NDialogProvider, zhCN, dateZhCN } from 'naive-ui'

const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#1b6ea9',
    primaryColorHover: '#2578b2',
    primaryColorPressed: '#0f5f98',
    borderRadius: '10px',
    fontFamily: '"Noto Sans SC", "PingFang SC", "Microsoft YaHei", sans-serif',
  },
  Card: {
    borderRadius: '16px',
    titleFontWeight: '700',
  },
  Input: {
    borderRadius: '9px',
  },
  Select: {
    peers: {
      InternalSelection: {
        borderRadius: '9px',
      },
    },
  },
}
</script>
