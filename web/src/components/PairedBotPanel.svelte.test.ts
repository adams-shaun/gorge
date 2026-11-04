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

const pick = () => page.evaluate(() => (window as unknown as { __pick: () => unknown }).__pick());

const open = async (query = '') => {
  page ??= await browser.newPage();
  await page.goto(`${browserURL}src/components/PairedBotPanel.fixture.html${query}`);
  await page.waitForSelector(panel, { timeout: 60_000 });
};

describe('PairedBotPanel', () => {
  it('advises a player alone with bots from the first decision, and marks its pick on the decision', async () => {
    await open();
    await page.evaluate(() => (window as unknown as { __releaseFirst: () => void }).__releaseFirst());
    await page.waitForSelector(`${panel} table.advice`);
    expect(await page.locator(`${panel} .switch`).getAttribute('aria-checked')).toBe('true');
    expect((await probe()).asked).toEqual([1]);
    expect(await pick()).toEqual({ seq: 1, options: [1], payment: null, objs: [], chance: 65 });
    expect(await page.locator('[data-paired-marked]').textContent()).toContain("Bot's pick, 65%");

    // Turning the mark off clears it from the decision and keeps the panel.
    await page.click('[data-paired-mark-setting]');
    expect(await pick()).toBeNull();
    expect(await page.locator(`${panel} table.advice`).count()).toBe(1);
    await page.click('[data-paired-mark-setting]');
    expect(await pick()).not.toBeNull();

    // The next decision clears the old pick before its own advice arrives.
    await setSeq(6);
    await page.waitForSelector(`${panel}[data-state="working"]`);
    expect(await pick()).toBeNull();
    // A server busy with other searches answers later; the panel says so calmly.
    await page.waitForSelector('[data-paired-slow]', { timeout: 10_000 });
    expect(await page.locator(panel).textContent()).toContain('Play on whenever you like');

    // Off is remembered, and clears the mark.
    await setSeq(2);
    await page.waitForSelector(`${panel} table.advice`);
    await page.click(`${panel} .switch`);
    expect(await state()).toBe('off');
    expect(await pick()).toBeNull();
    expect(await page.evaluate(() => localStorage.getItem('gorge.pairedbot.v1'))).toBe('off');
  });

  it('at a table with another human, stays off until asked, shows only the decision in front of the player, notes a different answer, and answers nothing', async () => {
    await open('?shared=1');
    await page.waitForSelector(`${panel}[data-state="off"]`);
    expect(await page.locator(panel).textContent()).toContain('Any player at this table may be asking');
    await page.waitForTimeout(400);
    expect((await probe()).asked).toEqual([]);

    await page.click(`${panel} .switch`);
    await page.waitForSelector(`${panel}[data-state="working"]`);
    await page.waitForFunction(() => (window as unknown as { __probe: { asked: number[] } }).__probe.asked.length === 1);

    // The decision moves while the bot thinks; the older answer arrives late and never shows.
    await setSeq(2);
    await page.waitForSelector(`${panel} table.advice`);
    await page.evaluate(() => (window as unknown as { __releaseFirst: () => void }).__releaseFirst());
    await page.waitForTimeout(100);
    expect(await page.locator(`${panel} table.advice tbody tr`).first().textContent()).toContain('Cast Spell 2');
    const best = page.locator(`${panel} tr.best`);
    expect(await best.textContent()).toContain('Recommended');
    expect(await best.textContent()).toContain('Cast Spell 2');
    expect(await best.textContent()).toContain('65%');
    expect(await best.textContent()).toContain('60%');
    const second = page.locator(`${panel} tbody tr`).nth(1);
    expect(await second.textContent()).toContain('45%');
    expect(await second.textContent()).toContain('40%');

    // What the bot's rules would do is marked, and overruled, until the player turns that off.
    expect(await second.textContent()).toContain("The bot's rules");
    expect(await page.locator('[data-paired-overruled]').count()).toBe(1);
    await page.click('[data-paired-rules-setting]');
    expect(await second.textContent()).not.toContain("The bot's rules");
    expect(await page.locator('[data-paired-overruled]').count()).toBe(0);
    await page.click('[data-paired-rules-setting]');
    expect(await page.locator('[data-paired-summary]').textContent()).toContain('Search looked at 100 lines');
    expect(await page.locator(`${panel}`).textContent()).toContain('from what you can see');

    // Pressing a candidate answers nothing.
    await page.locator(`${panel} table.advice tbody tr`).first().click();
    expect((await probe()).posts).toBe(0);

    // Answering other than the recommendation leaves a note, once; the recommendation leaves none.
    const answer = (a: object) => page.evaluate((v) => (window as unknown as { __answer: (a: object) => void }).__answer(v), a);
    const differed = () => page.evaluate(() => (window as unknown as { __differed: string[] }).__differed);
    await answer({ seq: 2, choices: [0] });
    await page.waitForSelector('[data-paired-differed]');
    expect(await differed()).toEqual(['You differed: Pass priority, estimated 45%. Search preferred Cast Spell 2, estimated 65%.']);
    await answer({ seq: 2, choices: [0] });
    await page.waitForTimeout(50);
    expect(await differed()).toHaveLength(1);

    await setSeq(3);
    await page.waitForSelector(`${panel}[data-state="refused"]`);
    expect(await page.locator(panel).textContent()).toContain('Refused in plain words.');

    await page.click(`${panel} .switch`);
    expect(await state()).toBe('off');
  });

  it('calls a request the game overtook out of date and offers it again, and never shows a server error', async () => {
    await open();
    await setSeq(4);
    await page.waitForSelector(`${panel}[data-state="stale"]`);
    expect(await page.locator(panel).textContent()).toContain('This advice is out of date.');
    await page.click(`${panel} .again`);
    await page.waitForSelector(`${panel} table.advice`);
    expect(await page.locator(`${panel} tr.best`).textContent()).toContain('Cast Spell 4');

    await setSeq(5);
    await page.waitForSelector(`${panel}[data-state="failed"]`);
    const text = await page.locator(panel).textContent();
    expect(text).toContain('The bot did not answer.');
    expect(text).not.toContain('host:');
  });
});
