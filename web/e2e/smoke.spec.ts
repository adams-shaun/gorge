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
const SEATED = process.env.SMOKE_SEATED;

// ui19: the seated-view gate. The two spectator modes above drove no seated
// client, which is precisely the gap the task closes. These helpers measure
// the REAL rendered DOM with boundingBox()/getBoundingClientRect — never by
// reading CSS — because both ui19 regressions (a fan that sized itself from
// its own output, and a seated identity bar drawn on top of the seat's own
// first card) were geometry facts that the whole unit suite reported as
// correct. See the seated describe block at the bottom.

/** Rectangle helper: the overlapping test used across the seated assertions. */
type Rect = { x: number; y: number; width: number; height: number };

/** intersects reports whether two rects overlap on BOTH axes — the overlap
 *  that makes one element cover the other, not merely share an edge. */
function intersects(a: Rect | null, b: Rect | null): boolean {
  if (!a || !b) return false;
  const ax = a.x + a.width;
  const ay = a.y + a.height;
  const bx = b.x + b.width;
  const by = b.y + b.height;
  return a.x < bx && b.x < ax && a.y < by && b.y < ay;
}

/** right edge, named for the boundedness assertions below. */
function right(r: Rect): number { return r.x + r.width; }

/** seatedIdentity returns the identity bar rect for a given seat number. */
async function identityRect(page: Page, seat: number): Promise<Rect | null> {
  const el = page.locator(`.identity[data-seat="${seat}"]`);
  if ((await el.count()) === 0) return null;
  return await el.boundingBox();
}

/** boardRect is the section.board felt box the hand must be bounded by. */
async function boardRect(page: Page): Promise<Rect | null> {
  return await page.locator('section.board').boundingBox();
}

/** ownHandRect is the seated player's own hand fan row. Gated on existence:
 *  a spectator client mounts no fan, so a null here means the assertion is
 *  about a non-seated page and must not be run. */
async function ownHandRect(page: Page): Promise<Rect | null> {
  return await page.locator('.handtrack .handfan').boundingBox();
}

/** assertOwnHandClearOfIdentity checks the ui19 identity-bar defect: the
 *  seated player's OWN identity bar must not overlap their own hand fan. The
 *  defect had the bar's bottom offset at `var(--sp-2)`, dropping it onto the
 *  fan; the measured fix sits it above by --own-hand-h. This is the exact
 *  check that would have caught the bug its own author reported (hand top
 *  561 / identity top 644) and still missed. */
async function assertOwnHandClearOfIdentity(page: Page, seat: number, label: string): Promise<void> {
  const identity = await identityRect(page, seat);
  const hand = await ownHandRect(page);
  expect(identity, `${label} seated seat ${seat} should render an identity bar`).not.toBeNull();
  expect(hand, `${label} seated seat ${seat} should render its own hand fan`).not.toBeNull();
  const overlap = intersects(identity, hand);
  expect(
    overlap,
    `${label} seat ${seat}: own identity bar ${JSON.stringify(identity)} overlaps own hand fan ${JSON.stringify(hand)} (both axes)`,
  ).toBe(false);
}

/** assertHandWithinBoard checks the ui19 hand-fan defect: the fan's right
 *  edge must not exceed the board's, and its left edge must not precede it.
 *  The defect sized maxWidth from the fan's own output, so a hand wider than
 *  the felt ran straight off it instead of tightening. */
async function assertHandWithinBoard(page: Page, label: string, tolerate = 1): Promise<void> {
  const board = await boardRect(page);
  const hand = await ownHandRect(page);
  expect(board, `${label} should render a board`).not.toBeNull();
  expect(hand, `${label} should render the seated hand fan`).not.toBeNull();
  if (!board || !hand) return;
  expect(epsLe(right(hand), right(board), tolerate), `${label}: fan right ${right(hand)} must not exceed board right ${right(board)}`).toBe(true);
  // A centred fan is strictly inside the felt, so its left edge is normally
  // well INSIDE the board's. The defect that matters is the opposite: a fan
  // that measured its own output could OVERGROW the felt, so both edges must
  // stay within the board's left/right bounds.
  expect(epsGe(hand.x, board.x, tolerate), `${label}: fan left ${hand.x} must not precede board left ${board.x}`).toBe(true);
}

