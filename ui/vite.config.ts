import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { tanstackRouter } from '@tanstack/router-plugin/vite'

export default defineConfig({
  plugins: [tanstackRouter({ quoteStyle: 'single' }), react()],
  build: { outDir: 'dist' },
  server: {
    port: 7610,
    proxy: {
      '/api': {
        target: 'http://localhost:7600',
        changeOrigin: true,
      },
    },
  },
})
