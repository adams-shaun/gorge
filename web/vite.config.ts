/// <reference types="vitest/config" />
import { resolve } from 'node:path';
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  build: { outDir: '../cmd/gorged/webdist', emptyOutDir: true },
  // Worktrees share node_modules; callers may point Vite's mutable cache at
  // their own scratch directory rather than writing through that symlink.
  cacheDir: process.env.VITE_CACHE_DIR,
  server: { proxy: { '/api': { target: 'http://localhost:8080', changeOrigin: true } } },
  test: {
    // Keep the persistent transform cache in the configured isolated Vite cache
    // (or this worktree's .vite directory), never in a shared mutable cache.
    fsModuleCache: true,
    fsModuleCachePath: resolve(process.env.VITE_CACHE_DIR ?? '.vite', 'vitest-fs-cache'),
    environment: 'node',
    include: ['src/**/*.test.ts'],
    globalSetup: './src/test/browser.global.ts',
  },
});
