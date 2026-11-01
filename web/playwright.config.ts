import { defineConfig } from '@playwright/test';

/**
 * Smoke-gate Playwright config.
 *
 * This config exists for `make smoke` / `npm run e2e`: a headless-browser
 * smoke gate that drives the REAL built client served by a REAL `gorged`
 * and fails on any browser error. The servers it drives are started by
  * `scripts/smoke.sh` and handed to the test through the SMOKE_* variables
  * (base URLs). We deliberately do NOT use Playwright's `webServer` to start
  * gorged: the smoke gate needs its own teardown semantics (kill every server
  * and remove their temp dirs even on failure), and the servers must be
  * started/stopped together as a set, not one-per-test.
 *
 * testDir is ./e2e so this can never pick up the Vitest unit tests in src/.
 */
export default defineConfig({
  testDir: './e2e',
  timeout: 120_000,
  // One worker: the gate keeps the load on the engine (and this shared box) to
  // a single browser at a time.
  workers: 1,
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: [['list']],
  use: {
    browserName: 'chromium',
    headless: true,
    viewport: { width: 1440, height: 900 },
    // The gorged origins are per-run and injected by scripts/smoke.sh; each
    // test computes its own URLs from the SMOKE_* variables.
    trace: 'retain-on-failure',
  },
});
