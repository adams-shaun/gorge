import { chromium } from 'playwright';
import { createServer } from 'vite';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';

/**
 * CardMenu.test.ts is the mounted half of the Ctrl-hold-priority requirement
 * (prio3, r2 review finding 1): a Ctrl-click on a card-menu cast/ability must
 * reach SeatPanelState.click's `{ holdPriority: true }` through the tile post
 * path — the pure helpers are tested in cardoptions.test.ts, but the thread
 * from a REAL click through CardTile → OptionPicker → TileOptions.post is
 * what the r2 review broke by code trace, so it is held here by a mounted
 * run against the fixture.
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

type Posted = [number, boolean, boolean][];

const posted = (page: import('playwright').Page): Promise<Posted> =>
  page.evaluate(() => (window as unknown as { __posted: Posted }).__posted);

describe('Ctrl held on a card-menu cast holds priority (the tile post thread)', () => {
  it('a plain radial click posts without hold-priority; a Ctrl-click posts holdPriority', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/CardMenu.fixture.html`);
    // Open the radial by a real badge click (seeding it open would skip the
    // anchor capture and clamp every wheel button onto (8,8)); the picker
    // closes on every choose, so each click re-opens it first.
    await page.locator('#radial .badge').click();
    const wheel = page.locator('body > [data-radial-picker]');
    await wheel.locator('button[data-wire-index="3"]').waitFor();

    await wheel.locator('button[data-wire-index="3"]').click();
    expect(await posted(page)).toEqual([[3, false, false]]);

    await page.locator('#radial .badge').click();
    await wheel.locator('button[data-wire-index="8"]').waitFor();
    await wheel.locator('button[data-wire-index="8"]').click({ modifiers: ['Control'] });
    expect(await posted(page)).toEqual([[3, false, false], [8, false, true]]);
    await page.close();
  });

  it('Ctrl-clicking the single-action cast icon holds priority too; a plain click does not', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/CardMenu.fixture.html`);

    await page.locator('#single [data-single-action]').click({ modifiers: ['Control'] });
    expect(await posted(page)).toEqual([[21, true, true]]);

    await page.locator('#single [data-single-action]').click();
    expect(await posted(page)).toEqual([[21, true, true], [21, true, false]]);
    await page.close();
  });
});
