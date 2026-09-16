import { type Browser, type Page } from 'playwright';
import { browserURL, sharedBrowser } from '../test/browser';
import { beforeAll, describe, expect, it } from 'vitest';

let browser: Browser;
let url = '';

beforeAll(async () => {
  url = browserURL;
  browser = await sharedBrowser();
});

describe('PileModal', () => {
  it('opens from a pile caret, labels and focuses the dialog, and renders card names with types', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PileModal.fixture.html`);
    await page.locator('[data-pile="graveyard"]').click();

    const dialog = page.getByRole('dialog', { name: "Alice's graveyard" });
    expect(await dialog.isVisible()).toBe(true);
    expect(await page.evaluate(() => document.activeElement?.getAttribute('role'))).toBe('dialog');
    const cardText = await dialog.locator('[data-obj="52"]').textContent();
    expect(cardText).toContain('Archive Card 52');
    expect(cardText).toContain('Creature — Wizard');
    await page.close();
  });

  it('closes with Escape, backdrop, and the visible close control, returning focus every time', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PileModal.fixture.html`);
    const trigger = page.locator('[data-pile="graveyard"]');

    await trigger.click();
    await page.keyboard.press('Escape');
    expect(await page.getByRole('dialog').count()).toBe(0);
    expect(await trigger.evaluate((node) => node === document.activeElement)).toBe(true);

    await trigger.click();
    await page.getByRole('button', { name: "Close Alice's graveyard" }).click();
    expect(await page.getByRole('dialog').count()).toBe(0);
    expect(await trigger.evaluate((node) => node === document.activeElement)).toBe(true);

    await trigger.click();
    await page.locator('[data-pile-backdrop]').click({ position: { x: 2, y: 2 } });
    expect(await page.getByRole('dialog').count()).toBe(0);
    expect(await trigger.evaluate((node) => node === document.activeElement)).toBe(true);
    await page.close();
  });

  it('caps a long pile below the viewport and scrolls it instead of growing the page', async () => {
    const page = await browser.newPage({ viewport: { width: 900, height: 700 } });
    await page.goto(`${url}src/components/PileModal.fixture.html`);
    await page.locator('[data-pile="graveyard"]').click();
    const metrics = await page.locator('[data-pile-scroll]').evaluate((node) => {
      const el = node as HTMLElement;
      const style = getComputedStyle(el);
      return { overflowY: style.overflowY, height: el.clientHeight, scrollHeight: el.scrollHeight };
    });
    expect(metrics.overflowY).toBe('auto');
    expect(metrics.height).toBeLessThanOrEqual(490); // 70vh of the 700px fixture viewport
    expect(metrics.scrollHeight).toBeGreaterThan(metrics.height);
    await page.close();
  });
});