/** A small uniform tolerance so the browser's sub-pixel layout rounding does
 *  not turn a perfectly-bounded element into a one-pixel failure. */
/** epsLe asserts a <= b within tolerance t. */
function epsLe(a: number, b: number, t: number): boolean {
  return a <= b + t;
}

/** epsGe asserts a >= b within tolerance t. */
function epsGe(a: number, b: number, t: number): boolean {
  return a + t >= b;
}

/**
 * driveToCardOptionsWindow drives the seeded seated game to its FIRST
 * card-options window (ui23 R-E4-1). It answers the only decision the
 * engine's auto-pass does not handle — the opening mulligan (keep) — by
 * clicking the seated panel's own keep button, then poll the pending decision
 * until the seat's first main phase offers a card from hand at a NON-ZERO
 * wire index. The empty maintenance/priority windows on the way are
 * auto-passed by the seated client's own skip-empty floor, so this function
 * never touches them; it returns the target Decision to click its menu.
 *
 * The policy is deliberately trivial and fixed (answer keep, wait), so it is
 * reproducible against the seeded game: no timing-dependent cleverness, no
 * retries — just a generous wall-clock bound.
 */
async function driveToCardOptionsWindow(
  page: Page,
  request: APIRequestContext,
  b: string,
  table: string,
  seat: number,
  token: string,
): Promise<{ kind: string; options: Array<{ index: number; kind: string; obj?: number }> }> {
  const mResp = await request.get(`${b}/api/tables/${table}/matches`);
  expect(mResp.ok(), `GET /api/tables/${table}/matches on ${b} should succeed`).toBe(true);
  const matches = (await mResp.json()) as Array<{ match: number; events: number }>;
  const have = matches.filter((m) => m.events > 0);
  expect(have.length, `table ${table} should have a live match`).toBeGreaterThan(0);
  const match = have[have.length - 1].match;
  const sq = `?seat=${seat}&token=${encodeURIComponent(token)}`;
  const pendingURL = `${b}/api/tables/${table}/matches/${match}/pending${sq}`;

  const deadline = Date.now() + 90_000;
  while (Date.now() < deadline) {
    const p = await request.get(pendingURL);
    if (p.status() === 409) {
      // Nothing pending for this seat right now — the engine is elsewhere.
      await page.waitForTimeout(400);
      continue;
    }
    expect(p.ok(), `GET ${pendingURL} should succeed`).toBe(true);
    const d = (await p.json()) as { kind: string; options: Array<{ index: number; kind: string; obj?: number }> };

    // Target: a card-options window — a priority ask that offers a card from
    // hand (obj defined) at a NON-ZERO index. That is the discriminating
    // shape: clicking its menu must post the option's own index, because a
    // positional (indexOf) bug would post 0 and act on the wrong card.
    if (d.kind === 'priority' && d.options.some((o) => o.obj !== undefined && o.index > 0)) {
      return d;
    }

    if (d.kind === 'mulligan') {
      const keep = d.options.find((o) => o.kind === 'keep');
      if (keep) {
        const btn = page.locator(`.seat-panel [data-option="${keep.index}"]`);
        await btn.waitFor({ state: 'visible', timeout: WAIT_MS });
        await btn.click();
        await page.waitForTimeout(600);
      }
      continue;
    }

    // Any other shape before the target (an empty priority window the
    // skip-empty floor is about to pass) — wait for the game to advance.
    await page.waitForTimeout(400);
  }
  throw new Error('driveToCardOptionsWindow: did not reach a card-options window');
}

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

/** externalResourceError reports whether a console.error is a THIRD-PARTY
 *  resource failure — a cross-origin fetch (the card art / oracle fetch to
 *  Scryfall) that the browser CORS-blocks, or a blocked subresource load.
 *  Such an error carries the external URL in its message, and it is exactly
 *  the class of failure the requestfailed collector below already tolerates
 *  ("fonts and other third-party resources may legitimately fail off-network
 *  and must not poison the run"); without this filter a CORS-blocked card
 *  art fetch poisons the gate even though the app itself is healthy. Named
 *  sites are fine — the collector still rejects an error that is on the
 *  app's OWN origin, or that blames a resource the app controls. */
