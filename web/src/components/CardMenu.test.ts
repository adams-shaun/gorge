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
    // closes on every choose of an ORDINARY action, so each click re-opens
    // it first. Attacker selections (below) deliberately do not close.
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

/**
 * fb-20260923T020152Z: an attacker declaration is a SELECTION in a
 * still-pending multi-pick decision, so choosing one must post its own wire
 * index and leave the radial open for the next attacker. The two other close
 * paths the report named are held here too: an outside click and Escape must
 * still dismiss the wheel, and an ordinary immediate action must still close
 * it (the latter is the first describe block's existing contract).
 */
describe('declaring attackers keeps the radial picker open (fb-20260923T020152Z)', () => {
  it('a click on an attacker option posts its wire index and leaves the radial attached for the next attacker', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/CardMenu.fixture.html`);
    await page.locator('#attackers .badge').click();
    const wheel = page.locator('body > [data-radial-picker]');
    await wheel.locator('button[data-wire-index="40"]').waitFor();

    // PRECONDITION: the attacker wheel is attached before the pick (a
    // wheel that never opened would make the post assertion below vacuous).
    expect(await wheel.count()).toBe(1);

    await wheel.locator('button[data-wire-index="40"]').click();
    expect(await posted(page)).toEqual([[40, true, false]]);

    // The fix under test: the same wheel is still attached at the SAME
    // anchor after a successful attacker pick (no badge re-open), and the
    // next attacker posts its own wire index (R-E4-1, never list position).
    expect(await wheel.count()).toBe(1);
    await wheel.locator('button[data-wire-index="41"]').waitFor();
    await wheel.locator('button[data-wire-index="41"]').click();
    expect(await posted(page)).toEqual([[40, true, false], [41, true, false]]);

    // A third successive attacker still needs no re-open.
    expect(await wheel.count()).toBe(1);
    await wheel.locator('button[data-wire-index="42"]').click();
    expect(await posted(page)).toEqual([[40, true, false], [41, true, false], [42, true, false]]);
    await page.close();
  });

  it('an outside click still dismisses an attacker wheel', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/CardMenu.fixture.html`);
    await page.locator('#attackers .badge').click();
    const wheel = page.locator('body > [data-radial-picker]');
    await wheel.locator('button[data-wire-index="40"]').waitFor();
    expect(await wheel.count()).toBe(1);

    // The portaled radial layer is pointer-events:none, so a click at the
    // viewport corner lands on the page beneath it — an outside click.
    await page.mouse.click(4, 4);
    await wheel.waitFor({ state: 'detached' });
    expect(await posted(page)).toEqual([]);
    await page.close();
  });

  it('Escape still dismisses an attacker wheel', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/CardMenu.fixture.html`);
    await page.locator('#attackers .badge').click();
    const wheel = page.locator('body > [data-radial-picker]');
    await wheel.locator('button[data-wire-index="40"]').waitFor();
    expect(await wheel.count()).toBe(1);

    await page.keyboard.press('Escape');
    await wheel.waitFor({ state: 'detached' });
    expect(await posted(page)).toEqual([]);
    await page.close();
  });
});
