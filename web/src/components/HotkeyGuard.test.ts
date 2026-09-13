import { chromium } from 'playwright';
import { createServer } from 'vite';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';

/**
 * HotkeyGuard.test.ts is the mounted half of the modal-picker guard (prio3
 * review r2 through sol2): the pure grammar test passes a synthetic picker
 * predicate, so it cannot catch a modal surface that markup failed to expose.
 * Here every transient picker kind found by the structural audit is mounted
 * over the real HotButtonStrip/SeatPanel wiring: OptionPicker's radial and
 * long-list branches, HandFan's separate menu, and PileModal's dialog. The
 * feedback dialog is covered too. Keyboard presses target the live document,
 * with focus deliberately moved off native controls where that matters.
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
  it('Space and Enter stay behind PileModal; Escape closes it and preserves End Turn', async () => {
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

    // Arm without a pointer so the open surface stays present. Escape closes
    // PileModal's own layer while both capture listeners still see the dialog
    // and therefore leave the underlying run untouched.
    await page.evaluate(() => (window as unknown as { __armRun: () => void }).__armRun());
    await page.waitForFunction(() => (window as unknown as { __posts: Posts }).__posts.length === 1);
    expect(await playMode(page)).toBe('end-turn');
    await page.keyboard.press('Escape');
    await page.waitForSelector('[data-pile-modal]', { state: 'detached' });
    expect(await posts(page)).toEqual([{ seq: 7, player: 0, choices: [9] }]);
    expect(await playMode(page)).toBe('end-turn');
    await page.close();
  });

  it('Escape with the radial picker open closes the picker and the End Turn run survives; the next Escape cancels the run', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/HotkeyGuard.fixture.html`);

    // Open the radial FIRST (a pointer on it would cancel a live run — the
    // panel's own, correct pointerdown behaviour), then arm the run
    // programmatically: no pointer is involved in arming.
    await page.locator('#radial .badge').click();
    await page.waitForSelector('[data-option-picker][data-radial-picker]');
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    await page.keyboard.press('Space');
    await page.keyboard.press('Enter');
    await page.waitForTimeout(50);
    expect(await posts(page)).toEqual([]);
    expect(await playMode(page)).toBe('custom');

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

  it('Space, Enter and Escape stay behind the open seven-option list picker', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/HotkeyGuard.fixture.html`);

    await page.locator('#menu .badge').click();
    await page.waitForSelector('.menu-pop[data-option-picker]');
    // Deliberately remove button focus. Otherwise the generic interactive-
    // target guard would hide a missing modal marker — exactly the Safari /
    // programmatic-open failure this regression is meant to expose.
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());

    await page.keyboard.press('Space');
    await page.keyboard.press('Enter');
    await page.waitForTimeout(50);
    expect(await posts(page)).toEqual([]);
    expect(await playMode(page)).toBe('custom');

    // Escape must close only the picker, not the End Turn run beneath it.
    // Arming is programmatic so no pointerdown cancels the run first.
    await page.evaluate(() => (window as unknown as { __armRun: () => void }).__armRun());
    await page.waitForFunction(() => (window as unknown as { __posts: Posts }).__posts.length === 1);
    expect(await playMode(page)).toBe('end-turn');
    await page.keyboard.press('Escape');
    await page.waitForSelector('.menu-pop[data-option-picker]', { state: 'detached' });
    expect(await posts(page)).toEqual([{ seq: 7, player: 0, choices: [9] }]);
    expect(await playMode(page)).toBe('end-turn');
    await page.close();
  });

  it('Space and Enter stay behind HandFan role=menu; Escape closes it and preserves End Turn', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/HotkeyGuard.fixture.html`);

    await page.locator('#hand .badge').click();
    await page.waitForSelector('#hand [role="menu"]');
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    await page.keyboard.press('Space');
    await page.keyboard.press('Enter');
    await page.waitForTimeout(50);
    expect(await posts(page)).toEqual([]);
    expect(await playMode(page)).toBe('custom');

    await page.evaluate(() => (window as unknown as { __armRun: () => void }).__armRun());
    await page.waitForFunction(() => (window as unknown as { __posts: Posts }).__posts.length === 1);
    await page.keyboard.press('Escape');
    await page.waitForSelector('#hand [role="menu"]', { state: 'detached' });
    expect(await posts(page)).toEqual([{ seq: 7, player: 0, choices: [9] }]);
    expect(await playMode(page)).toBe('end-turn');
    await page.close();
  });

  it('the feedback dialog owns Escape and preserves an underlying End Turn run', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/HotkeyGuard.fixture.html`);

    await page.locator('#feedback .feedback-badge').click();
    await page.waitForSelector('#feedback [role="dialog"]');
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    await page.keyboard.press('Space');
    await page.keyboard.press('Enter');
    await page.waitForTimeout(50);
    expect(await posts(page)).toEqual([]);

    await page.evaluate(() => (window as unknown as { __armRun: () => void }).__armRun());
    await page.waitForFunction(() => (window as unknown as { __posts: Posts }).__posts.length === 1);
    await page.keyboard.press('Escape');
    await page.waitForSelector('#feedback [role="dialog"]', { state: 'detached' });
    expect(await posts(page)).toEqual([{ seq: 7, player: 0, choices: [9] }]);
    expect(await playMode(page)).toBe('end-turn');
    await page.close();
  });

  it('modalPickerOpen structurally detects a bare role=menu with no data marker', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/HotkeyGuard.fixture.html`);

    const result = await page.evaluate(() => {
      const menu = document.createElement('div');
      menu.setAttribute('role', 'menu');
      document.body.append(menu);
      const open = (window as unknown as { __modalPickerOpen: () => boolean }).__modalPickerOpen();
      menu.remove();
      const closed = (window as unknown as { __modalPickerOpen: () => boolean }).__modalPickerOpen();
      return { open, closed };
    });
    expect(result).toEqual({ open: true, closed: false });
    await page.close();
  });
});