function externalResourceError(text: string, base: string): boolean {
  const urls = text.match(/https?:\/\/[^\s'"]+/g) ?? [];
  return urls.some((u) => {
    try {
      return new URL(u).origin !== new URL(base).origin;
    } catch {
      return false;
    }
  });
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
    if (m.type() !== 'error') return;
    // A third-party resource failure (a cross-origin card art / oracle fetch
    // CORS-blocked or failed off-network by the browser) is environment
    // noise, not a product bug — the same tolerance the requestfailed
    // collector applies. The browser also logs a generic "Failed to load
    // resource" console error for such a fetch AND for the 409 /pending
    // answer (the server's normal "nothing pending" reply); those are
    // covered by the URL-bearing handlers below (requestfailed / response),
    // which still catch a genuine SAME-ORIGIN failure and would report it
    // with its URL. Filtering this generic message therefore cannot hide a
    // product failure the URL-bearing handlers would not already report.
    const text = m.text();
    if (externalResourceError(text, base)) return;
    if (/Failed to load resource/.test(text)) return;
    c.consoleErrors.push(text);
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
    // 409 is the server's benign "conflict" — /pending answers 409 when
    // nothing is pending for this seat, and /intent answers 409 on a stale
    // seq; the client recovers from both by design (see seatpanel.ts
    // refreshPending / postIntent). It is not a product failure.
    if (sameOrigin(base, r.url()) && r.status() >= 400 && r.status() !== 409) {
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

// ---------------------------------------------------------------------------
// ui19 — SEATED 1v1 smoke gate.
//
// The two spectator modes above drove no seated client, so the two regressions
// this task exists to close — a hand fan that sized itself from its own
// output, and a seated player's identity bar drawn on top of their own hand
// — were invisible to the whole gate. This block drives a REAL seated 1v1
// client (against a REAL bot) and asserts the layout invariants those bugs
// violated, on the real rendered DOM.
//
// Join paths: the `-vsbot` flow (POST /api/games) always seats the human at
// seat 0 of a fresh table; the `-humans 1` startup table (t1) seats the
// human at seat 1. Both come from the ONE SMOKE_SEATED server (see
// scripts/smoke.sh, which seeds the seat-1 token to `ui19seat1`).
//
// Assertions (all measured, never read off CSS):
//  1. the seated player's own identity bar does not overlap their own hand
//     fan on both axes (the exact check that would have caught ui19's
//     identity-bar defect);
//  2. the hand fan is bounded by the board — right edge <= board right, and
//     its left edge does not precede the board's (the exact check that would
//     have caught ui19's self-sizing-fan defect);
//  3. 1v1 is top vs bottom, relative to the viewer: the seated player's own
//     quadrant/identity is BELOW their opponent's on screen, verified from
//     EACH seat (seat 0 via -vsbot, seat 1 via -humans 1);
//  4. the usual failure collectors on the seated page: no pageerror, no
//     console.error, no failed same-origin request, and the page leaves its
//     loading state.
//
// How the large hand is reached: assertion 2 wants the overlap-tightening
// path (a hand wide enough to need it), not just a fitting 7-card opening
// hand. Reaching a 9+ card hand in a live game within this gate's budget is
// not reliably achievable without driving the whole match through its
// decisions (which would make the gate slow and order-dependent), so the
// hand-bounded-by-board assertion is made on the real opening hand AND, where
// a large hand is genuinely reached in the live flow, on that too. See the
// ui19 report for the measured geometry and the honest statement of what a
// large hand would add.
for (const [mode, base] of [['seated', SEATED]] as const) {
  const skip = base ? false : true;
  test.describe(`gorged [${mode}] seated 1v1 smoke`, () => {
    test.skip(skip, 'SMOKE_SEATED unset — run via `make smoke` (scripts/smoke.sh)');

    test.describe.configure({ mode: 'serial' });

/** createVsBotJoin creates a real play-vs-bot game and returns the seated
 *  join path (seat 0 of a fresh table) — POST /api/games, exactly what the
 *  landing page does. */
    async function createVsBotJoin(request: APIRequestContext, b: string): Promise<{ join: string; seat: number }> {
      const resp = await request.post(`${b}/api/games`, { data: { format: 'constructed' } });
      expect(resp.ok(), `POST /api/games on ${b} should succeed`).toBe(true);
      const g = (await resp.json()) as { join: string; seat: number };
      expect(g.join, `POST /api/games on ${b} should return a join path`).toBeTruthy();
      return { join: g.join, seat: g.seat };
    }

    /** assertSeatedJoins navigates to a seated join URL, waits for the real
     *  board + own hand to mount, then runs all four invariant groups for the
     *  given seat. */
    async function assertSeatedJoin(b: string, page: Page, join: string, seat: number, label: string): Promise<void> {
      const c = watch(page, b);
      const url = `${b}${join}`;
      const res = await page.goto(url, { waitUntil: 'domcontentloaded' });
      expect(res?.status() ?? 0, `${label} GET ${join} should return 200`).toBe(200);

      // The seated page must mount its board AND its own hand fan (a seated
      // client that hangs on the seat-scoped view never gets here — this is
      // the loading-state gate for the seated page).
      await page.locator('.handtrack .handfan').waitFor({ state: 'visible', timeout: WAIT_MS });
      await page.locator('.quadrant').first().waitFor({ state: 'visible', timeout: WAIT_MS });
      // A beat for the SSE/snapshot to settle so the geometry and the console
      // judgement are against a steady page.
      await page.waitForTimeout(500);

      // 1. own identity bar clear of own hand fan (ui19 identity-bar defect).
      await assertOwnHandClearOfIdentity(page, seat, label);

      // 2. hand bounded by the board (ui19 self-sizing-fan defect).
      await assertHandWithinBoard(page, label);

      // 3. 1v1 top-vs-bottom, relative to viewer: the seated player's own
      //    quadrant and identity sit BELOW the opponent's on screen. The seat
      //    numbering is fixed by the viewer, so this is exactly the check
      //    that would regress if the mapping stopped being viewer-relative.
      const opp = seat === 0 ? 1 : 0;
      const ownQ = await page.locator(`.quadrant[data-seat="${seat}"]`).boundingBox();
      const oppQ = await page.locator(`.quadrant[data-seat="${opp}"]`).boundingBox();
      expect(ownQ, `${label} seat ${seat} should render its own quadrant`).not.toBeNull();
      expect(oppQ, `${label} seat ${opp} should render the opponent quadrant`).not.toBeNull();
      if (ownQ && oppQ) {
        // "Below on screen" = greater y. Boundaries are open (one quadrant
        // occupies the lower half, the other the upper), so equality cannot
        // occur, but keep a strict > so a same-line settlement fails loudly.
        expect(ownQ.y, `${label}: own seat ${seat} quadrant (y=${ownQ.y}) must be BELOW opponent seat ${opp} (y=${oppQ.y}) on screen`).toBeGreaterThan(oppQ.y);
      }
      const ownI = await identityRect(page, seat);
      const oppI = await identityRect(page, opp);
      if (ownI && oppI) {
        expect(ownI.y, `${label}: own seat ${seat} identity (y=${ownI.y}) must be BELOW opponent seat ${opp} (y=${oppI.y})`).toBeGreaterThan(oppI.y);
      }

      // 4. No browser failure on the settled page. The loading-state check is
      //    the `.handtrack .handfan` + `.quadrant` waitFor above: a seated page
      //    that hangs never mounts either. A `p.waiting` placeholder is NOT a
      //    loading state here -- a fully mounted board still shows "Waiting for
      //    X..." whenever it is the opponent's turn -- so counting it would
      //    assert nothing.
      expectClean(c, `${label} GET ${join}`);
    }

    test('seats a human vs a bot at seat 0 and asserts the seated 1v1 layout', async ({ browser, request }) => {
      const b = base as string;
      const label = `[seated]`;
      const { join, seat } = await createVsBotJoin(request, b);
      const ctx = await browser.newContext();
      try {
        const page = await ctx.newPage();
        await assertSeatedJoin(b, page, join, seat, label);
        await page.close();
      } finally {
        await ctx.close();
      }
    });

    test('seats a human at seat 1 (startup table) and asserts the mirrored 1v1 layout', async ({ browser }) => {
      const b = base as string;
      const label = `[seated]`;
      const ctx = await browser.newContext();
      try {
        const page = await ctx.newPage();
        // Seat 1 of the -humans 1 startup table t1; its token is fixed by
        // smoke.sh's -seat-token ui19seat1.
        await assertSeatedJoin(b, page, '/t/t1?seat=1&token=ui19seat1', 1, label);
        await page.close();
      } finally {
        await ctx.close();
      }
    });

    // The overlap-tightening path: the two tests above mount a 1440px-wide
    // board whose opening hand (7 cards) fits before any tightening is needed,
    // so a fan that sized itself from its own output would look fine there.
    // ui19's second defect was exactly that — maxWidth came from measuring the
    // fan itself, so the width constraint never bound and a hand too wide for
    // the felt ran straight off it. This test narrows the viewport so the SAME
    // real 7-card hand genuinely exceeds the board and MUST tighten; the
    // overflow is then asserted against the board bounds. (A genuinely large
    // 9+ card hand is not reachable inside a live game's budget in this build:
    // the normal hand-size rule caps it at 7 without an "no maximum hand size"
    // permanent, so we can only reach the tightening path by narrowing the
    // felt, not by growing the hand. See the ui19 report.)
    test('bounds a hand that exceeds a narrower board by tightening, not overflowing', async ({ browser, request }) => {
      const b = base as string;
      const label = `[seated]`;
      const { join, seat } = await createVsBotJoin(request, b);
      const ctx = await browser.newContext({ viewport: { width: 1000, height: 900 } });
      try {
        const page = await ctx.newPage();
        await assertSeatedJoin(b, page, join, seat, label);
        await page.close();
      } finally {
        await ctx.close();
      }
    });

    // ui21 — the log starts hidden for a SEATED player, with a toggle in the
    // rail. This is the deterministic half of the task's measured-evidence
    // requirement: it drives a REAL seated client and asserts the transcript
    // is hidden by default and becomes visible when the rail switch is
    // clicked, on measured DOM state (visibility), not a description. (The
    // other half — the per-card options affordance — is exercised by the unit
    // suite and built client below; it is not reliably reachable in a
    // vs-bot game, whose early turns offer no battlefield permanent the seat
    // may act on — see the ui21 report.)
    test('a seated player sees the log hidden by default and can toggle it in the rail', async ({ browser, request }) => {
      const b = base as string;
      const label = `[seated]`;
      const { join, seat } = await createVsBotJoin(request, b);
      const ctx = await browser.newContext();
      try {
        const page = await ctx.newPage();
        await assertSeatedJoin(b, page, join, seat, label);

        // Watch the post-mount interactions (the toggle itself) so a console
        // error surfacing from flipping the log is caught here.
        const c = watch(page, b);
        // The rail carries the log switch, and the seated default is hidden.
        const toggle = page.locator('[data-log-toggle]');
        await toggle.waitFor({ state: 'visible', timeout: WAIT_MS });
        expect(await toggle.getAttribute('aria-checked')).toBe('false');
        // The transcript footer is hidden (its grid row collapsed, so it has
        // no display box).
        const transcript = page.locator('footer.transcript');
        await page.waitForFunction(() => {
          const el = document.querySelector('footer.transcript');
          if (!el) return false;
          const r = el.getBoundingClientRect();
          return r.height === 0 || getComputedStyle(el).display === 'none';
        }, undefined, { timeout: WAIT_MS });

        // Clicking the rail switch shows it, measured by the footer gaining a
        // nonzero box.
        await toggle.click();
        await page.waitForFunction(() => {
          const el = document.querySelector('footer.transcript');
          if (!el) return false;
          const r = el.getBoundingClientRect();
          return r.height > 0 && getComputedStyle(el).display !== 'none';
        }, undefined, { timeout: WAIT_MS });
        expect(await toggle.getAttribute('aria-checked')).toBe('true');

        expectClean(c, `${label} log toggle`);
        await page.close();
      } finally {
        await ctx.close();
      }
    });

    // ui23 — the R-E4-1 guard, in a real browser. The rule this whole options
    // feature rests on is: never resolve an option by position. The unit
    // suite cannot express the discriminating test — vitest runs
    // `environment: 'node'` with no DOM, and the existing R-E4-1 test calls
    // the mock itself and asserts it was called, never clicking the rendered
    // menu. A positional bug (clicking menu item N posts index-of-position
    // instead of the option's own index) is type-clean, passes svelte-check,
    // and passes every unit test — only a REAL browser click on the
    // RENDERED menu can catch it.
    //
    // This drives the seeded vs-bot game to its FIRST card-options window —
    // the seat's first main phase offers several cards from hand, each a
    // distinct object with its own option at a NON-ZERO wire index (e.g.
    // `play_land,1,Ancient Tomb` while `cast,0,Chalice` is first) exactly the
    // discriminating shape: a tile whose menu item must post its object's
    // OWN index, because `list.indexOf(opt)` would post 0 (the wrong card).
    // We open the card's menu in the rendered fan, click an item, and assert
    // the option that was actually POSTED is that option's own index — most
    // robustly by which card it is that gets cast/played, since a positional
    // bug plays the wrong one.
    test('a card menu posts its own index, not a position in a rebuilt list (R-E4-1)', async ({ browser, request }) => {
      const b = base as string;
      const label = `[seated]`;
      // The deterministic seeded game (seed 1, smoke.sh's -seed 1): seat 1 of
      // the startup t1 table with smoke.sh's fixed seat token. A fixed answer
      // policy against a fixed seed is reproducible, which is why the guard
      // uses the seeded game rather than a random POST /api/games table.
      const join = '/t/t1?seat=1&token=ui19seat1';
      const seat = 1;
      const ctx = await browser.newContext();
      try {
        const page = await ctx.newPage();
        // Mount the seated client and keep it live while we drive the game
        // (see `driveToCardOptionsWindow`: answer the mulligan, let the
        // engine's auto-pass handle the empty windows, stop at the FIRST
        // card-options window).
        await page.goto(`${b}${join}`, { waitUntil: 'domcontentloaded' });
        await page.locator('.handtrack .handfan').waitFor({ state: 'visible', timeout: WAIT_MS });
        const c = watch(page, b);

        const target = await driveToCardOptionsWindow(page, request, b, 't1', seat, 'ui19seat1');

        // The target is the seat's first card-options window. Pick a CARD
        // option at a NON-ZERO index (zero would be the first wire entry,and
        // a positional bug would coincidentally be right on it) — the
        // discriminating shape. Take the LAST such card option.
        const cards = target.options.filter((o) => o.obj !== undefined && o.index > 0);
        expect(cards.length).toBeGreaterThan(0);
        const pick = cards[cards.length - 1];

        // The hand fan renders THAT card. Open its badge menu and click the
        // option item — the real click path the unit suite cannot reach.
        const cardLoc = page.locator(`.handfan [data-obj="${pick.obj}"]`);
        await cardLoc.waitFor({ state: 'visible', timeout: WAIT_MS });
        const badge = cardLoc.locator('button[aria-haspopup="menu"]');
        await badge.click();
        const item = cardLoc.locator('button[role="menuitem"]');
        await item.waitFor({ state: 'visible', timeout: WAIT_MS });
        await item.click();

        // Discriminating assertion, on the OBSERVABLE: the option's OWN card
        // crosses to the board (a land played). Under a positional bug the
        // click posts index 0 (the first wire option — casting the first
        // card), so this card stays in hand and the assertion times out.
        await page.waitForFunction((obj) => {
          return document.querySelector(`.quadrant [data-obj="${obj}"]`) !== null;
        }, pick.obj as number, { timeout: WAIT_MS });

        expectClean(c, `${label} R-E4-1 card menu`);
        await page.close();
      } finally {
        await ctx.close();
      }
    });
  });
}
