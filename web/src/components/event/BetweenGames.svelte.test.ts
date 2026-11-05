import { beforeAll, describe, expect, it, vi } from 'vitest';
import { render } from 'svelte/server';
import type { Browser, Page } from 'playwright';
import { browserURL, sharedBrowser } from '../../test/browser';
import { eventState } from '../../test/eventfixtures';
import type { EventSeries } from '../../protocol';
import BetweenGames from './BetweenGames.svelte';

vi.mock('../../lib/images', () => ({ images: { url: () => new Promise<string | null>(() => {}), offline: () => false } }));

const seats = eventState().seats;
const row = (over: Partial<EventSeries> = {}): EventSeries => ({
  round: 1, a: 0, b: 3, best_of: 3, state: 'between', table: 'e1-r1-s03-g1-a1', game: 2, games: [3], chooser: 0, waiting: [0, 3], score: [0, 1], winner: null, ...over,
});
const ssr = (series: EventSeries, you = 0) => render(BetweenGames, { props: { event: 'e1', token: 'evtok0', you, series, seats } }).body;

// SSR pins what the panel SAYS from the server's series row; nothing loads
// (onMount never runs). The clicks run in the shared Chromium below.
describe('BetweenGames (SSR)', () => {
  it('the loser of game 1 is told they choose play or draw; the score reads from their side', () => {
    const body = ssr(row());
    expect(body).toContain('Game 2 of 3 — you lost game 1');
    expect(body).toMatch(/data-testid="sb-chooser"[^>]*>You will choose play or draw</);
    expect(body).toMatch(/data-testid="sb-score"[^>]*>You 0 – 1 Seat 4 · Bot 4 \(bot\)</);
    expect(body).toMatch(/data-testid="sb-opponent"[^>]*>opponent: sideboarding…</);
    expect(body).toContain('Loading your pool…');
  });

  it('the winner is told who chooses; a ready opponent shows as ready', () => {
    const body = ssr(row({ a: 3, b: 0, games: [0], score: [0, 1], chooser: 3, waiting: [0] }));
    expect(body).toContain('Game 2 of 3 — you won game 1');
    expect(body).toMatch(/data-testid="sb-chooser"[^>]*>Seat 4 · Bot 4 \(bot\) chooses play or draw</);
    expect(body).toMatch(/data-testid="sb-score"[^>]*>You 1 – 0 Seat 4/);
    expect(body).toMatch(/data-testid="sb-opponent"[^>]*>opponent ready</);
  });

  it('a drawn game says so; after the viewer\'s deck is in it waits for the opponent', () => {
    const body = ssr(row({ game: 3, games: [3, -1], score: [0, 1], waiting: [3] }));
    expect(body).toContain('Game 3 of 3 — game 2 was drawn');
    expect(body).toMatch(/data-testid="sb-waiting"[^>]*>\s*Waiting for Seat 4 · Bot 4 \(bot\)…/);
    expect(body).not.toContain('data-testid="sb-keep"');
  });

  it('game 1 shows the rebuild headline', () => {
    const body = ssr(row({ round: 2, a: 0, b: 2, game: 1, games: [], chooser: null, waiting: [0, 2], score: [0, 0], table: undefined }));
    expect(body).toContain('Round 2: before your match, keep your deck or rebuild it from your pool');
    expect(body).not.toContain('Game 1 of 3');
  });
});

describe('BetweenGames (browser)', () => {
  let browser: Browser;
  beforeAll(async () => {
    browser = await sharedBrowser();
  });

  type W = { sideboardBodies: unknown[]; sideboardStatus: number; staleCalls: number; adopted: unknown[] };
  async function open(query = ''): Promise<Page> {
    const page = await browser.newPage();
    await page.goto(`${browserURL}src/components/event/BetweenGames.fixture.html${query}`);
    await page.getByTestId('sb-keep').waitFor();
    await expect.poll(() => page.getByTestId('sb-check').textContent()).toContain('40 cards');
    return page;
  }
  const read = (page: Page) => page.evaluate(() => {
    const w = window as unknown as W;
    return { bodies: w.sideboardBodies, stale: w.staleCalls, adopted: w.adopted.length };
  });

  it('Keep sends {game, keep: true} and shows the wait for the opponent', async () => {
    const page = await open();
    await page.getByTestId('sb-keep').click();
    await page.getByTestId('sb-waiting').waitFor();
    const got = await read(page);
    expect(await page.getByTestId('sb-waiting').textContent()).toContain('Waiting for Seat 4 · Bot 4');
    await page.close();
    expect(got.bodies).toEqual([{ game: 2, keep: true }]);
    expect(got.adopted).toBe(1);
  });

  it('the game-1 rebuild window keeps the deck with {game: 1}', async () => {
    const page = await open('?game=1');
    expect(await page.getByTestId('between-games').textContent()).toContain('Round 2: before your match, keep your deck or rebuild it from your pool');
    await page.getByTestId('sb-keep').click();
    await page.getByTestId('sb-waiting').waitFor();
    const got = await read(page);
    await page.close();
    expect(got.bodies).toEqual([{ game: 1, keep: true }]);
  });

  it('a swap (tap a card each way, one more basic) submits {game, deck} with the moved ids', async () => {
    const page = await open();
    expect(await page.getByTestId('sb-submit').isDisabled()).toBe(true); // nothing changed yet: Keep is the answer
    await page.locator('[data-zone="deck"][data-id="c2"]').click();
    await page.locator('[data-zone="side"][data-id="c3"]').click();
    await page.getByRole('button', { name: 'more Forest' }).click();
    await expect.poll(() => page.getByTestId('sb-check').textContent()).toContain('41 cards');
    await page.getByTestId('sb-submit').click();
    await page.getByTestId('sb-waiting').waitFor();
    const got = await read(page);
    await page.close();
    expect(got.bodies).toEqual([{ game: 2, deck: { main: ['c1', 'c3'], basics: [0, 0, 0, 0, 39] } }]);
  });

  it('a 409 refetches the state and the pool; a 422 shows the server\'s message', async () => {
    const page = await open();
    await page.evaluate(() => ((window as unknown as W).sideboardStatus = 409));
    await page.getByTestId('sb-keep').click();
    await expect.poll(async () => (await read(page)).stale).toBe(1);
    await expect.poll(() => page.getByRole('status').first().textContent()).toMatch(/moved on/);

    await page.evaluate(() => ((window as unknown as W).sideboardStatus = 422));
    await page.locator('[data-zone="side"][data-id="c3"]').click();
    await expect.poll(() => page.getByTestId('sb-check').textContent()).toContain('41 cards');
    await page.getByTestId('sb-submit').click();
    await expect.poll(() => page.getByTestId('sb-error').textContent()).toBe('build: Lightning Bolt is not in your pool');
    expect(await page.getByTestId('sb-keep').isVisible()).toBe(true); // still between games: the panel stays
    await page.close();
  });

  it('"Open full builder" edits the same working copy in a sheet with the same buttons', async () => {
    const page = await open();
    await page.getByTestId('sb-full').click();
    const sheet = page.getByTestId('sb-sheet');
    await sheet.waitFor();
    expect(await sheet.getByTestId('zone-deck').isVisible()).toBe(true);
    expect(await sheet.getByTestId('ready').count()).toBe(0); // no BUILD Ready here
    await sheet.getByRole('button', { name: 'more Forest' }).click();
    await sheet.getByRole('button', { name: 'Close' }).click();
    expect(await page.getByTestId('sb-basic-G').textContent()).toBe('39');
    await page.close();
  });
});
