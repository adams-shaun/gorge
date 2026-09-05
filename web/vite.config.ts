/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  build: { outDir: '../cmd/gorged/webdist', emptyOutDir: true },
  server: { proxy: { '/api': { target: 'http://localhost:8080', changeOrigin: true } } },
  test: { environment: 'node', include: ['src/**/*.test.ts'] },
});
