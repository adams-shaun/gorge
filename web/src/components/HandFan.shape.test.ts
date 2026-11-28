import { type Browser, type Page } from 'playwright';
import { beforeAll, describe, expect, it } from 'vitest';
import { browserURL, sharedBrowser } from '../test/browser';

/**
 * A hand fan's positioned .card box must be exactly the size of the
 * CardImage it wraps. The wrapper has width + aspect-ratio but an automatic
 * height, so its inline child's line box can still make it taller than the
 * face: ubuntu-24.04's fallback font (DejaVu Sans) added 3px that way and
 * failed SpectatorHands.geometry.test.ts's "a card's shape" assertion in CI.
 * The box, not the font, has to decide the size.
 */

let browser: Browser;

beforeAll(async () => {
  browser = await sharedBrowser();
});

interface Box { width: number; height: number }
interface FaceBox { wrapper: Box; image: Box }

async function faceBoxes(page: Page, selector: string): Promise<FaceBox[]> {
  return page.evaluate((sel) => [...document.querySelectorAll(sel)].map((card) => {
    const image = card.querySelector('.card-image');
    if (!image) throw new Error(`card has no CardImage: ${card.outerHTML.slice(0, 120)}`);
    const wrapper = card.getBoundingClientRect();
    const face = image.getBoundingClientRect();
    return {
      wrapper: { width: wrapper.width, height: wrapper.height },
      image: { width: face.width, height: face.height },
    };
  }), selector);
}

function expectCardSize(boxes: FaceBox[], who: string): void {
  expect(boxes.length, `${who}: faces rendered`).toBeGreaterThan(0);
  boxes.forEach((b, i) => {
    // Both boxes must be real (non-zero) so the equality below compares two
    // measured boxes rather than two absent ones.
    expect(b.image.width, `${who} ${i}: CardImage width`).toBeGreaterThan(0);
    expect(b.image.height, `${who} ${i}: CardImage height`).toBeGreaterThan(0);
    expect(b.wrapper.width, `${who} ${i}: card box width matches its face`).toBeCloseTo(b.image.width, 5);
    expect(b.wrapper.height, `${who} ${i}: card box height matches its face`).toBeCloseTo(b.image.height, 5);
    expect(b.wrapper.width / b.wrapper.height, `${who} ${i}: card box has a card's shape`).toBeCloseTo(63 / 88, 1);
  });
}

describe('a hand fan card box', () => {
  it('is exactly its CardImage in the omniscient spectator hand', { timeout: 120_000 }, async () => {
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    try {
      await page.goto(`${browserURL}src/components/SpectatorHands.fixture.html?seats=8&mode=omni&preset=focus8`);
      await page.waitForFunction(() => (window as Window & { fixtureReady?: boolean }).fixtureReady === true, undefined, { timeout: 60_000 });
      await page.waitForSelector('[data-passive-hand] [data-hand-card] .card-image', { state: 'attached', timeout: 60_000 });
      const boxes = await faceBoxes(page, '[data-passive-hand] [data-hand-card]');
      expect(boxes.length, 'every exposed seat hand renders').toBe(56);
      expectCardSize(boxes, 'spectator');
    } finally {
      await page.close();
    }
  });

  it('is exactly its CardImage in the seated hand', { timeout: 30_000 }, async () => {
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    try {
      await page.goto(`${browserURL}src/components/HandFan.geometry.html`);
      await page.waitForSelector('[data-obj] .face .card-image', { state: 'attached', timeout: 60_000 });
      const boxes = await faceBoxes(page, '[data-obj]');
      expect(boxes.length, 'every hand card renders').toBe(7);
      expectCardSize(boxes, 'seated');
    } finally {
      await page.close();
    }
  });
});
