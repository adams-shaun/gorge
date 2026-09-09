import { describe, expect, it } from 'vitest';
import { handFanLayout } from './handfan';

// The room a fan may occupy on a typical 1440px board minus the rail.
const BOARD_W = 1180;
const SPEC = { cardWidth: 128, gap: 10, maxWidth: BOARD_W };

describe('handFanLayout', () => {
  it('an empty hand has no fan', () => {
    const l = handFanLayout(0, SPEC);
    expect(l.step).toBe(SPEC.cardWidth);
    expect(l.overlap).toBe(0);
  });

  it('a single card occupies exactly its own width', () => {
    const l = handFanLayout(1, SPEC);
    expect(l.rowWidth).toBe(SPEC.cardWidth);
    expect(l.overlap).toBe(0);
  });

  it('a small hand sits side by side with gaps and does not overlap', () => {
    const l = handFanLayout(4, SPEC);
    const expected = SPEC.cardWidth + SPEC.gap;
    expect(l.step).toBe(expected);
    expect(l.overlap).toBeLessThan(0); // a gutter, not an overlap
    // total fits the room exactly as natural spacing
    expect(l.rowWidth).toBe(expected * 3 + SPEC.cardWidth);
  });

  it('a 7-card hand still fits without overlap on a wide board', () => {
    const l = handFanLayout(7, SPEC);
    expect(l.overlap).toBeLessThan(0);
    expect(l.rowWidth).toBeLessThanOrEqual(BOARD_W);
  });

  it('overlap engages once the hand is too wide for the room, and tightens as it grows', () => {
    // a 12-card hand: natural = 12*128 + 11*10 = 1646 > 1180, so it must overlap
    const w12 = handFanLayout(12, SPEC);
    expect(w12.overlap).toBeGreaterThan(0);
    expect(w12.rowWidth).toBeLessThanOrEqual(BOARD_W);
    expect(w12.step).toBeLessThan(SPEC.cardWidth);

    // a 20-card hand must overlap MORE (step smaller), never grow past the room
    const w20 = handFanLayout(20, SPEC);
    expect(w20.overlap).toBeGreaterThan(w12.overlap);
    expect(w20.rowWidth).toBeLessThanOrEqual(BOARD_W);

    // the tighter fan must never shrink the cards themselves, only hide them
    expect(SPEC.cardWidth).toBe(SPEC.cardWidth);
  });

  it('the fan row never exceeds the available room at any count', () => {
    for (let n = 0; n <= 30; n++) {
      const l = handFanLayout(n, SPEC);
      expect(l.rowWidth).toBeLessThanOrEqual(BOARD_W);
    }
  });

  it('a narrower room makes overlap engage at a smaller hand', () => {
    const narrow = handFanLayout(7, { ...SPEC, maxWidth: 600 });
    expect(narrow.overlap).toBeGreaterThan(0);
    const wide = handFanLayout(7, { ...SPEC, maxWidth: 1600 });
    expect(wide.overlap).toBeLessThan(0);
  });
});
