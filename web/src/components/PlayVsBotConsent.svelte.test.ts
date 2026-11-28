import { readFileSync } from 'node:fs';
import { type Browser, type Page } from 'playwright';
import { afterEach, beforeAll, beforeEach, describe, expect, it } from 'vitest';
import type { DeckReport } from '../protocol';
import { browserURL, sharedBrowser } from '../test/browser';
import type { HeldPost } from './PlayVsBotConsent.fixture';

// The check and consent wiring of the real PlayVsBot in a browser, where its
// handlers and reactivity run: which requests it sends, which answer it shows,
// and that consent never outlives the report it was given for.
const swapped = JSON.parse(readFileSync(new URL('../../../server/host/httpapi/testdata/web-deck-report.json', import.meta.url), 'utf8')) as DeckReport;
const unswapped: DeckReport = { ...swapped, substituted: [] };
const LIST = 'Commander\n1 Ghoulcaller Gisa\n\n1 Sol Ring';
const OTHER = 'Commander\n1 Krenko, Mob Boss\n\n1 Sol Ring';

type Posts = { posts: HeldPost[] };

describe('PlayVsBot deck check and consent', () => {
  let browser: Browser;
  let page: Page;

  beforeAll(async () => {
    browser = await sharedBrowser();
  });

  beforeEach(async () => {
    page = await browser.newPage();
    await page.goto(`${browserURL}src/components/PlayVsBotConsent.fixture.html`);
    await page.getByTestId('deck-text').fill(LIST);
  });

  afterEach(async () => {
    await page.close();
  });

  /** settle lets every queued answer and the redraw it causes finish. */
  const settle = () => page.evaluate(() => new Promise<void>((r) => setTimeout(r, 0)));
  const sent = async () => {
    await settle();
    return page.evaluate(() => (window as unknown as Posts).posts.map((p) => ({ to: p.to, body: p.body })));
  };
  const consents = async () => (await sent()).map((p) => p.body.substitute);
  const reply = async (i: number, status: number, payload: unknown) => {
    await page.evaluate(([n, s, p]) => (window as unknown as Posts).posts[n as number].reply(s as number, p), [i, status, payload] as const);
    await settle();
  };

  const panels = () => page.getByTestId('deck-report').count();
  const swaps = () => page.getByTestId('deck-report-substituted').count();
  const box = () => page.getByTestId('deck-substitute');
  const check = () => page.getByTestId('deck-check').click();
  const start = () => page.getByRole('button', { name: 'Start game' }).click();
  const errorLine = async () => ((await page.locator('p.error').count()) ? page.locator('p.error').textContent() : null);

  /** consented checks the list, ticks the box and answers both with the server's reports: posts 0 and 1. */
  async function consented() {
    await check();
    await reply(0, 200, unswapped);
    await box().check();
    await reply(1, 200, swapped);
    expect(await consents()).toEqual([false, true]);
    expect(await box().isChecked()).toBe(true);
    expect(await swaps()).toBe(2);
  }

  /** startedDeck presses Start and returns the seat entry that was sent. */
  async function startedDeck() {
    const before = (await sent()).length;
    await start();
    const posts = await sent();
    expect(posts).toHaveLength(before + 1);
    expect(posts[before].to).toBe('start');
    return (posts[before].body.decks as Record<string, unknown>[])[0];
  }

  it('re-posts once per tick and never because the list or format changed', async () => {
    await consented();
    const text = page.getByTestId('deck-text');
    await text.press('End');
    await text.pressSequentially('\n1 Sw');
    await settle();
    await page.getByRole('radio', { name: 'Commander' }).check();
    expect(await consents()).toEqual([false, true]);
  });

  it('shows the answer for the latest tick when an earlier one lands after it', async () => {
    await check();
    await reply(0, 200, unswapped);
    await box().check();
    await box().uncheck();
    expect(await consents()).toEqual([false, true, false]);
    await reply(2, 200, unswapped);
    await reply(1, 200, swapped);
    expect(await box().isChecked()).toBe(false);
    expect(await swaps()).toBe(0);
    expect(await startedDeck()).toEqual({ seat: 0, text: LIST });
  });

  it('keeps checking until the latest check is answered, ignoring a stale reply or failure', async () => {
    await check();
    await reply(0, 200, unswapped);
    await box().check();
    await box().uncheck();
    await box().check();
    expect(await consents()).toEqual([false, true, false, true]);
    await reply(1, 200, swapped);
    await reply(2, 429, { code: 'rate_limited', message: 'slow down' });
    expect(await page.getByTestId('deck-check').isDisabled()).toBe(true);
    expect(await swaps()).toBe(0);
    expect(await box().isChecked()).toBe(true);
    expect(await errorLine()).toBeNull();
    await reply(3, 200, swapped);
    expect(await page.getByTestId('deck-check').isEnabled()).toBe(true);
    expect(await swaps()).toBe(2);
  });

  it('drops the report and the consent when the pasted list changes', async () => {
    await consented();
    await page.getByTestId('deck-text').fill(OTHER);
    expect(await panels()).toBe(0);
    expect(await startedDeck()).toEqual({ seat: 0, text: OTHER });
    await reply(2, 429, { code: 'rate_limited', message: 'slow down' });
    await check();
    expect((await sent())[3].body).toEqual({ format: 'constructed', text: OTHER, substitute: false });
  });

  it('drops the report and the consent when the format changes', async () => {
    await consented();
    await page.getByRole('radio', { name: 'Commander' }).check();
    expect(await panels()).toBe(0);
    expect(await startedDeck()).toEqual({ seat: 0, text: LIST });
  });

  it('withdraws the consent when the re-posted check fails', async () => {
    await check();
    await reply(0, 200, unswapped);
    await box().check();
    await reply(1, 429, { code: 'rate_limited', message: 'slow down' });
    expect(await errorLine()).toBe('slow down');
    expect(await panels()).toBe(0);
    expect(await startedDeck()).toEqual({ seat: 0, text: LIST });
  });

  it('withdraws the consent when the new report leaves nothing to consent to', async () => {
    await consented();
    await check();
    await reply(2, 200, { ...unswapped, unsupported: [] });
    expect(await box().count()).toBe(0);
    expect(await startedDeck()).toEqual({ seat: 0, text: LIST });
  });

  it.each([
    ['report', 200, swapped],
    ['failure', 429, { code: 'rate_limited', message: 'slow down' }],
  ])('ignores a late %s for a list that has since changed', async (_name, status, payload) => {
    await check();
    await page.getByTestId('deck-text').fill(OTHER);
    expect(await page.getByTestId('deck-check').isEnabled()).toBe(true);
    await reply(0, status, payload);
    expect(await panels()).toBe(0);
    expect(await errorLine()).toBeNull();
  });

  it('sends the ticked consent on Start and shows the refused deck report over a check still in flight', async () => {
    await consented();
    await check();
    expect(await startedDeck()).toEqual({ seat: 0, text: LIST, substitute: true });
    const problem = '4 cards this engine cannot play yet; at most 3 can be substituted';
    await reply(3, 400, { code: 'deck_refused', message: 'seat 0 deck: refused', deck: { ...unswapped, problems: [problem] } });
    await reply(2, 200, swapped);
    expect(await page.getByTestId('deck-report-problem').textContent()).toBe(problem);
    expect(await swaps()).toBe(0);
    expect(await errorLine()).toBe('seat 0 deck: refused');
    expect(await page.getByTestId('deck-check').isEnabled()).toBe(true);
  });

  it('does not show a refusal for a list that has changed since Start', async () => {
    await start();
    await page.getByTestId('deck-text').fill(OTHER);
    await reply(0, 400, { code: 'deck_refused', message: 'seat 0 deck: refused', deck: unswapped });
    expect(await errorLine()).toBe('seat 0 deck: refused');
    expect(await panels()).toBe(0);
  });
});
