import { type Browser } from 'playwright';
import { browserURL, sharedBrowser } from '../test/browser';
import { beforeAll, describe, expect, it } from 'vitest';

/**
 * SeatPanelFollowUp.test.ts is the mounted proof for fb-20260923T050205Z: the
 * treasure's colour ask opens the radial mana wheel when the activation was
 * posted from the SEAT PANEL's plain option button — not only from a card
 * tile. The fixture posts through the real SeatPanelState.click, so this test
 * exercises the arm site the fix moved; CardMenu.test.ts's fb-e079def5 case
 * still covers the tile surface.
 */

let browser: Browser;
let url = '';

beforeAll(async () => {
  url = browserURL;
  browser = await sharedBrowser();
});

type Arm = { seq: number; obj: number } | null;

describe('a panel-answered treasure activation raises the radial colour wheel (fb-20260923T050205Z)', () => {
  it('panel click 1 arms obj 207, decodes the 5-colour ask, and the wheel opens with five pips', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/SeatPanelFollowUp.fixture.html`);

    // PRECONDITION: nothing is armed before the click, so the assertion
    // below cannot pass on a fixture that pre-seeded the expectation.
    expect(await page.evaluate(() => window.__armState())).toBeNull();

    // Model the race: the follow-up decision is already available before the
    // POST response arms the expectation. Its first decode must be inert;
    // arming then triggers the second decode (Table tracks followUpRevision).
    expect(await page.evaluate(() => window.__decode())).toBeNull();
    await page.locator('#panel-option-1').click();
    await page.waitForFunction(() => window.__armState() !== null);
    const armed = await page.evaluate((): Arm => window.__armState());
    // The armed expectation is the answered option's OWN obj (207), never the
    // decision source or a list position — the sibling option is obj 205.
    expect(armed).toEqual({ seq: 688, obj: 207 });

    // Table.svelte's effect body: decode the next decision against the arm.
    const open = await page.evaluate((): Arm => window.__decode());
    expect(open).toEqual({ seq: 693, obj: 207 });

    // The decode hands the tile autoOpen, so the radial picker mounts, and
    // all five options are mana pips — not the panel's plain button list.
    const wheel = page.locator('body > [data-radial-picker]');
    await wheel.locator('button[data-mana-option]').first().waitFor();
    expect(await wheel.count()).toBe(1);
    expect(await wheel.locator('button[data-mana-option]').all()).toHaveLength(5);
    await page.close();
  });
});
