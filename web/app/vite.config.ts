import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// Ports are injected by `make start/dev` from configs/salvo.yaml so that
// backend and frontend stay in sync. Defaults allow running vite standalone.
const BACKEND_PORT = process.env.SALVO_BACKEND_PORT || '8766'
const FRONTEND_PORT = parseInt(process.env.SALVO_FRONTEND_PORT || '3000', 10)
const FRONTEND_HOST = process.env.SALVO_FRONTEND_HOST || 'localhost'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    host: FRONTEND_HOST,
    port: FRONTEND_PORT,
    proxy: {
      '/api': {
        target: `http://localhost:${BACKEND_PORT}`,
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: '../../web/dist',
    emptyOutDir: true,
  },
})
