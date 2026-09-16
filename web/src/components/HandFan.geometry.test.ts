import { type Browser } from 'playwright';
import { browserURL, sharedBrowser } from '../test/browser';
import { beforeAll, describe, expect, it } from 'vitest';

let browser: Browser;
let url = '';

beforeAll(async () => {
  url = browserURL;
  browser = await sharedBrowser();
});

// The stage is fixed 600x420 (see HandFan.geometry.html), smaller than every
// supported test viewport, so the geometry measured is the stage's clip, not
// the viewport's.
const VIEWPORTS = [
  { width: 1440, height: 900 },
  { width: 1000, height: 900 },
  { width: 650, height: 700 },
] as const;

interface Rect {
  top: number; bottom: number; left: number; right: number; width: number; height: number;
}

interface Measure {
  stage: Rect;
  card: Rect;
  /** the stage's scroll position: the board is overflow:hidden, which is
   *  programmatically scrollable — a focused half-clipped face would scroll
   *  it to "reveal" the clipped half (the felt jumps). Pinned at 0. */
  stageScrollTop: number;
}

// Tolerance: subpixel rounding across the transform chain (fan shift, card
// raise) plus the transition having settled. 2px is generous to rounding and
// still far below the half-card (~72px) the assertions discriminate on.
const TOL = 2;

function intersectH(a: Rect, b: Rect): number {
  return Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
}

// Wait until the card's raise TRANSITION has actually settled at the expected
// raised top (the .card transition is 0.12s, but under a loaded runner a
// fixed sleep races it — measured 32px of 73.6 mid-flight). A timeout throws
// with the live geometry, transform and focus state, so a genuine failure is
// diagnosable rather than a bare timeout.
async function settleRaised(
  page: Awaited<ReturnType<typeof browser.newPage>>,
  raisedTop: number,
): Promise<void> {
  try {
    await page.waitForFunction(
      (exp) => {
        const c = document.querySelector('[data-obj="1"]');
        if (!c) return false;
        return Math.abs(c.getBoundingClientRect().top - exp) <= 1;
      },
      raisedTop,
      { timeout: 10_000, polling: 50 },
    );
  } catch {
    const diag = await page.evaluate(() => {
      const c = document.querySelector('[data-obj="1"]');
      const face = c?.querySelector('.face');
      return {
        top: c?.getBoundingClientRect().top ?? null,
        transform: c ? getComputedStyle(c).transform : null,
        activeObj: document.activeElement?.closest('[data-obj]')?.getAttribute('data-obj') ?? null,
        faceFocusVisible: face?.matches(':focus-visible') ?? null,
      };
    });
    throw new Error(`raise never settled at ${raisedTop}: ${JSON.stringify(diag)}`);
  }
}

// Land KEYBOARD focus on a given card's face before measuring the focus
// raise. Tab is retried only after an explicit blur, so a slow first Tab
// cannot be double-stepped past the target face; under a loaded runner the
// keypress can otherwise land while the raise settle is already waiting.
async function focusFaceByTab(
  page: Awaited<ReturnType<typeof browser.newPage>>,
  obj: string,
): Promise<void> {
  for (let attempt = 0; attempt < 3; attempt++) {
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur?.());
    await page.keyboard.press('Tab');
    try {
      await page.waitForFunction(
        (o) => document.activeElement?.closest('[data-obj]')?.getAttribute('data-obj') === o,
        obj,
        { timeout: 2_000, polling: 25 },
      );
      return;
    } catch {
      /* retry from a clean slate */
    }
  }
  throw new Error(`keyboard focus never landed on card ${obj}`);
}

async function measure(page: Awaited<ReturnType<typeof browser.newPage>>): Promise<Measure> {
  return page.evaluate(() => {
    const rect = (el: Element): Rect => {
      const b = el.getBoundingClientRect();
      return { top: b.top, bottom: b.bottom, left: b.left, right: b.right, width: b.width, height: b.height };
    };
    const stage = document.querySelector('#stage')!;
    // The FIRST card: its right-hand band is covered by card 2 at rest (later
    // siblings paint above), so only the hover raise's z-index can put it
    // back on top there — the z-order assertion discriminates.
    const card = document.querySelector('[data-obj="1"]')!;
    return { stage: rect(stage), card: rect(card), stageScrollTop: stage.scrollTop };
  });
}

