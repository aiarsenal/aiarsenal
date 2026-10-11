import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

// Tauri 生产构建用 file:// 加载，必须相对 base；dev 用 http://localhost:5174
export default defineConfig({
  plugins: [vue()],
  base: './',
  clearScreen: false,
  server: {
    port: 5174,
    strictPort: true,
  },
  build: {
    target: 'es2021',
    outDir: 'dist',
    emptyOutDir: true,
  },
});
