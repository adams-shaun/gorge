import { type Browser } from 'playwright';
import { browserURL, sharedBrowser } from '../test/browser';
import { beforeAll, describe, expect, it } from 'vitest';

let browser: Browser;
let url = '';

declare global {
  interface Window {
    __flip: () => void;
    __end: () => void;
    __viewChanged: () => boolean;
    __controlsPresent: () => boolean;
  }
}

beforeAll(async () => {
  url = browserURL;
  browser = await sharedBrowser();
});

describe('starting-player to mulligan teardown', () => {
  it('keeps the board and safely unmounts the controls after the arriving view changes', async () => {
    const page = await browser.newPage();
    const errors: string[] = [];
    page.on('pageerror', (error) => errors.push(error.message));
    try {
      await page.goto(`${url}src/components/BoardStageTeardown.fixture.html`);
      await page.locator('[data-hot-strip]').waitFor({ state: 'attached' });
      // Preconditions: a real mounted strip/SeatPanel, and a distinct next view.
      expect(await page.locator('[data-hot-strip]').count()).toBe(1);
      expect(await page.evaluate(() => window.__viewChanged())).toBe(false);
      await page.evaluate(() => window.__flip());
      expect(await page.evaluate(() => window.__viewChanged())).toBe(true);
      await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => resolve())));
      expect(await page.evaluate(() => window.__controlsPresent())).toBe(true);
      expect(await page.locator('[data-hot-strip]').count()).toBe(0);
      expect(errors).toEqual([]);
      expect(await page.locator('[data-board-stage]').count()).toBe(1);
    } finally {
      await page.close();
    }
  });

  it('unmounts on game over even while the controls object stays safe to read', async () => {
    const page = await browser.newPage();
    const errors: string[] = [];
    page.on('pageerror', (error) => errors.push(error.message));
    try {
      await page.goto(`${url}src/components/BoardStageTeardown.fixture.html`);
      await page.locator('[data-hot-strip]').waitFor({ state: 'attached' });
      expect(await page.locator('[data-hot-strip]').count()).toBe(1);
      expect(await page.evaluate(() => window.__viewChanged())).toBe(false);
      await page.evaluate(() => window.__end());
      await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => resolve())));
      expect(await page.evaluate(() => window.__viewChanged())).toBe(true);
      expect(await page.evaluate(() => window.__controlsPresent())).toBe(true);
      expect(await page.locator('[data-hot-strip]').count()).toBe(0);
      expect(errors).toEqual([]);
      expect(await page.locator('[data-board-stage]').count()).toBe(1);
    } finally {
      await page.close();
    }
  });
});
