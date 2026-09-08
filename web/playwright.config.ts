import { defineConfig } from '@playwright/test';

/**
 * Smoke-gate Playwright config (Task SG1).
 *
 * This config exists for `make smoke` / `npm run e2e`: a headless-browser
 * smoke gate that drives the REAL built client served by a REAL `gorged`
 * and fails on any browser error. The two servers it drives are started by
 * `scripts/smoke.sh` and handed to the test through SMOKE_PUBLIC / SMOKE_OMNI
 * (base URLs). We deliberately do NOT use Playwright's `webServer` to start
 * gorged: the smoke gate needs its own teardown semantics (kill both servers
 * and remove their temp dirs even on failure), and the two servers must be
 * started/stopped together as a pair, not one-per-test.
 *
 * testDir is ./e2e so this can never pick up the Vitest unit tests in src/.
 */
export default defineConfig({
  testDir: './e2e',
  timeout: 120_000,
  // One worker: the gate drives two live servers and keeps the load on the
  // engine (and this shared box) to a single browser + two gorged at a time.
  workers: 1,
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: [['list']],
  use: {
    browserName: 'chromium',
    headless: true,
    viewport: { width: 1440, height: 900 },
    // The gorged origin is per-run and injected by scripts/smoke.sh; each
    // test computes its own URLs from SMOKE_PUBLIC / SMOKE_OMNI.
    trace: 'retain-on-failure',
  },
});
