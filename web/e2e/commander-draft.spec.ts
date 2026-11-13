import { test, expect, type Page } from '@playwright/test';

/**
 * The Commander-draft browser gate: one human and three bots draft a
 * Commander-capable set (EventSetInfo.commander, e.g. MBC), build around a
 * drafted commander and sit down at the pod's four-player free-for-all.
 *
 * It drives the REAL built client against a REAL gorged started with
 * `-events` on a corpus that offers a Commander-draft set. The server's base
 * URL arrives in SMOKE_EVENTS; without it (or without a Commander set on
 * that server) the spec skips, exactly as smoke.spec.ts does for its own
 * SMOKE_* variables. Nothing here starts or stops a server.
 *
 * Nothing about the format is hard-coded: the number of picks, the picks
 * per pass, the deck size and the starting life are whatever the server
 * says. The one fixed fact asserted is the Commander one the mode exists
 * for: four players at the table, each at the format's starting life of 40.
 *
 * Bot assist (off by default) is turned on at the first pack: the seat's own
 * drafter must mark at least one card with a numbered suggestion badge, and
 * the build screen then shows the bot's suggested commanders.
 */

const BASE = process.env.SMOKE_EVENTS;
const WAIT_MS = 30_000;
const DRAFT_MS = 420_000;
const STARTING_LIFE = 40;

/** watchPage collects page errors and same-origin server errors (5xx); a 409 stale pick is the client's designed recovery. */
function watchPage(page: Page, base: string): string[] {
  const bad: string[] = [];
  const origin = new URL(base).origin;
  page.on('pageerror', (e) => bad.push(`pageerror: ${String(e)}`));
  page.on('response', (r) => {
    if (r.status() >= 500 && new URL(r.url()).origin === origin && !/^\/(art\/|cards\/named$)/.test(new URL(r.url()).pathname)) {
      bad.push(`HTTP ${r.status()} ${r.request().method()} ${r.url()}`);
    }
  });
  return bad;
}

