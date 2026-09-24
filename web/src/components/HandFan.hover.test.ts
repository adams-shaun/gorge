import { type Browser, type ElementHandle, type Page } from 'playwright';
import { browserURL, sharedBrowser } from '../test/browser';
import { beforeAll, describe, expect, it } from 'vitest';

/**
 * The mounted half of the hand-fan hover-inspector lifetime regression
 * (fb-20260923T020655Z: "mouse over gives popup art, but it always disappears
 * on any tick in the client"). The same class as the battlefield fix
 * fb-20260915T182335Z, but on a surface its scope boundary excluded: HandFan
 * gated its CardDetail on `hover.show && hovered === c`, object IDENTITY.
 *
 * Every live view refresh (`MatchState.refreshLive` on a 'decision' frame)
 * assigns a freshly deserialized view wholesale. The keyed each
 * `{#each hand as c, i (c.id)}` keeps the DOM node — the pointer never leaves
 * it, so no pointer event ever re-fires — but hands it a FRESH CardView for
 * the same id, so `hovered === c` flipped false and the `{#if}` unmounted the
 * open panel. Only a real browser fixture exercises that keyed-list update;
 * the SSR harness (HandFan.svelte.test.ts) has no DOM, no pointer events and
 * no $effect.
 *
 * The pointer is moved ONCE onto a card and then deliberately kept
 * stationary: the whole defect is that a view update unmounts the panel while
 * no pointer event will ever fire again. The dwelled inspector portals to
 * `body > .card-detail#card-detail-<id>`.
 */

let browser: Browser;
let url = '';

beforeAll(async () => {
  url = browserURL;
  browser = await sharedBrowser();
});

const detail = (page: Page, id: number) => `body > .card-detail#card-detail-${id}`;

type Hooks = {
  tick: (over?: Record<string, unknown>) => Promise<void>;
  replaceId: (from: number, to: number) => Promise<void>;
  remove: (id: number) => Promise<void>;
};

const hooks = (page: Page): Hooks => ({
  tick: (over) =>
    page.evaluate((o) => (window as unknown as { __tick: (x?: Record<string, unknown>) => void }).__tick(o), over),
  replaceId: (from, to) =>
    page.evaluate(
      ([f, t]) => (window as unknown as { __replaceId: (a: number, b: number) => void }).__replaceId(f, t),
      [from, to] as const,
    ),
  remove: (id) =>
    page.evaluate((i) => (window as unknown as { __remove: (a: number) => void }).__remove(i), id),
});

/** Hover object 7 and wait for its dwelled inspector. */
async function openInspector(page: Page): Promise<{ hooks: Hooks; panel: ElementHandle }> {
  await page.goto(`${url}src/components/HandFan.fixture.html`);
  await page.locator('[data-obj="7"]').hover();
  return { hooks: hooks(page), panel: await page.waitForSelector(detail(page, 7)) };
}

/** the SAME panel node is still connected: proves the inspector SURVIVED the update rather than being destroyed and re-opened. */
const stillConnected = (page: Page, panel: ElementHandle) => page.evaluate((el) => el.isConnected, panel);

describe('the hand-fan hover inspector across a keyed-list view refresh (fb-20260923T020655Z)', () => {
  it('a live view refresh that rebuilds the hand with fresh objects keeps the open inspector on the same card', async () => {
    const page = await browser.newPage();
    const { hooks: h, panel } = await openInspector(page);

    // Precondition for the freshness assertion: the panel is genuinely open,
    // connected, and does NOT yet carry the field the refresh will change.
    expect(await stillConnected(page, panel)).toBe(true);
    await expect.poll(() => page.locator(detail(page, 7)).textContent()).not.toContain('tapped');

    // The tick: the whole hand is rebuilt with fresh-identity objects, same
    // ids (exactly what refreshLive's setRenderedView(v, seq) does). The
    // pointer stays where it is.
    await h.tick({ tapped: true });

    expect(await stillConnected(page, panel)).toBe(true); // the very node survived the refresh — not a re-open

    const view = page.locator(detail(page, 7));
    await view.waitFor({ state: 'visible' });
    await expect.poll(() => view.textContent()).toContain('Grizzly Bears');
    await expect.poll(() => view.textContent()).toContain('#7'); // still the SAME card's detail — never retargeted
    await expect.poll(() => view.textContent()).toContain('tapped'); // and it renders the REFRESHED view, not a stale snapshot
    await page.close();
  });

  it('removing the hovered card from the hand closes the panel', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/HandFan.fixture.html`);
    // Focus opens the panel immediately (no dwell); the mouse never approaches
    // the card, so the removal cannot get a synthetic re-hover. The fan's
    // focusable face is the inner .face (the outer [data-obj] wrapper is not
    // focusable), unlike CardTile whose root carries tabindex.
    await page.locator('[data-obj="7"] .face').focus();
    await page.waitForSelector(detail(page, 7));
    const h = hooks(page);

    await h.remove(7);

    await page.waitForSelector(detail(page, 7), { state: 'detached' });
    await page.waitForTimeout(400); // longer than one dwell: nothing re-arms for the gone card
    await expect.poll(() => page.locator('body > .card-detail').count()).toBe(0);
    await page.close();
  });

  it('replacing the hovered card id closes the panel — no silent retarget', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/HandFan.fixture.html`);
    await page.locator('[data-obj="7"] .face').focus();
    await page.waitForSelector(detail(page, 7));
    const h = hooks(page);

    await h.replaceId(7, 12);

    await page.waitForSelector(detail(page, 7), { state: 'detached' });
    // The replaced card is a DIFFERENT object; no panel may ever describe the
    // old id again, and none silently follows the new one.
    await page.waitForTimeout(400);
    await expect.poll(() => page.locator(detail(page, 7)).count()).toBe(0);
    await page.close();
  });
});
