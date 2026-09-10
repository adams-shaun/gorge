import { describe, expect, it } from 'vitest';
import { placeMenu, placeRadial, MENU_WIDTH, MENU_MARGIN, RADIAL_BUTTON, RADIAL_MARGIN } from './menuplacement';

// placeMenu pins the viewport guarantees of the card options menu (task
// ui23): it opens down and right-aligned to the badge, flips up when there is
// not enough room below, and caps maxHeight so a tall menu scrolls inside the
// viewport rather than leaving it.

const VW = 1440;
const VH = 900;

describe('placeMenu', () => {
  it('opens below the badge, right-aligned to its right edge, when there is room', () => {
    const anchor = { left: 600, top: 300, right: 700, bottom: 390 };
    const p = placeMenu(anchor, VW, VH);
    expect(p.up).toBe(false);
    // right edge aligned to the badge's right edge (minus the menu width)
    expect(p.x).toBe(anchor.right - MENU_WIDTH);
    // opens just below the badge
    expect(p.y).toBe(anchor.bottom + MENU_MARGIN);
    expect(p.maxHeight).toBeGreaterThanOrEqual(64);
  });

  it('flips up when the card is near the bottom of the viewport', () => {
    // a land tile near the bottom edge: almost no room below
    const anchor = { left: 600, top: 860, right: 700, bottom: 950 };
    const p = placeMenu(anchor, VW, VH);
    expect(p.up).toBe(true);
    expect(p.y).toBeLessThan(anchor.top);
    // the menu must end above the badge's top (no downward overflow)
    expect(p.y + (p.maxHeight <= anchor.top ? p.maxHeight : anchor.top)).toBeLessThanOrEqual(anchor.top + MENU_MARGIN);
  });

  it('clamps to the viewport so a card near the right edge never runs off screen', () => {
    const anchor = { left: 1400, top: 200, right: 1436, bottom: 236 };
    const p = placeMenu(anchor, VW, VH);
    expect(p.x + MENU_WIDTH).toBeLessThanOrEqual(VW - MENU_MARGIN);
    expect(p.x).toBeGreaterThanOrEqual(MENU_MARGIN);
  });

  it('caps maxHeight to the space available in the chosen direction', () => {
    // a card very close to the top, so even the above-space is tight
    const anchor = { left: 600, top: 4, right: 700, bottom: 66 };
    const p = placeMenu(anchor, VW, VH);
    // downward is preferred (there is plenty of room below a top card), and
    // maxHeight is bounded by the viewport bottom
    expect(p.up).toBe(false);
    expect(p.y + p.maxHeight).toBeLessThanOrEqual(VH - MENU_MARGIN);
  });
});

describe('placeRadial', () => {
  it('opens inward from a viewport-edge badge and keeps all 42px circles on screen', () => {
    const points = placeRadial({ left: 2, top: 430, right: 20, bottom: 448 }, 6, VW, VH);
    expect(points).toHaveLength(6);
    for (const point of points) {
      expect(point.x).toBeGreaterThanOrEqual(RADIAL_MARGIN);
      expect(point.y).toBeGreaterThanOrEqual(RADIAL_MARGIN);
      expect(point.x + RADIAL_BUTTON).toBeLessThanOrEqual(VW - RADIAL_MARGIN);
      expect(point.y + RADIAL_BUTTON).toBeLessThanOrEqual(VH - RADIAL_MARGIN);
    }
    // The cluster as a whole opens toward the board/viewport centre.
    expect(points.reduce((sum, point) => sum + point.x, 0) / points.length).toBeGreaterThan(20);
  });
});
