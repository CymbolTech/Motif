import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 4173,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:18080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
      '/worker': {
        target: 'http://127.0.0.1:18081',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/worker/, ''),
      },
    },
  },
});
