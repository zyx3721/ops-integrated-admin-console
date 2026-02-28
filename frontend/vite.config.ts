import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd())
  const apiTarget = env.VITE_API_BASE || 'http://localhost:8080'

  return {
    plugins: [vue()],
    base: '/',
    resolve: {
      alias: {
        '@': resolve(__dirname, 'src')
      }
    },
    build: {
      outDir: resolve(__dirname, 'dist'),
      emptyOutDir: true
    },
    server: {
      // 监听 0.0.0.0 以便在局域网 / 容器环境中通过 IP 访问开发服务器
      host: '0.0.0.0',
      port: 3000,
      proxy: {
        '/api': {
          target: apiTarget,
          changeOrigin: true,
          ws: true
        }
      }
    }
  }
})