describe('HandFan — the hand-card peek (fb-20260916T024357Z-9005ad6a)', () => {
  it('at rest exactly the top half of a hand card is visible inside the clipped board; hover and keyboard focus each raise it fully inside, above its neighbours', { timeout: 20_000 }, async () => {
    for (const { width, height } of VIEWPORTS) {
      const page = await browser.newPage({ viewport: { width, height } });
      await page.goto(`${url}src/components/HandFan.geometry.html`);
      await page.waitForSelector('[data-obj="1"] .face', { timeout: 5000 });
      // Let the layout (ResizeObserver -> measured) and paints settle.
      await page.waitForTimeout(120);

      const atRest = await measure(page);
      const h = atRest.card.height;
      const stage = atRest.stage;

      // Sanity: the fixture really is a 63:88 card inside the stage.
      expect(h).toBeGreaterThan(50);

      // 1. AT REST — the peek. The card's lower half lies outside the board
      //    (the rect genuinely extends below the clip edge — getBoundingClientRect
      //    reports the layout box, the board's overflow: hidden is what cuts
      //    it), and the intersection of card and board is exactly half the
      //    card's height. On the unshifted fan the intersection is the whole
      //    card, so this fails both on the pre-fix code and with the shift
      //    removed.
      expect(atRest.card.bottom).toBeGreaterThan(stage.bottom - TOL);
      expect(atRest.card.bottom).toBeLessThanOrEqual(stage.bottom + h / 2 + TOL);
      expect(Math.abs(intersectH(atRest.card, stage) - h / 2)).toBeLessThanOrEqual(TOL);
      expect(atRest.card.top).toBeGreaterThanOrEqual(stage.top - TOL);
      expect(atRest.card.left).toBeGreaterThanOrEqual(stage.left - TOL);

      // 2. The rest-state z-order baseline: in the band card 1 shares with
      //    card 2, card 2 (later sibling) is the hit — card 1 is UNDER its
      //    neighbour at rest.
      const band = { x: atRest.card.left + atRest.card.width * 0.9, y: atRest.card.top + h * 0.25 };
      expect(band.x).toBeGreaterThan(atRest.card.left + atRest.card.width / 2);
      const hitAt = (x: number, y: number) =>
        page.evaluate(([px, py]) => {
          const el = document.elementFromPoint(px, py);
          return el?.closest('[data-obj]')?.getAttribute('data-obj') ?? null;
        }, [x, y]);
      expect(band.y).toBeLessThan(stage.bottom); // inside the visible half
      expect(await hitAt(band.x, band.y)).toBe('2');

      // 3. HOVER the VISIBLE face — a point only card 1 covers (its left
      //    region, clear of card 2's overlap), in the visible top half. The
      //    raise must be exactly the half the fan was lowered, the full rect
      //    must lie inside the board, and the card must come back above its
      //    neighbour in the shared band.
      await page.mouse.move(atRest.card.left + atRest.card.width * 0.35, atRest.card.top + h * 0.25);
      await settleRaised(page, atRest.card.top - h / 2);
      const hovered = await measure(page);
      expect(Math.abs(hovered.card.top - (atRest.card.top - h / 2))).toBeLessThanOrEqual(TOL);
      expect(hovered.card.bottom).toBeLessThanOrEqual(stage.bottom + TOL);
      expect(hovered.card.top).toBeGreaterThanOrEqual(stage.top - TOL);
      expect(hovered.card.left).toBeGreaterThanOrEqual(stage.left - TOL);
      expect(hovered.card.right).toBeLessThanOrEqual(stage.right + TOL);
      // Above the neighbour again: the shared band now hits card 1.
      expect(await hitAt(band.x, band.y)).toBe('1');

      // 4. KEYBOARD FOCUS — same full-card geometry. Move the pointer well
      //    clear of the fan first (so :hover is off and ONLY the focus rule
      //    can raise the card), then Tab into the fan: its faces are the
      //    page's only focusables, so the first Tab lands on card 1's face.
      await page.mouse.move(5, 5);
      await focusFaceByTab(page, '1');
      await settleRaised(page, atRest.card.top - h / 2);
      const focused = await measure(page);
      // The focus reveal must NOT have scrolled the clipped board to uncover
      // the resting face's lower half — the felt would jump by up to half a
      // card (measured 73-75px before the negative scroll-margin fix).
      expect(focused.stageScrollTop).toBe(0);
      const active = await page.evaluate(() => ({
        tag: document.activeElement?.tagName ?? null,
        obj: document.activeElement?.closest('[data-obj]')?.getAttribute('data-obj') ?? null,
      }));
      expect(active.obj).toBe('1');
      expect(active.tag).toBe('DIV');
      expect(Math.abs(focused.card.top - (atRest.card.top - h / 2))).toBeLessThanOrEqual(TOL);
      expect(focused.card.bottom).toBeLessThanOrEqual(stage.bottom + TOL);
      expect(focused.card.top).toBeGreaterThanOrEqual(stage.top - TOL);
      // The focused card is above its neighbour too (the same z-index rides
      // the :focus-visible rule).
      expect(await hitAt(band.x, band.y)).toBe('1');

      await page.close();
    }
  });

  it('a half-hidden hand card still receives the pointer on its visible half and the clip leaves the neighbour faces alone', async () => {
    // The pointer-affordance half of the contract: a card's VISIBLE top half
    // is what the pointer hits (the clipped lower half is not hit-testable),
    // and the rest-state hit in each card's exclusive region is that card —
    // so entering the visible face, seeing it rise, and using its actions
    // all key off geometry that actually exists. Measured through
    // elementFromPoint, without moving the pointer.
    for (const { width, height } of VIEWPORTS) {
      const page = await browser.newPage({ viewport: { width, height } });
      await page.goto(`${url}src/components/HandFan.geometry.html`);
      await page.waitForSelector('[data-obj="1"] .face', { timeout: 5000 });
      await page.waitForTimeout(120);

      const m = await measure(page);
      const h = m.card.height;
      // A point in card 1's visible half that only card 1 covers.
      const probe = { x: m.card.left + m.card.width * 0.35, y: m.card.top + h * 0.25 };
      const hit = await page.evaluate(([px, py]) => {
        const el = document.elementFromPoint(px, py);
        return el?.closest('[data-obj]')?.getAttribute('data-obj') ?? null;
      }, [probe.x, probe.y]);
      expect(hit).toBe('1');
      // And a point clearly below the clip edge (the hidden half's zone) hits
      // nothing inside the fan — the stage's overflow clips it away.
      const hidden = await page.evaluate(([px, py]) => {
        const el = document.elementFromPoint(px, py);
        return el?.closest('[data-obj]')?.getAttribute('data-obj') ?? null;
      }, [probe.x, m.stage.bottom + 10]);
      expect(hidden).toBeNull();

      await page.close();
    }
  });
});
