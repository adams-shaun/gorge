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
