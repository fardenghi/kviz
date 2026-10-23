/// <reference types="vitest" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// `npm run dev` levanta la interfaz con recarga en caliente y le pasa /api al backend,
// que tiene que estar corriendo aparte (`go run ./cmd/kviz -no-abrir`).
export default defineConfig({
  plugins: [react()],
  build: { outDir: 'dist', emptyOutDir: true },
  server: { port: 5173, proxy: { '/api': 'http://127.0.0.1:7777' } },
  test: { environment: 'node', include: ['src/**/*.test.ts'] },
});
