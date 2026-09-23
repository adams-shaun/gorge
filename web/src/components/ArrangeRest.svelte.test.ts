import { beforeAll, describe, expect, it } from 'vitest';
import { type Browser } from 'playwright';
import { browserURL, sharedBrowser } from '../test/browser';

let browser: Browser;
let url = '';
beforeAll(async () => {
  url = browserURL;
  browser = await sharedBrowser();
});

describe('Restable arrange: player orders both piles', () => {
  for (const kind of ['scry', 'surveil'] as const) {
    it(`${kind} posts the visibly chosen pile-B order, not the offered order`, async () => {
      const page = await browser.newPage();
      try {
        await page.goto(`${url}src/components/PromptSurface.fixture.html?case=${kind}`, { timeout: 5000 });
        await page.locator('[data-arrange-open]').click({ timeout: 5000 });
        const keep = page.locator('[data-arrange-keep]');
        const pool = page.locator('[data-arrange-pool]');
        expect(await pool.locator('[data-arrange-pool-card]').count()).toBe(5); // real unchosen cards
        expect(await page.locator('.pile.pool h3').textContent()).toContain(kind === 'scry' ? 'bottom of the library' : 'graveyard');
        expect(await pool.locator('[data-arrange-pool-card="4"]').getAttribute('draggable')).toBe('true');
        await pool.locator('[data-arrange-pool-card="0"] button.face').click();
        expect(await keep.locator('[data-arrange-keep-card="0"]').count()).toBe(1);
        // Moving the LAST unchosen card onto the FIRST unchosen card must
        // change their order; no choice of pile A alone can encode this.
        await page.evaluate(() => {
          const dt = new DataTransfer();
          const source = document.querySelector('[data-arrange-pool-card="4"]')!;
          const target = document.querySelector('[data-arrange-pool-card="1"]')!;
          source.dispatchEvent(new DragEvent('dragstart', { dataTransfer: dt, bubbles: true }));
          target.dispatchEvent(new DragEvent('dragover', { dataTransfer: dt, bubbles: true, cancelable: true }));
          target.dispatchEvent(new DragEvent('drop', { dataTransfer: dt, bubbles: true, cancelable: true }));
          source.dispatchEvent(new DragEvent('dragend', { dataTransfer: dt, bubbles: true }));
        });
        const displayed = await pool.locator('[data-arrange-pool-card]').evaluateAll((els) => els.map((el) => Number(el.getAttribute('data-arrange-pool-card'))));
        expect(displayed).toEqual([4, 1, 2, 3]);
        expect(displayed).not.toEqual([1, 2, 3, 4]); // offered order actually differs
        await page.locator('[data-arrange-submit]').click();
        await page.waitForFunction(() => (window as unknown as { __posted?: string }).__posted !== undefined, null, { timeout: 5000 });
        const posted = JSON.parse(await page.evaluate(() => (window as unknown as { __posted?: string }).__posted!)) as { choices: number[]; rest?: number[] };
        expect(posted.choices).toEqual([0]);
        expect(posted.rest).toEqual(displayed);
      } finally {
        await page.close();
      }
    }, 20_000);
  }
});
