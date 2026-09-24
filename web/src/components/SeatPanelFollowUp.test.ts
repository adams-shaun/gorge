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
 *
 * The second case is the L2 finding (verdict-tm1): the follow-up decision can
 * arrive through SSE BEFORE the POST response arms the expectation. The
 * fixture runs Table.svelte's decode effect reactively, so arming must
 * retrigger decoding (followUpRevision) for the wheel to open in that order.
 */

let browser: Browser;
let url = '';

declare global {
  interface Window {
    __click: (index: number) => void;
    __deliverFollowUp: () => void;
    __armState: () => { seq: number; obj: number } | null;
    __autoOpen: () => { seq: number; obj: number } | null;
    __revision: () => number;
  }
}

beforeAll(async () => {
  url = browserURL;
  browser = await sharedBrowser();
});

type Arm = { seq: number; obj: number } | null;

describe('a panel-answered treasure activation raises the radial colour wheel (fb-20260923T050205Z)', () => {
  it('panel click 1 arms obj 207, decodes the 5-colour ask, and the wheel opens with five pips', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/SeatPanelFollowUp.fixture.html`);

    // PRECONDITION: nothing is armed and nothing decoded before the click, so
    // the assertions below cannot pass on a fixture that pre-seeded either.
    expect(await page.evaluate(() => window.__armState())).toBeNull();
    expect(await page.evaluate(() => window.__autoOpen())).toBeNull();

    await page.locator('#panel-option-1').click();
    await page.waitForFunction(() => window.__armState() !== null);
    const armed = await page.evaluate((): Arm => window.__armState());
    // The armed expectation is the answered option's OWN obj (207), never the
    // decision source or a list position — the sibling option is obj 205.
    expect(armed).toEqual({ seq: 688, obj: 207 });

    // The ordinary network order: the follow-up decision arrives after the
    // arm. The reactive effect decodes it and the tile autoOpen opens the
    // wheel. (The race order is the next test.)
    await page.evaluate(() => window.__deliverFollowUp());
    await page.waitForFunction(() => window.__autoOpen() !== null);
    expect(await page.evaluate((): Arm => window.__autoOpen())).toEqual({ seq: 693, obj: 207 });
    await expectTheWheel(page);
    await page.close();
  });

  it('opens the wheel when the follow-up decision arrives BEFORE the post response arms (the L2 race)', async () => {
    const page = await browser.newPage();
    await page.goto(`${url}src/components/SeatPanelFollowUp.fixture.html`);

    // PRECONDITION: nothing armed yet, so the arm below is the only source.
    expect(await page.evaluate(() => window.__armState())).toBeNull();

    // SSE-first: the 5-colour ask is already the arriving decision before the
    // panel click posts. The effect's first run sees no expectation and must
    // be inert (nothing opens on an unarmed decision).
    await page.evaluate(() => window.__deliverFollowUp());
    await page.waitForFunction(() => window.__autoOpen() === null);
    expect(await page.evaluate((): Arm => window.__autoOpen())).toBeNull();

    // Now the click: the post's response arms the expectation. Arming must
    // retrigger the decode even though the decision did not change, or the
    // wheel never opens — this is exactly the L2 break. The arm itself is
    // transient here (the effect decodes and clears it in the same turn), so
    // the observable is followUpRevision reaching 1; the arm did happen.
    expect(await page.evaluate(() => window.__revision())).toBe(0);
    await page.locator('#panel-option-1').click();
    await page.waitForFunction(() => window.__autoOpen() !== null);
    // PRECONDITION: the click really armed (revision moved 0 -> 1); the
    // decode below is the retrigger under test, not a pre-seeded open.
    expect(await page.evaluate(() => window.__revision())).toBe(1);
    expect(await page.evaluate((): Arm => window.__autoOpen())).toEqual({ seq: 693, obj: 207 });
    await expectTheWheel(page);
    await page.close();
  });
});

async function expectTheWheel(page: Awaited<ReturnType<Browser['newPage']>>): Promise<void> {
  // The decode hands the tile autoOpen, so the radial picker mounts, and all
  // five options are mana pips — not the panel's plain button list.
  const wheel = page.locator('body > [data-radial-picker]');
  await wheel.locator('button[data-mana-option]').first().waitFor();
  expect(await wheel.count()).toBe(1);
  expect(await wheel.locator('button[data-mana-option]').all()).toHaveLength(5);
}
