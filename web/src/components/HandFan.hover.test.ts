import { type Browser, type ElementHandle, type Page } from 'playwright';
import { beforeAll, describe, expect, it } from 'vitest';
import { browserURL, sharedBrowser } from '../test/browser';

let browser: Browser;
let url = '';

beforeAll(async () => {
  url = browserURL;
  browser = await sharedBrowser();
});

const detail = (page: Page, id: number) => `body > .card-detail#card-detail-${id}`;
const call = (page: Page, name: '__refreshHand' | '__replaceId' | '__removeCard', id?: number) =>
  page.evaluate(({ name, id }) => (window as unknown as Record<string, (id?: number) => void>)[name](id), { name, id });
const connected = (page: Page, panel: ElementHandle) => page.evaluate((el) => el.isConnected, panel);

async function open(page: Page, focus = false): Promise<ElementHandle> {
  await page.goto(`${url}src/components/HandFan.fixture.html`);
  const face = page.locator('[data-obj="7"] .face');
  if (focus) await face.focus();
  else await face.hover();
  const panel = await page.waitForSelector(detail(page, 7));
  expect(await connected(page, panel)).toBe(true);
  expect(await page.locator(detail(page, 7)).isVisible()).toBe(true);
  return panel;
}

describe('HandFan inspector across live hand replacement', () => {
  it('keeps the same open panel and renders fresh card data for a stable id', async () => {
    const page = await browser.newPage();
    const panel = await open(page);
    const oldText = await page.locator(detail(page, 7)).innerText();
    expect(oldText).toContain('Grizzly Bears');

    await call(page, '__refreshHand');

    expect(await connected(page, panel)).toBe(true);
    const fresh = page.locator(detail(page, 7));
    await expect.poll(() => fresh.innerText()).toContain('Refreshed Grizzly Bears');
    expect(await fresh.innerText()).not.toBe(oldText);
    await page.close();
  });

  it('closes when the described card is removed from the hand', async () => {
    const page = await browser.newPage();
    await open(page, true);
    await call(page, '__removeCard');
    await page.waitForSelector(detail(page, 7), { state: 'detached' });
    expect(await page.locator('body > .card-detail').count()).toBe(0);
    await page.close();
  });

  it('closes rather than retargeting when the hand card id changes', async () => {
    const page = await browser.newPage();
    await open(page, true);
    await call(page, '__replaceId', 12);
    await page.waitForSelector(detail(page, 7), { state: 'detached' });
    expect(await page.locator('body > .card-detail').count()).toBe(0);
    await page.close();
  });
});
