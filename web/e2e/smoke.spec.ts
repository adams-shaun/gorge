import { test, expect, type Page, type APIRequestContext } from '@playwright/test';

/**
 * Task SG1 — a browser smoke gate.
 *
 * The unit suite (45 files, 439 tests), svelte-check over 382 files and
 * eslint were ALL GREEN while the whole public demo was dead: every table
 * page stuck on "Loading match N..." because `routes/Table.svelte` spread
 * `...p.hand`, which is a literal JSON `null` for a viewer without
 * omniscience. Every fixture in every test had an array hand, so nothing in
 * the suite saw the real-server shape.
 *
 * This spec is the gate that would have caught it. It drives the REAL built
 * client against a REAL `gorged`, in the two spectator modes the server
 * serves — `public` (the mode that was broken) and `omniscient` (the mode
 * that hid it) — and fails the build on any pageerror, any console.error,
 * any failed request to the app's own origin, or a page that silently never
 * leaves its loading state.
 *
 * The servers are started by scripts/smoke.sh and handed to us as base URLs
 * in SMOKE_PUBLIC / SMOKE_OMNI. Nothing here starts or stops a server; the
 * gate's teardown lives in the script so both servers die even on failure.
 */

const PUBLIC = process.env.SMOKE_PUBLIC;
const OMNI = process.env.SMOKE_OMNI;

// How long a page may sit in its loading state before we call it a hang.
// The live server paces decisions at 1.5s and pushes a snapshot on first
// subscribe, so a healthy page mounts in well under a second; 20s is a
// generous ceiling that still turns a never-mounting page into a failure.
const WAIT_MS = 20_000;

/** sameOrigin reports whether url is on the same origin as base — the only
 *  requests the gate forbids from failing (fonts and other third-party
 *  resources may legitimately fail off-network and must not poison the run). */
function sameOrigin(base: string, url: string): boolean {
  try {
    return new URL(url).origin === new URL(base).origin;
  } catch {
    return false;
  }
}

interface Issues {
  pageErrors: string[];
  consoleErrors: string[];
  failed: string[];
}

/** watch arms the three failure collectors on the page, scoped to the server's
 *  own origin, and returns the collector to assert against after the page has
 *  settled. */
function watch(page: Page, base: string): Issues {
  const c: Issues = { pageErrors: [], consoleErrors: [], failed: [] };
  page.on('pageerror', (e) => c.pageErrors.push(String(e)));
  page.on('console', (m) => {
    if (m.type() === 'error') c.consoleErrors.push(m.text());
  });
  page.on('requestfailed', (r) => {
    // ERR_ABORTED is the browser cancelling an in-flight request because
    // the page navigated away or closed — navigation noise, not a product
    // failure. A page that genuinely fails to reach the server reports a
    // connection/DNS/timeout error instead, and we still catch those.
    if (r.failure()?.errorText?.includes('ERR_ABORTED')) return;
    if (sameOrigin(base, r.url())) {
      c.failed.push(`requestfailed ${r.method()} ${r.url()} :: ${r.failure()?.errorText ?? ''}`);
    }
  });
  page.on('response', (r) => {
    if (sameOrigin(base, r.url()) && r.status() >= 400) {
      c.failed.push(`HTTP ${r.status()} ${r.request().method()} ${r.url()}`);
    }
  });
  return c;
}

function formatIssues(c: Issues): string[] {
  return [
    ...c.pageErrors.map((e) => `pageerror: ${e}`),
    ...c.consoleErrors.map((e) => `console.error: ${e}`),
    ...c.failed,
  ];
}

function expectClean(c: Issues, context: string): void {
  const bad = formatIssues(c);
  expect(bad, `${context} produced ${bad.length} browser failure(s)`).toEqual([]);
}

/** liveTableAndMatch reads the real ids from the server rather than
 *  hardcoding: the first table, and the latest match on it that has any
 *  events (the live one is the last entry of /api/tables/{t}/matches). */
