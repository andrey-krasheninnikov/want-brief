import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const port = Number(process.env.WEB_PORT || 5173)
const api = process.env.API_PROXY || 'http://127.0.0.1:8080'

export default defineConfig({
  plugins: [react()],
  server: {
    host: '127.0.0.1',
    port,
    strictPort: true,
    proxy: {
      '/api': {
        target: api,
        changeOrigin: true,
      },
    },
  },
})
