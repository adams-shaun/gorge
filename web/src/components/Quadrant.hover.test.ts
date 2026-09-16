import { type Browser, type ElementHandle, type Page } from 'playwright';
import { browserURL, sharedBrowser } from '../test/browser';
import { beforeAll, describe, expect, it } from 'vitest';

/**
 * The mounted half of the hover-inspector lifetime regression
 * (fb-20260915T182335Z: "mouseover popup oracle should not disappear when the
 * game state progresses between phases"). The CardTile unit pins prove the
 * HoverCard guard, but the reported path is the PARENT keyed list: Quadrant
 * keyed every CardStack by the mutable stacking-equivalence string (g.key),
 * so any state change to a rendered permanent — untap, attack stop, a
 * counter — produced a new key, unmounted CardStack and its CardTile, and
 * destroyed the tile's local hover state with the pointer still on it. Only
 * a real browser fixture exercises that keyed-list update; the SSR harness
 * has no DOM, no pointer events and no $effect.
 *
 * The pointer is moved ONCE onto the tile and then deliberately kept
 * stationary: the whole defect is that a view update unmounts the hovered
 * tile while no pointer event will ever fire again. The dwelled inspector
 * portals to `body > .card-detail#card-detail-<id>`.
 */

let browser: Browser;
let url = '';

beforeAll(async () => {
  url = browserURL;
  browser = await sharedBrowser();
});

const detail = (page: Page, id: number) => `body > .card-detail#card-detail-${id}`;

type Hooks = {
  patch: (patch: Record<string, unknown>) => Promise<void>;
  replace: (card: Record<string, unknown>) => Promise<void>;
  remove: () => Promise<void>;
};

const hooks = (page: Page): Hooks => ({
  patch: (patch) =>
    page.evaluate((p) => (window as unknown as { __patchCard: (x: Record<string, unknown>) => void }).__patchCard(p), patch),
  replace: (card) =>
    page.evaluate((c) => (window as unknown as { __replaceCard: (x: Record<string, unknown>) => void }).__replaceCard(c), card),
  remove: () => page.evaluate(() => (window as unknown as { __removeCard: () => void }).__removeCard()),
});

async function openInspector(page: Page): Promise<{ hooks: Hooks; panel: ElementHandle }> {
  await page.goto(`${url}src/components/Quadrant.fixture.html`);
  await page.locator('[data-obj="7"]').hover();
  return { hooks: hooks(page), panel: await page.waitForSelector(detail(page, 7)) };
}

/** the SAME panel node is still connected: proves the inspector SURVIVED the update rather than being destroyed and re-opened by a synthetic pointer event (Chromium re-hit-tests a stationary cursor after a layout change, so under the mutable key a remounted tile can reopen a panel — a NEW node, never the one the reader was reading). */
const stillConnected = (page: Page, panel: ElementHandle) => page.evaluate((el) => el.isConnected, panel);

describe('the battlefield hover inspector across a keyed-list view update (fb-20260915T182335Z)', () => {
  it('a state update that changes the stacking key keeps the open inspector on the same object (tapped)', async () => {
    const page = await browser.newPage();
    const { hooks: h, panel } = await openInspector(page);

    // The same object 7 gets a new live view whose `tapped` changed — the
    // phase-boundary shape from the report. The pointer stays where it is.
    await h.patch({ tapped: true });

    expect(await stillConnected(page, panel)).toBe(true); // the inspector survived the re-render — the very node, not a re-open

    const view = page.locator(detail(page, 7));
    await view.waitFor({ state: 'visible' });
    await expect.poll(() => view.textContent()).toContain('Grizzly Bears');
    await expect.poll(() => view.textContent()).toContain('#7'); // still the SAME object's detail — never retargeted
    await expect.poll(() => view.textContent()).toContain('tapped'); // and it renders the NEW state, not a stale snapshot
    await page.close();
  });

  it('the same contract for a counters update (the second stackKey field class)', async () => {
    const page = await browser.newPage();
    const { hooks: h, panel } = await openInspector(page);

    await h.patch({ counters: { P1P1: 1 } });

    expect(await stillConnected(page, panel)).toBe(true);

    const view = page.locator(detail(page, 7));
    await view.waitFor({ state: 'visible' });
    await expect.poll(() => view.textContent()).toContain('#7');
    await expect.poll(() => view.textContent()).toContain('P1P1'); // the counters row reflects the updated view
    await page.close();
  });

  it('replacing the tile with another object id closes the detail — no silent retarget', async () => {
    const page = await browser.newPage();
    // Keyboard focus opens the panel with no dwell and the mouse never
    // approaches the tile, so the replacement cannot re-open anything under
    // a stationary pointer (Chromium fires boundary events at a freshly
    // mounted element under the cursor, and that fresh hover is a different
    // contract from retargeting — the pointer tests above own that path).
    await page.goto(`${url}src/components/Quadrant.fixture.html`);
    await page.locator('[data-obj="7"]').focus();
    await page.waitForSelector(detail(page, 7));
    const h = hooks(page);

    await h.replace({ id: 12, name: 'Memorial to Genius', types: 'Land', printing: { name: 'Memorial to Genius' }, token: '' });

    // The old panel leaves the DOM: the object was replaced, not updated —
    // and no panel ever describes the old object again.
    await page.waitForSelector(detail(page, 7), { state: 'detached' });
    await page.waitForTimeout(400); // longer than one dwell: nothing re-arms for the old id
    await expect.poll(() => page.locator('body > .card-detail').count()).toBe(0);
    await page.close();
  });

  it('removing the described object from the battlefield closes the detail', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/Quadrant.fixture.html`);
    await page.locator('[data-obj="7"]').focus();
    await page.waitForSelector(detail(page, 7));
    const h = hooks(page);

    await h.remove();

    await page.waitForSelector(detail(page, 7), { state: 'detached' });
    await expect.poll(() => page.locator('body > .card-detail').count()).toBe(0);
    await page.close();
  });
});
