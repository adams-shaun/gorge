import { type Browser } from 'playwright';
import { browserURL, sharedBrowser } from '../test/browser';
import { beforeAll, describe, expect, it } from 'vitest';

/**
 * CardMenu.test.ts is the mounted half of the Ctrl-hold-priority requirement
 * (prio3, r2 review finding 1): a Ctrl-click on a card-menu cast/ability must
 * reach SeatPanelState.click's `{ holdPriority: true }` through the tile post
 * path — the pure helpers are tested in cardoptions.test.ts, but the thread
 * from a REAL click through CardTile → OptionPicker → TileOptions.post is
 * what the r2 review broke by code trace, so it is held here by a mounted
 * run against the fixture.
 */

let browser: Browser;
let url = '';

beforeAll(async () => {
  url = browserURL;
  browser = await sharedBrowser();
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
    // fb-e079def5: every picker post arms the card-follow-up expectation
    // (the Talisman stage-1 → stage-2 colour wheel), so expectFollowUp is
    // true for radial clicks too, not just the single-action badge.
    expect(await posted(page)).toEqual([[3, true, false]]);

    await page.locator('#radial .badge').click();
    await wheel.locator('button[data-wire-index="8"]').waitFor();
    await wheel.locator('button[data-wire-index="8"]').click({ modifiers: ['Control'] });
    expect(await posted(page)).toEqual([[3, true, false], [8, true, true]]);
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

  // fb-e079def5: the reported Talisman of Indulgence flow — stage 1 answered
  // through the radial wheel must arm the follow-up, and the stage-2 colour
  // wheel must re-open at the card instead of falling back to the seat
  // panel's generic option list.
  it('a picker-answered stage-1 arms the follow-up and the stage-2 colour wheel opens at the card', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/CardMenu.fixture.html`);
    await page.locator('#followup .badge').click();
    const wheel = page.locator('body > [data-radial-picker]');
    await wheel.locator('button[data-wire-index="6"]').waitFor(); // the Add B or R option
    await wheel.locator('button[data-wire-index="6"]').click();

    // Stage 2 arrives on the next "frame" (a macrotask here): the colour
    // wheel must re-open at the card, pip-tinted (both options match the
    // Add <C> shape), never only the panel's generic list.
    const stageTwoWheel = page.locator('body > [data-radial-picker]');
    await stageTwoWheel.locator('button[data-wire-index="0"]').waitFor();
    await stageTwoWheel.locator('button[data-wire-index="1"]').waitFor();
    expect(await stageTwoWheel.locator('button[data-mana-option]').all()).toHaveLength(2);
    expect(await posted(page)).toEqual([[6, true, false]]);
    await page.close();
  });
});