async function liveTableAndMatch(request: APIRequestContext, base: string): Promise<{ table: string; match: number }> {
  const tResp = await request.get(`${base}/api/tables`);
  expect(tResp.ok(), `GET /api/tables on ${base} should succeed`).toBe(true);
  const tables = (await tResp.json()) as Array<{ id: string; state: string }>;
  expect(tables.length, `${base} should have at least one table`).toBeGreaterThan(0);
  const table = tables[0].id;

  const mResp = await request.get(`${base}/api/tables/${table}/matches`);
  expect(mResp.ok(), `GET /api/tables/${table}/matches on ${base} should succeed`).toBe(true);
  const matches = (await mResp.json()) as Array<{ match: number; events: number }>;
  const haveEvents = matches.filter((m) => m.events > 0);
  expect(haveEvents.length, `${base} table ${table} should have a match with events`).toBeGreaterThan(0);
  return { table, match: haveEvents[haveEvents.length - 1].match };
}

/** assertLobby checks the overview route mounts and stays clean. */
async function assertLobby(page: Page, base: string, label: string): Promise<void> {
  const c = watch(page, base);
  const res = await page.goto(`${base}/`, { waitUntil: 'domcontentloaded' });
  expect(res?.status() ?? 0, `${label} GET / should return 200`).toBe(200);
  await page.locator('main.overview').waitFor({ state: 'visible', timeout: WAIT_MS });
  await page.locator('main.overview h1').waitFor({ state: 'visible', timeout: WAIT_MS });
  // A beat to let the overview's own /api/tables fetch and any SSE window
  // settle before we judge its console.
  await page.waitForTimeout(1000);
  expectClean(c, `${label} lobby /`);
}

/** assertTable checks one table route (live or finished) mounts its board,
 *  renders at least one transcript line, and leaves no loading state. */
async function assertTable(page: Page, base: string, path: string, label: string): Promise<void> {
  const c = watch(page, base);
  const url = `${base}${path}`;
  const res = await page.goto(url, { waitUntil: 'domcontentloaded' });
  expect(res?.status() ?? 0, `${label} GET ${path} should return 200`).toBe(200);

  // The page must actually mount the board within the bound. A page that
  // silently never leaves "Loading match N..." / "Waiting for tX..." fails
  // here — this is exactly the bug the gate exists for.
  let mounted = true;
  try {
    await page.locator('.quadrant').first().waitFor({ state: 'visible', timeout: WAIT_MS });
  } catch {
    mounted = false;
  }
  if (!mounted) {
    // Report what the page was actually showing so the failure names the
    // loading/blank state rather than just "no board".
    let shown: string;
    try {
      shown = (await page.locator('p.waiting').first().textContent()) ?? '(no .waiting element — blank page)';
    } catch {
      shown = '(no .waiting element — blank page)';
    }
    expect(mounted, `${label} GET ${path} never mounted (showing: "${shown}")`).toBe(true);
  }

  // The page must render something real, not merely stay error-free: seat
  // boxes for every player, and at least one transcript line.
  const seats = await page.locator('.quadrant').count();
  expect(seats, `${label} GET ${path} should render seat boxes`).toBeGreaterThan(0);

  // The transcript's live lines: give the SSE/DVR a moment to land the first
  // event line for the live route.
  await page.waitForFunction(
    () => document.querySelectorAll('.transcript .line').length >= 1,
    undefined,
    { timeout: WAIT_MS },
  );
  expectClean(c, `${label} GET ${path}`);
}

for (const [mode, base] of [
  ['public', PUBLIC],
  ['omniscient', OMNI],
] as const) {
  const skip = base ? false : true;
  test.describe(`gorged [${mode}] spectator smoke`, () => {
    test.skip(skip, 'SMOKE_PUBLIC/SMOKE_OMNI unset — run via `make smoke` (scripts/smoke.sh)');

    test.describe.configure({ mode: 'serial' });

    test('lobby / and live table and specific match mount cleanly', async ({ browser, request }) => {
      const b = base as string;
      const label = `[${mode}]`;
      const ctx = await browser.newContext();
      try {
        // Each route gets its own page so a navigation from one route to
        // the next cannot abort the previous route's in-flight requests
        // (which would read as a spurious requestfailure).
        const lobby = await ctx.newPage();
        await assertLobby(lobby, b, label);
        await lobby.close();

        // Resolve the real ids from the server.
        const { table, match } = await liveTableAndMatch(request, b);

        const live = await ctx.newPage();
        await assertTable(live, b, `/t/${table}`, `${label} live table /t/${table}`);
        await live.close();

        const fin = await ctx.newPage();
        await assertTable(fin, b, `/t/${table}/m/${match}`, `${label} match /t/${table}/m/${match}`);
        await fin.close();
      } finally {
        await ctx.close();
      }
    });
  });
}
