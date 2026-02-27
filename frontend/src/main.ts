import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createDiscreteApi, zhCN, dateZhCN } from 'naive-ui'
import App from './App.vue'
import router from './router'
import './styles/index.scss'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)

const { message } = createDiscreteApi(['message'], {
  configProviderProps: {
    locale: zhCN,
    dateLocale: dateZhCN,
  },
})
app.config.globalProperties.$message = message

app.mount('#app')
