import { type Browser, type Page } from 'playwright';
import { beforeAll, describe, expect, it } from 'vitest';
import { browserURL, sharedBrowser } from '../test/browser';

/**
 * HandFan.faceShape.test.ts is the font-agnostic proof that the seated
 * player's own-hand face is a CARD, whatever font the machine resolves: every
 * `.handtrack .handfan .card` box measures 63/88, the printed card's shape.
 *
 * The card's width is var(--card-w) and its shape aspect-ratio 63/88, but a
 * box is only that shape while its content has no say about its height. The
 * face's CardImage is an INLINE-level box, so the text row it sits in reserves
 * room for that row's font DESCENDER below it (the strut), and a content-sized
 * card grows by it: 3px under a tall-descender serif. PassiveHandFan's
 * 19px-wide faces failed CI on exactly that; the own hand's 142px faces hide
 * it under the tolerance, so this file measures at a size where the strut
 * moves the ratio past it (see SMALL below) as well as at the stock desktop.
 * Each family first has to prove it really drops a descender below a bare
 * inline row, so no case can pass without exercising the defect.
 */

let browser: Browser;

beforeAll(async () => {
  browser = await sharedBrowser();
});

/** The app's declared family, the Ubuntu runner's default, and the generics. */
const FAMILIES = ['"IBM Plex Sans"', '"DejaVu Sans"', 'serif', 'monospace'];

async function open(query: string, width: number, height: number): Promise<Page> {
  const page = await browser.newPage({ viewport: { width, height } });
  await page.goto(`${browserURL}src/components/SpectatorHands.fixture.html?${query}`);
  await page.waitForFunction(() => (window as Window & { fixtureReady?: boolean }).fixtureReady === true, undefined, { timeout: 60_000 });
  await page.waitForSelector('.handtrack .handfan .card', { state: 'attached', timeout: 60_000 });
  await page.evaluate(() => document.fonts.ready);
  return page;
}

/**
 * driveFont drives the whole document to one family and returns how many px
 * that family drops BELOW the baseline of a bare inline row the size of a
 * hand face (its strut). A family with no strut proves nothing.
 */
async function driveFont(page: Page, family: string): Promise<number> {
  return page.evaluate(async (fam: string) => {
    document.getElementById('face-shape-font')?.remove();
    const style = document.createElement('style');
    style.id = 'face-shape-font';
    style.textContent = `*, *::before, *::after { font-family: ${fam} !important }`;
    document.head.appendChild(style);
    await document.fonts.ready;
    const card = document.querySelector('.handtrack .handfan .card');
    const w = card ? card.getBoundingClientRect().width : 142;
    const h = Math.round((w * 88) / 63);
    const row = document.createElement('div');
    row.style.cssText = `position:fixed;left:-9999px;top:0;width:${w}px;visibility:hidden`;
    const item = document.createElement('span');
    item.style.cssText = `display:inline-flex;width:${w}px;height:${h}px;background:red`;
    row.appendChild(item);
    document.body.appendChild(row);
    const strut = row.getBoundingClientRect().height - h;
    row.remove();
    return strut;
  }, family);
}

/**
 * shrinkFaces overrides the fan's --card-w to `px`, the size where one strut
 * of descender moves the ratio past the suite's tolerance; the measured box
 * is the same `.card` rule, only smaller.
 */
async function shrinkFaces(page: Page, px: number): Promise<void> {
  await page.evaluate((w: number) => {
    for (const fan of document.querySelectorAll<HTMLElement>('.handtrack .handfan')) fan.style.setProperty('--card-w', `${w}px`);
  }, px);
}

/** faceBoxes measures every own-hand face box. */
async function faceBoxes(page: Page): Promise<{ i: number; w: number; h: number; ratio: number }[]> {
  return page.evaluate(() =>
    [...document.querySelectorAll('.handtrack .handfan .card')].map((c, i) => {
      const b = c.getBoundingClientRect();
      return { i, w: b.width, h: b.height, ratio: b.width / b.height };
    }),
  );
}

/** A face this wide turns a 3px strut into a ratio error past toBeCloseTo(_, 1). */
const SMALL = 24;

async function assertShape(page: Page, label: string): Promise<void> {
  for (const family of FAMILIES) {
    const strut = await driveFont(page, family);
    expect(strut, `${label} ${family}: must drop a descender below a bare inline row, or this family cannot exercise the shape`).toBeGreaterThan(0);
    const faces = await faceBoxes(page);
    expect(faces.length, `${label} ${family}: own-hand faces measured`).toBeGreaterThan(0);
    for (const f of faces) {
      expect(f.ratio, `${label} ${family}: face ${f.i} (${f.w}x${f.h}) has a card's shape`).toBeCloseTo(63 / 88, 1);
      // The face box is exactly the card's ratio height: no strut painted below the image.
      expect(Math.abs(f.h - (f.w * 88) / 63), `${label} ${family}: face ${f.i} height ${f.h} is its width's card height`).toBeLessThan(0.5);
    }
  }
}

describe("the seated player's own-hand face keeps the card shape whatever font the machine resolves", () => {
  it('two seats seated 1440x900: every own-hand face measures 63/88 under every font', { timeout: 120_000 }, async () => {
    const page = await open('seats=2&mode=seated', 1440, 900);
    try {
      await assertShape(page, 'stock');
    } finally {
      await page.close();
    }
  });

  it('two seats seated 1440x900 with small faces: the strut would break the tolerance, and the box still measures 63/88', { timeout: 120_000 }, async () => {
    const page = await open('seats=2&mode=seated', 1440, 900);
    try {
      await shrinkFaces(page, SMALL);
      const faces = await faceBoxes(page);
      expect(faces.length, 'small faces measured').toBeGreaterThan(0);
      for (const f of faces) expect(f.w, `face ${f.i} took the overridden width`).toBeCloseTo(SMALL, 0);
      await assertShape(page, 'small');
    } finally {
      await page.close();
    }
  });
});
