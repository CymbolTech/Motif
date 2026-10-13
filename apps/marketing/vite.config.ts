import { defineConfig } from 'vite';
import { tanstackStart } from '@tanstack/react-start/plugin/vite';
import react from '@vitejs/plugin-react';
import { nitro } from 'nitro/vite';

export default defineConfig({
  server: { port: 4174, strictPort: true },
  plugins: [tanstackStart({ srcDirectory: 'src' }), react(), nitro()],
});
