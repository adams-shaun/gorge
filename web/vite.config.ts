/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  build: { outDir: '../cmd/gorged/webdist', emptyOutDir: true },
  // Worktrees share node_modules; callers may point Vite's mutable cache at
  // their own scratch directory rather than writing through that symlink.
  cacheDir: process.env.VITE_CACHE_DIR,
  server: { proxy: { '/api': { target: 'http://localhost:8080', changeOrigin: true } } },
  test: { environment: 'node', include: ['src/**/*.test.ts'] },
});
