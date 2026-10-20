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

const panel = '[data-review-panel]';
const intent = () => page.getAttribute(panel, 'data-intent');
const cursor = () => page.evaluate(() => (window as unknown as { __reviewProbe: { cursor: number } }).__reviewProbe.cursor);
const setCursor = (seq: number) => page.evaluate((s) => { (window as unknown as { __reviewProbe: { cursor: number } }).__reviewProbe.cursor = s; }, seq);

async function open(query = '') {
  page ??= await browser.newPage();
  await page.goto(`${browserURL}src/components/DecisionPanel.fixture.html${query}`);
  await page.waitForSelector(`${panel} .quiet, ${panel} .entry`, { timeout: 60_000 });
}

describe('DecisionPanel (match reviewer T5)', () => {
  it('shows the decision at the cursor, dims it between decisions, and jumps with o/f/d', async () => {
    await open();
    await setCursor(15);
    await page.waitForSelector(`${panel}[data-intent="1"]`);
    expect(await page.getAttribute(panel, 'data-exact')).toBe('true');
    // Options with the chosen and bot rows marked, the source badge, the search table.
    expect(await page.locator(`${panel} .options li`).count()).toBe(3);
    expect(await page.locator(`${panel} .options li.chosen`).textContent()).toContain('Cast Lightning Bolt');
    expect(await page.locator(`${panel} .options li.bot`).textContent()).toContain('Pass priority');
    expect(await page.getAttribute(`${panel} .badge`, 'data-source')).toBe('policy');
    expect(await page.locator(`${panel} table.search tbody tr`).count()).toBe(2);
    expect(await page.locator(`${panel} .foot`).textContent()).toContain('+0.30');

    // Between decisions the last one stays, dimmed.
    await setCursor(20);
    await page.waitForSelector(`${panel}[data-exact="false"]`);
    expect(await intent()).toBe('1');

    // o: next override (intent 5, the refusal fallback that changed the answer); O: back.
    await page.keyboard.press('o');
    await page.waitForSelector(`${panel}[data-intent="5"]`);
    expect(await cursor()).toBe(55);
    await page.keyboard.press('Shift+O');
    await page.waitForSelector(`${panel}[data-intent="1"]`);
    // f: next fallback; d: next searched.
    await page.keyboard.press('f');
    await page.waitForSelector(`${panel}[data-intent="3"]`);
    expect(await page.getAttribute(`${panel} .badge`, 'data-source')).toBe('fallback_root');
    await page.keyboard.press('d');
    await page.waitForSelector(`${panel}[data-intent="4"]`);
    await page.keyboard.press('Shift+D');
    await page.waitForSelector(`${panel}[data-intent="1"]`);
    // Nothing later: the cursor stays and the panel says so.
    await setCursor(55);
    await page.keyboard.press('d');
    await page.waitForSelector(`${panel} .missed`);
    expect(await cursor()).toBe(55);
  });

  it('says so when the record carries no explain, and the keys do nothing', async () => {
    await open('?plain=1');
    await setCursor(15);
    await page.waitForSelector(`${panel}[data-intent="1"]`);
    expect(await page.locator(panel).textContent()).toContain('No explain recorded');
    expect(await page.locator(`${panel} .badge`).count()).toBe(0);
    expect(await page.locator(`${panel} .options li.chosen`).textContent()).toContain('Cast Lightning Bolt');
    await page.keyboard.press('o');
    await page.waitForTimeout(100);
    expect(await cursor()).toBe(15);
    await setCursor(0);
    await page.waitForSelector(`${panel} .quiet`);
    expect(await page.locator(panel).textContent()).toContain('first decision is at event 5');
  });
});
