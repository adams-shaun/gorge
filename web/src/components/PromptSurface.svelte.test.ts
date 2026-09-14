import { chromium } from 'playwright';
import { createServer } from 'vite';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';

let server: Awaited<ReturnType<typeof createServer>>;
let browser: Awaited<ReturnType<typeof chromium.launch>>;
let url = '';

beforeAll(async () => {
  server = await createServer({ root: process.cwd(), configLoader: 'runner', server: { port: 0 } });
  await server.listen();
  url = server.resolvedUrls!.local[0];
  browser = await chromium.launch();
});

afterAll(async () => {
  await browser?.close();
  await server?.close();
});

/**
 * The prompt-surface browser gate (brief Jobs 1, 2 and 4): a real browser,
 * because the asks are about what a player can SEE and do — a required
 * prompt visible with the ACTIONS dropdown never opened, and the arrange
 * popup ordering cards by click and by drag and posting the picked order.
 * The fixture mounts the strip and the board prompt surface the way Table
 * mounts them, sharing one SeatPanelState; the arrange case captures the
 * posted intent at window.__posted so the wire shape is asserted without a
 * server.
 */
describe('PromptSurface', () => {
  it('a required prompt is visible and answerable with the ACTIONS dropdown closed', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PromptSurface.fixture.html`);

    const prompt = page.locator('[data-answer-surface] [data-prompt]');
    expect(await prompt.isVisible()).toBe(true);
    expect(await prompt.textContent()).toContain('Choose a target');
    // The prompt surface is NOT inside the (closed) ACTIONS drop: it is its
    // own mount on the board. The drop's own panel — the strip's copy — is a
    // different element, so this count is 0.
    expect(await page.locator('#hot-panel-actions [data-answer-surface]').count()).toBe(0);
    // And the board surface's option list is reachable right there: the seat
    // can answer without ever opening the dropdown.
    const option = page.locator('[data-answer-surface] [data-option="0"]');
    expect(await option.isVisible()).toBe(true);
    await option.click();
    // Answered: the surface drops the DECISION (it stays mounted for the
    // waiting state), so nothing is left to answer on it. The post is async,
    // so wait for the option list to drop rather than racing it.
    await page.waitForFunction(() => document.querySelectorAll('[data-answer-surface] [data-option]').length === 0);

    await page.close();
  });

  it('an initiative decision keeps the ACTIONS drop free of the option list (the prompt/action split)', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PromptSurface.fixture.html`);

    // The drop's SeatPanel names the prompt as living on the board and offers
    // no second posting route for it.
    expect(await page.locator('[data-strip-pointer]').count()).toBe(1);
    expect(await page.locator('#hot-panel-actions [data-option]').count()).toBe(0);
    // The board surface carries the options.
    expect(await page.locator('[data-answer-surface] [data-option]').count()).toBe(1);

    await page.close();
  });
});


describe('ArrangeModal (via the seat panel)', () => {
  it('keeps cards by click, orders them by drag, and posts the picked order', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PromptSurface.fixture.html?case=arrange`);

    await page.locator('[data-arrange-open]').click();
    const keep = page.locator('[data-arrange-keep]');
    const pool = page.locator('[data-arrange-pool]');
    expect(await keep.isVisible()).toBe(true);
    expect(await keep.locator('[data-arrange-keep-card]').count()).toBe(0);
    expect(await pool.locator('[data-arrange-pool-card]').count()).toBe(5);

    // Click three pool cards into the keep pile (click-to-place).
    await pool.locator('[data-arrange-pool-card="1"]').click();
    await pool.locator('[data-arrange-pool-card="0"]').click();
    await pool.locator('[data-arrange-pool-card="3"]').click();
    expect(await keep.locator('[data-arrange-keep-card]').count()).toBe(3);
    expect(await keep.locator('[data-arrange-keep-card="3"] button.face').getAttribute('aria-label')).toBe('3: Spell Pierce');
    expect(await keep.locator('[data-arrange-keep-card="0"] button.face').getAttribute('aria-label')).toBe('2: Brazen Borrower');

    // Drag the keep pile's last card (3) onto its first (1): the pile reads
    // 3, 1, 0. The drag is the HTML5 event sequence a real drag produces.
    await page.evaluate(() => {
      const dt = new DataTransfer();
      const source = document.querySelector('[data-arrange-keep-card="3"]')!;
      const onto = document.querySelector('[data-arrange-keep-card="1"]')!;
      source.dispatchEvent(new DragEvent('dragstart', { dataTransfer: dt, bubbles: true }));
      onto.dispatchEvent(new DragEvent('dragover', { dataTransfer: dt, bubbles: true, cancelable: true }));
      onto.dispatchEvent(new DragEvent('drop', { dataTransfer: dt, bubbles: true, cancelable: true }));
      source.dispatchEvent(new DragEvent('dragend', { dataTransfer: dt, bubbles: true }));
    });
    expect(await keep.locator('[data-arrange-keep-card="3"] button.face').getAttribute('aria-label')).toBe('1: Spell Pierce');
    expect(await keep.locator('[data-arrange-keep-card="1"] button.face').getAttribute('aria-label')).toBe('2: Fabled Pass');

    // Keep the last two cards, then submit the keep pile's order.
    await pool.locator('[data-arrange-pool-card="2"]').click();
    await pool.locator('[data-arrange-pool-card="4"]').click();
    await page.locator('[data-arrange-submit]').click();

    const posted = JSON.parse(await page.evaluate(() => (window as unknown as { __posted?: string }).__posted ?? 'null')) as { seq: number; player: number; choices: number[] };
    expect(posted.seq).toBe(7);
    expect(posted.player).toBe(0);
    expect(posted.choices).toEqual([3, 1, 0, 2, 4]);

    await page.close();
  });

  it('the same final keep order posted through the old click-order picker is byte-identical', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/PromptSurface.fixture.html?case=arrange`);

    // The old answer path, kept on the surface: clicking the arrange row's
    // cards in the final order appends each index, building exactly the
    // picked array the popup posts for the same ordering (lib/arrange.ts's
    // arrangeOrder). 3, 1, 0, 2, 4 again.
    for (const i of [3, 1, 0, 2, 4]) {
      await page.locator(`[data-arrange-row] [data-option="${i}"]`).click();
    }
    await page.locator('[data-submit]').click();

    const posted = JSON.parse(await page.evaluate(() => (window as unknown as { __posted?: string }).__posted ?? 'null')) as { seq: number; player: number; choices: number[] };
    expect(posted.choices).toEqual([3, 1, 0, 2, 4]);

    await page.close();
  });
});
