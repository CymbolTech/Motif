import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { tanstackRouter } from '@tanstack/router-plugin/vite';

export default defineConfig({
  // The router plugin generates src/routeTree.gen.ts from src/routes and must run before React.
  plugins: [tanstackRouter({ target: 'react', routesDirectory: './src/routes', generatedRouteTree: './src/routeTree.gen.ts' }), react(), tailwindcss()],
  server: {
    port: 4173,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:18080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
      '/mcp': {
        target: 'http://127.0.0.1:18082',
        changeOrigin: true,
      },
      '/worker': {
        target: 'http://127.0.0.1:18081',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/worker/, ''),
      },
    },
  },
});