describe('PileModal — the card list\'s shared hover inspector', () => {
  // The ?case=shrink fixture mounts PileModal itself, open on a live $state
  // pile (window.__pileRemoveFirst shrinks it) — the SeatTable mount above
  // cannot change the cards prop while the modal sits open.
  const openShrink = async (): Promise<Page> => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PileModal.fixture.html?case=shrink`);
    await page.getByRole('dialog', { name: 'Fixture graveyard' }).waitFor({ state: 'visible' });
    return page;
  };

  it('a pointer dwell on a graveyard card opens the detail describing that card, and pointerleave closes it', async () => {
    const page = await openShrink();
    const first = page.locator('[data-obj="1"] button');
    await first.hover();
    const detail = page.locator('#card-detail-1');
    await detail.waitFor({ state: 'visible', timeout: 5_000 }); // the ~250ms dwell
    expect(await detail.textContent()).toContain('Fixture Card 1');
    // the described-by wire follows the open panel
    expect(await first.getAttribute('aria-describedby')).toBe('card-detail-1');

    await page.mouse.move(4, 4); // pointerleave: the panel closes
    await detail.waitFor({ state: 'hidden', timeout: 5_000 });
    expect(await page.locator('body > .card-detail').count()).toBe(0);
    await page.close();
  });

  it('keyboard focus opens the detail immediately and blur closes it', async () => {
    const page = await openShrink();
    const second = page.locator('[data-obj="2"] button');
    await second.focus();
    const detail = page.locator('#card-detail-2');
    await detail.waitFor({ state: 'visible', timeout: 1_000 }); // no dwell on the focus path
    expect(await second.getAttribute('aria-describedby')).toBe('card-detail-2');

    await second.evaluate((node) => (node as HTMLElement).blur());
    await detail.waitFor({ state: 'hidden', timeout: 5_000 });
    expect(await page.locator('body > .card-detail').count()).toBe(0);
    await page.close();
  });

  it('a card leaving the pile while its detail is open closes the panel and keeps the modal open', async () => {
    const page = await openShrink();
    // The removed card is the hovered FINAL one: removing a final card
    // reflows nothing under the stationary pointer, so the assertion below
    // cannot be flaked by the browser re-arming the dwell on whatever shifts
    // into the pointer's spot (removing a non-final card moves its neighbour
    // under the pointer, which legitimately opens a NEW card's panel a dwell
    // later — a race a removal test must not run).
    await page.locator('[data-obj="3"] button').hover();
    const detail = page.locator('#card-detail-3');
    await detail.waitFor({ state: 'visible', timeout: 5_000 });

    // The pile changes under the pointer (play continues while the modal
    // sits open); the removed row never fires pointerleave, so supervise is
    // what must close the panel.
    await page.evaluate(() => (window as unknown as { __pileRemoveLast: () => void }).__pileRemoveLast());
    await detail.waitFor({ state: 'hidden', timeout: 5_000 });
    expect(await page.locator('body > .card-detail').count()).toBe(0);
    // the modal itself stayed open, now on the shorter pile
    expect(await page.getByRole('dialog', { name: 'Fixture graveyard' }).isVisible()).toBe(true);
    expect(await page.locator('[data-obj="3"]').count()).toBe(0);
    expect(await page.locator('[data-obj="1"]').count()).toBe(1);
    await page.close();
  });

  it('Escape peels layers on BOTH open paths — a pointer-dwell detail and a focused one — the modal only on the next press', async () => {
    const page = await openShrink();
    // POINTER PATH (the regression this test pins): a dwell opens the detail
    // while the DIALOG keeps focus — no card button owns the key event, so
    // the window-level handler is what must peel the layer, not a per-card
    // keydown that never runs.
    await page.locator('[data-obj="1"] button').hover();
    const detail = page.locator('#card-detail-1');
    await detail.waitFor({ state: 'visible', timeout: 5_000 });
    expect(await page.evaluate(() => document.activeElement?.getAttribute('role'))).toBe('dialog');
    await page.keyboard.press('Escape');
    await detail.waitFor({ state: 'hidden', timeout: 5_000 });
    expect(await page.getByRole('dialog', { name: 'Fixture graveyard' }).isVisible()).toBe(true);

    // FOCUS PATH: a focused card's detail peels the same way.
    await page.locator('[data-obj="2"] button').focus();
    await page.locator('#card-detail-2').waitFor({ state: 'visible', timeout: 1_000 });
    await page.keyboard.press('Escape');
    await page.locator('#card-detail-2').waitFor({ state: 'hidden', timeout: 5_000 });
    expect(await page.getByRole('dialog', { name: 'Fixture graveyard' }).isVisible()).toBe(true);

    // With no panel open, Escape closes the modal itself.
    await page.keyboard.press('Escape');
    await page.getByRole('dialog', { name: 'Fixture graveyard' }).waitFor({ state: 'hidden', timeout: 5_000 });
    await page.close();
  });

  it('the detail opened from inside the pile modal renders ABOVE the modal backdrop, and the hotkey guard marker stays', async () => {
    const page = await openShrink();
    await page.locator('[data-obj="1"] button').hover();
    await page.locator('#card-detail-1').waitFor({ state: 'visible', timeout: 5_000 });

    const order = await page.evaluate(() => {
      const detail = document.querySelector('body > .card-detail');
      const backdrop = document.querySelector('[data-pile-backdrop]');
      if (!detail || !backdrop) return null;
      const z = (el: Element) => Number.parseFloat(getComputedStyle(el).zIndex);
      return { detail: z(detail), backdrop: z(backdrop), guard: document.querySelector('[data-pile-modal]') !== null };
    });
    expect(order).not.toBeNull();
    expect(order!.detail).toBeGreaterThan(order!.backdrop); // the panel reads over the dim
    expect(order!.guard).toBe(true); // the modal guard still suppresses document hotkeys
    await page.close();
  });
});
