import { type Browser, type Page } from 'playwright';
import { browserURL, sharedBrowser } from '../test/browser';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';

let browser: Browser;
let page: Page;

beforeAll(async () => {
  browser = await sharedBrowser();
});
afterAll(async () => {
  await page?.close();
});

const panel = '[data-paired-panel]';
const state = () => page.getAttribute(panel, 'data-state');
const probe = () => page.evaluate(() => (window as unknown as { __probe: { asked: number[]; posts: number } }).__probe);
const setSeq = (s: number) => page.evaluate((v) => (window as unknown as { __setSeq: (s: number) => void }).__setSeq(v), s);

describe('PairedBotPanel', () => {
  it('stays off until asked, shows only the decision in front of the player, and answers nothing', async () => {
    page = await browser.newPage();
    await page.goto(`${browserURL}src/components/PairedBotPanel.fixture.html`);
    await page.waitForSelector(panel, { timeout: 60_000 });
    expect(await state()).toBe('off');
    await page.waitForTimeout(400);
    expect((await probe()).asked).toEqual([]);

    await page.click(`${panel} .switch`);
    await page.waitForSelector(`${panel}[data-state="working"]`);
    await page.waitForFunction(() => (window as unknown as { __probe: { asked: number[] } }).__probe.asked.length === 1);

    // The decision moves while the bot thinks; the older answer arrives late and never shows.
    await setSeq(2);
    await page.waitForSelector(`${panel} table.search`);
    await page.evaluate(() => (window as unknown as { __releaseFirst: () => void }).__releaseFirst());
    await page.waitForTimeout(100);
    expect(await page.locator(`${panel} table.search tbody tr`).first().textContent()).toContain('Cast Spell 2');
    expect(await page.locator(`${panel} tr.chosen`).textContent()).toContain('its pick');
    expect(await page.locator(`${panel} .foot`).textContent()).toContain('+0.20');
    expect(await page.locator(`${panel}`).textContent()).toContain("can't see hidden cards");

    // Pressing a candidate answers nothing.
    await page.locator(`${panel} table.search tbody tr`).first().click();
    expect((await probe()).posts).toBe(0);

    await setSeq(3);
    await page.waitForSelector(`${panel}[data-state="refused"]`);
    expect(await page.locator(panel).textContent()).toContain('Refused in plain words.');

    await page.click(`${panel} .switch`);
    expect(await state()).toBe('off');
  });
});
