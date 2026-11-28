import { type Browser, type Page } from 'playwright';
import { beforeAll, describe, expect, it } from 'vitest';
import { browserURL, sharedBrowser } from '../test/browser';

/**
 * PassiveHandFan.faceShape.test.ts is the font-agnostic proof that a
 * spectator's passive hand face is a CARD, whatever font the machine resolves:
 * every face box measures 63/88, the printed card's shape.
 *
 * A face's box is sized by arithmetic (width var(--card-w), aspect-ratio
 * 63/88), but a box is only that shape while nothing else has a say about its
 * height. The card's image is an INLINE-level box, so the row of text it sits
 * in reserves room for that row's font DESCENDER below it — the strut — and
 * the face grows by it: 2px with this machine's fallback, 3px on a GitHub
 * runner's (DejaVu Sans). On a 68px full-panel face that is noise; on a
 * focus8 side strip's 19x27px face it is 10%, and the face stops reading as a
 * card (the CI failure this file pins shut). So the invariant is measured
 * under several font families, and each one first has to prove it actually
 * puts a descender under a bare inline row — a family that did not would let
 * every assertion below pass without ever exercising the defect.
 */

let browser: Browser;

beforeAll(async () => {
  browser = await sharedBrowser();
});

/**
 * The families the runner may resolve: the generics always exist as an
 * alias, and the two named ones are this app's declared stack and the Ubuntu
 * runner's default. Every one of them resolves to SOME font, so the shape is
 * pinned against each of their descenders.
 */
const FAMILIES = ['"IBM Plex Sans"', '"DejaVu Sans"', 'serif', 'monospace'];

/** A strip face at 1440x900 focus8 is 19px wide; 26px is its ratio height. */
const PROBE_W = 19;
const PROBE_H = 26;

async function open(query: string, width: number, height: number): Promise<Page> {
  const page = await browser.newPage({ viewport: { width, height } });
  await page.goto(`${browserURL}src/components/SpectatorHands.fixture.html?${query}`);
  await page.waitForFunction(() => (window as Window & { fixtureReady?: boolean }).fixtureReady === true, undefined, { timeout: 60_000 });
  await page.waitForSelector('[data-own-board] .quadrant', { state: 'attached', timeout: 60_000 });
  await page.evaluate(() => document.fonts.ready);
  return page;
}

/**
 * driveFont drives the whole document to one family and returns how many px
 * that family's font drops BELOW the baseline of a bare inline row (its
 * strut). The face's height was once measured against exactly that row, so a
 * family with no strut proves nothing and the caller says so.
 */
async function driveFont(page: Page, family: string): Promise<number> {
  return page.evaluate(async ({ fam, w, h }: { fam: string; w: number; h: number }) => {
    document.getElementById('face-shape-font')?.remove();
    const style = document.createElement('style');
    style.id = 'face-shape-font';
    style.textContent = `*, *::before, *::after { font-family: ${fam} !important }`;
    document.head.appendChild(style);
    await document.fonts.ready;
    // A row that holds only an inline-level box of the face's size: the box
    // itself is `h` tall, so whatever the row measures over that is the
    // font's descender, and it is the font's, not the card's.
    const row = document.createElement('div');
    row.style.cssText = `position:fixed;left:-9999px;top:0;width:${w}px;visibility:hidden`;
    const item = document.createElement('span');
    item.style.cssText = `display:inline-flex;width:${w}px;height:${h}px;background:red`;
    row.appendChild(item);
    document.body.appendChild(row);
    const strut = row.getBoundingClientRect().height - h;
    row.remove();
    return strut;
  }, { fam: family, w: PROBE_W, h: PROBE_H });
}

/** faceRatios measures every passive face box on the table, per seat. */
async function faceRatios(page: Page): Promise<{ seat: number; obj: number; ratio: number }[]> {
  return page.evaluate(() =>
    [...document.querySelectorAll('[data-passive-hand]')].flatMap((el) =>
      [...el.querySelectorAll('[data-hand-card]')].map((c) => {
        const b = c.getBoundingClientRect();
        return { seat: Number(el.getAttribute('data-passive-hand')), obj: Number(c.getAttribute('data-hand-card')), ratio: b.width / b.height };
      }),
    ),
  );
}

describe('a passive face keeps the card shape whatever font the machine resolves', () => {
  it('focus8 1440x900: the compacted side strip faces and the full panels alike measure 63/88 under every font', { timeout: 120_000 }, async () => {
    const page = await open('seats=8&mode=omni&preset=focus8', 1440, 900);
    try {
      for (const family of FAMILIES) {
        const strut = await driveFont(page, family);
        expect(strut, `${family}: must drop a descender below a bare inline row, or this family cannot exercise the shape`).toBeGreaterThan(0);
        const faces = await faceRatios(page);
        // The strip faces are the small ones (a full panel's face is 3.5x
        // wider), so a fan that has one is the case the tolerance is about.
        expect(faces.length, `${family}: faces measured`).toBeGreaterThan(0);
        expect(new Set(faces.map((f) => f.seat)).size, `${family}: every seat's fan measured`).toBe(8);
        for (const f of faces) {
          expect(f.ratio, `${family}: seat ${f.seat} face ${f.obj} has a card's shape`).toBeCloseTo(63 / 88, 1);
        }
      }
    } finally {
      await page.close();
    }
  });

  it('two seats 650x700: the rim bands keep the same shape under every font', { timeout: 120_000 }, async () => {
    const page = await open('seats=2&mode=omni', 650, 700);
    try {
      for (const family of FAMILIES) {
        const strut = await driveFont(page, family);
        expect(strut, `${family}: must drop a descender below a bare inline row, or this family cannot exercise the shape`).toBeGreaterThan(0);
        const faces = await faceRatios(page);
        expect(faces.length, `${family}: faces measured`).toBeGreaterThan(0);
        expect(new Set(faces.map((f) => f.seat)).size, `${family}: every seat's fan measured`).toBe(2);
        for (const f of faces) expect(f.ratio, `${family}: seat ${f.seat} face ${f.obj} has a card's shape`).toBeCloseTo(63 / 88, 1);
      }
    } finally {
      await page.close();
    }
  });
});
