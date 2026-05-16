import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const WEBUI_PORT = Number(process.env.WEBUI_PORT || 3001)

export default defineConfig({
  plugins: [react()],
  build: {
    sourcemap: false,
  },
  server: {
    port: WEBUI_PORT,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true
      }
    }
  }
})
