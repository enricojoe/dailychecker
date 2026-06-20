import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    // Bind to all interfaces so the dev server is reachable over the LAN
    // (e.g. http://<your-ip>:5173 from a phone on the same network).
    host: true,
    // Same-origin API proxy: the browser calls /api on whatever host it loaded
    // the app from, and Vite forwards it to the backend. This avoids hardcoding
    // a LAN IP and sidesteps CORS entirely for LAN access.
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
