import { chromium } from 'playwright';
import { createServer } from 'vite';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';

/**
 * HotkeyGuard.test.ts is the mounted half of the modal-picker guard (prio3
 * review r2, findings 2 and 3): the pure grammar test passes a synthetic
 * picker predicate, so it cannot catch a modal surface that forgot its
 * marker. Here the REAL surfaces are open — a PileModal (hand/graveyard/
 * exile dialog, portaled to body) and a CardTile's radial picker — over the
 * real HotButtonStrip/SeatPanel wiring, and keyboard presses are driven at
 * the live document with focus exactly where the review's scenario puts it
 * (the non-interactive dialog).
 */

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

type Posts = { seq: number; player: number; choices: number[] }[];

const posts = (page: import('playwright').Page): Promise<Posts> =>
  page.evaluate(() => (window as unknown as { __posts: Posts }).__posts);

const playMode = (page: import('playwright').Page): Promise<string> =>
  page.evaluate(() => document.querySelector('[data-play-mode]')?.getAttribute('data-play-mode') ?? '');

describe('the hotkey guard against an open modal — mounted', () => {
  it('Space and Enter with a PileModal open (focus in the dialog) do nothing; closed, Space passes and Enter arms End Turn', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/HotkeyGuard.fixture.html`);

    await page.evaluate(() => (window as unknown as { __openPile: () => void }).__openPile());
    await page.waitForSelector('[data-pile-modal]');
    // The review's exact scenario: focus rests on the non-interactive dialog.
    await page.waitForFunction(() => document.activeElement?.getAttribute('role') === 'dialog');

    await page.keyboard.press('Space');
    await page.keyboard.press('Enter');
    expect(await posts(page)).toEqual([]);
    expect(await playMode(page)).toBe('custom'); // no run armed under the modal

    // Escape closes the PileModal itself (its own grammar, still live).
    await page.keyboard.press('Escape');
    await page.waitForSelector('[data-pile-modal]', { state: 'detached' });
    expect(await posts(page)).toEqual([]);
    expect(await playMode(page)).toBe('custom');

    // Positive controls, same keys, modal closed: Space passes once...
    await page.keyboard.press('Space');
    await page.waitForFunction(() => (window as unknown as { __posts: Posts }).__posts.length > 0);
    expect(await posts(page)).toEqual([{ seq: 7, player: 0, choices: [9] }]);
    // ...and with a fresh answerable window, Enter arms the End Turn run.
    await page.evaluate(() => (window as unknown as { __newDecision: () => void }).__newDecision());
    await page.keyboard.press('Enter');
    await page.waitForFunction(() => document.querySelector('[data-play-mode]')?.getAttribute('data-play-mode') === 'end-turn');
    await page.close();
  });

  it('Escape with the radial picker open closes the picker and the End Turn run survives; the next Escape cancels the run', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/HotkeyGuard.fixture.html`);

    // Open the radial FIRST (a pointer on it would cancel a live run — the
    // panel's own, correct pointerdown behaviour), then arm the run
    // programmatically: no pointer is involved in arming.
    await page.locator('#radial .badge').click();
    await page.waitForSelector('[data-radial-picker]');
    await page.evaluate(() => (window as unknown as { __armRun: () => void }).__armRun());
    await page.waitForFunction(() => document.querySelector('[data-play-mode]')?.getAttribute('data-play-mode') === 'end-turn');
    const before = await posts(page);
    expect(before).toEqual([{ seq: 7, player: 0, choices: [9] }]);

    // The review's break: SeatPanel's own window listener used to cancel the
    // run here, past the grammar's guard. Now the modal closes and the run
    // survives — exactly one Escape per layer.
    await page.keyboard.press('Escape');
    await page.waitForSelector('[data-radial-picker]', { state: 'detached' });
    expect(await posts(page)).toEqual(before); // no further pass was posted
    expect(await playMode(page)).toBe('end-turn');

    // The picker is closed, so Escape is the panic key again.
    await page.keyboard.press('Escape');
    await page.waitForFunction(() => document.querySelector('[data-play-mode]')?.getAttribute('data-play-mode') === 'custom');
    expect(await posts(page)).toEqual(before);
    await page.close();
  });
});