test.describe('MBC Commander Draft', () => {
  test.skip(!BASE, 'SMOKE_EVENTS unset: run against a gorged started with -events and a Commander-draft set');
  test.describe.configure({ mode: 'serial' });

  test('drafts, builds around a commander and seats a four-player pod at 40 life', async ({ page, request }) => {
    test.setTimeout(DRAFT_MS + 240_000);
    const base = BASE!;
    const sets = (await (await request.get(`${base}/api/events/sets`)).json()) as Array<{ code: string; commander?: boolean }>;
    test.skip(!sets.some((s) => s.commander), `${base} offers no Commander-draft set`);
    const bad = watchPage(page, base);

    // The lobby offers the mode only because a set has the capability.
    await page.goto(`${base}/`, { waitUntil: 'domcontentloaded' });
    await page.getByTestId('commander-draft-button').click();
    await page.getByTestId('new-event').waitFor({ state: 'visible', timeout: WAIT_MS });
    const mode = page.getByTestId('format-commander');
    await expect(mode).toBeVisible({ timeout: WAIT_MS });
    await mode.click();
    await expect(mode).toHaveAttribute('aria-pressed', 'true');
    // The default pod: seat 1 is you, seats 2-4 bots.
    await expect(page.getByTestId('seat-1-human')).toHaveAttribute('aria-pressed', 'true');
    for (const n of [2, 3, 4]) await expect(page.getByTestId(`seat-${n}-bot`)).toHaveAttribute('aria-pressed', 'true');
    await page.getByTestId('open-pod').click();

    // One human: the create screen sends us to our own invite link.
    await page.waitForURL(/\/e\/[^/?]+\?/, { timeout: WAIT_MS });
    await page.getByTestId('event-page').waitFor({ state: 'visible', timeout: WAIT_MS });
    await expect(page.getByTestId('lobby-summary')).toContainText('Commander draft');
    await page.getByLabel('Your name').fill('Commander tester');
    await page.getByRole('button', { name: 'Join the pod' }).click();

    // The draft: take the first card of every pack until the build screen.
    const deckZone = page.getByTestId('zone-deck');
    const firstCard = page.locator('[data-testid="pack"] button.pcard').first();
    const deadline = Date.now() + DRAFT_MS;
    let picks = 0;
    let sawSecondPick = false;
    let assisted = false;
    while (!(await deckZone.isVisible())) {
      expect(Date.now(), `the draft should reach the build screen within ${DRAFT_MS}ms (${picks} picks taken)`).toBeLessThan(deadline);
      if (!(await firstCard.isVisible()) || !(await firstCard.isEnabled())) {
        await page.waitForTimeout(250);
        continue;
      }
      const id = await firstCard.getAttribute('data-id');
      if (!id) continue;
      if (!assisted) {
        // Off by default; on, the drafter's picks for this pack wear a badge.
        const toggle = page.getByTestId('assist-toggle');
        await expect(toggle).toHaveAttribute('aria-checked', 'false');
        await toggle.click();
        await expect(toggle).toHaveAttribute('aria-checked', 'true');
        await expect(page.locator('[data-testid="pack"] [data-testid="assist-badge"]').first()).toBeVisible({ timeout: WAIT_MS });
        await expect(page.locator('[data-testid="pack"] [data-testid="assist-badge"][data-rank="1"]')).toHaveCount(1);
        assisted = true;
      }
      const pass = page.getByTestId('pass-pick');
      if ((await pass.count()) > 0 && Number((await pass.getAttribute('data-pass-pick')) ?? '1') > 1) {
        sawSecondPick = true;
        // The second pick of a pass is signposted, naming the card already taken.
        await expect(pass).toContainText('You already took');
      }
      await firstCard.click();
      await page.getByTestId('take').click();
      picks++;
      // The pick lands when that card leaves the pack (or the pack goes away).
      await expect(page.locator(`[data-testid="pack"] [data-id="${id}"]`)).toHaveCount(0, { timeout: WAIT_MS });
    }
    expect(picks, 'the human took at least one pick').toBeGreaterThan(0);
    expect(sawSecondPick, 'a Commander draft takes more than one card per pass, and the screen says so').toBe(true);

    // Build: start from the bot's suggested deck, which names its commanders.
    await page.getByTestId('commander-slots').waitFor({ state: 'visible', timeout: WAIT_MS });
    // The switch is remembered: the build screen shows the bot's suggested commanders.
    await expect(page.getByTestId('assist-commanders')).toContainText('Commander', { timeout: WAIT_MS });
    await page.getByTestId('load-suggestion').click();
    const ready = page.getByTestId('ready');
    try {
      await expect(ready).toBeEnabled({ timeout: 10_000 });
    } catch {
      // No legal suggestion: lead with the pool's first commander and let the server fill the lands.
      const slot = page.getByTestId('commander-1');
      const firstOption = await slot.locator('option').nth(1).getAttribute('value');
      expect(firstOption, 'the pool offers a commander').toBeTruthy();
      await slot.selectOption(firstOption!);
      await page.getByTestId('auto-lands').click();
      await expect(ready).toBeEnabled({ timeout: WAIT_MS });
    }
    await expect(page.getByTestId('deck-count')).toHaveClass(/ok/);
    await ready.click();

    // The hub: the pod game row, then its join link to the table.
    const podRow = page.getByTestId('pod-game').first();
    await podRow.waitFor({ state: 'visible', timeout: 120_000 });
    const join = page.getByTestId('join-game').first();
    await join.waitFor({ state: 'visible', timeout: 120_000 });
    const href = await join.getAttribute('href');
    expect(href, 'the pod row carries the seat-tokenised table link').toMatch(/\/t\/[^?]+\?.*event=/);
    await join.click();
    await page.waitForURL(/\/t\//, { timeout: WAIT_MS });

    // The table: four players, each at the Commander starting life.
    const rows = page.locator('[data-seat-row]');
    await expect(rows).toHaveCount(4, { timeout: 120_000 });
    const lives = page.locator('[data-seat-row] [data-stat="life"]');
    await expect(lives).toHaveCount(4);
    for (let i = 0; i < 4; i++) await expect(lives.nth(i)).toHaveAttribute('aria-label', `Life: ${STARTING_LIFE}`);

    expect(bad, 'the commander draft produced browser failures').toEqual([]);
  });
});
