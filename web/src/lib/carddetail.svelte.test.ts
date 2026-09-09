import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView } from '../protocol';
import { DWELL, HoverCard, placePanel, type AnchorRect } from './carddetail.svelte';
import CardDetail from '../components/CardDetail.svelte';
import CardTile from '../components/CardTile.svelte';

// fakeTimer is a deterministic clock: setTimeout/clearTimeout land in a
// table driven by tick, so the 250ms dwell is testable without real time.
function fakeTimer() {
  let clock = 0;
  const timers = new Map<number, { at: number; fn: () => void }>();
  let next = 1;
  const env = {
    setTimeout: (fn: () => void, ms: number) => {
      const id = next++;
      timers.set(id, { at: clock + ms, fn });
      return id;
    },
    clearTimeout: (id: unknown) => void timers.delete(id as number),
  };
  return {
    env,
    tick(ms: number) {
      clock += ms;
      for (const [id, t] of [...timers]) {
        if (t.at <= clock) {
          timers.delete(id);
          t.fn();
        }
      }
    },
  };
}

describe('HoverCard', () => {
  it('pointer dwell opens after DWELL ms; pointer leave before it cancels', () => {
    const { env, tick } = fakeTimer();
    const h = new HoverCard(env);
    const opened: string[] = [];
    h.arm(() => opened.push('open'));
    expect(h.show).toBe(false);
    tick(DWELL - 1);
    expect(h.show).toBe(false);
    expect(opened).toEqual([]);
    h.close(); // pointer leaves before the dwell completes
    tick(1);
    expect(h.show).toBe(false);
    expect(opened).toEqual([]);
  });

  it('pointer dwell that completes opens exactly once, even if armed repeatedly', () => {
    const { env, tick } = fakeTimer();
    const h = new HoverCard(env);
    h.arm();
    h.arm(); // a second pointerenter over the same tile must not stack a timer
    tick(DWELL);
    expect(h.show).toBe(true);
    tick(DWELL * 4);
    expect(h.show).toBe(true); // no second timer ever fires
    h.close();
    expect(h.show).toBe(false);
  });

  it('keyboard focus opens immediately and Escape closes it', () => {
    const { env } = fakeTimer();
    const h = new HoverCard(env);
    expect(h.show).toBe(false);
    h.open();
    expect(h.show).toBe(true);
    h.keydown({ key: 'Escape' });
    expect(h.show).toBe(false);
  });

  it('blur and pointer leave both close, and an armed dwell is cancelled', () => {
    const { env, tick } = fakeTimer();
    const h = new HoverCard(env);
    h.arm();
    h.close(); // blur/leave while the dwell is pending
    expect(h.show).toBe(false);
    tick(DWELL);
    expect(h.show).toBe(false);

    h.open();
    expect(h.show).toBe(true);
    h.close();
    expect(h.show).toBe(false);
  });

  it('open cancels an armed dwell instead of stacking it', () => {
    const { env, tick } = fakeTimer();
    const h = new HoverCard(env);
    h.arm();
    h.open(); // focus arrives while the pointer dwell is still pending
    expect(h.show).toBe(true);
    tick(DWELL);
    expect(h.show).toBe(true); // the armed timer must not re-fire a second open
    h.close();
    tick(DWELL);
    expect(h.show).toBe(false);
  });

  it('stays open while the object it describes is still present', () => {
    const { env, tick } = fakeTimer();
    const h = new HoverCard(env);
    h.arm();
    tick(DWELL);
    expect(h.show).toBe(true);
    // the surface re-feeds the cards it currently shows; this one is still there
    const closed = h.supervise(42, [card({ id: 42 }), card({ id: 7 })]);
    expect(closed).toBe(false);
    expect(h.show).toBe(true);
  });

  it('closes when the object it describes goes away, with no pointer event delivered', () => {
    const { env, tick } = fakeTimer();
    const h = new HoverCard(env);
    // pointer dwell opens the panel on object 42
    h.arm();
    tick(DWELL);
    expect(h.show).toBe(true);
    // the card is played: it leaves the hand, so the trigger <li> is removed
    // from the DOM while the pointer is over it and pointerleave NEVER fires.
    const closed = h.supervise(42, [card({ id: 7 })]);
    expect(closed).toBe(true);
    expect(h.show).toBe(false);
  });

  it('close-on-gone only acts when something is open (nothing open stays a no-op)', () => {
    const h = new HoverCard();
    expect(h.supervise(null, [])).toBe(false);
    expect(h.supervise(42, [])).toBe(false); // nothing open for 42 yet
    expect(h.show).toBe(false);
    h.open();
    expect(h.supervise(42, [card({ id: 42 })])).toBe(false); // present: open stays
    expect(h.supervise(42, [])).toBe(true); // gone: closes
    expect(h.show).toBe(false);
  });

  // --- superviseRendering: the panel's lifetime follows the OBJECT the
  // panel was opened for, not the mounted instance. On the board Svelte
  // keeps a CardTile instance alive when its card prop changes (the
  // permanent it showed was destroyed, the tile is handed the next one)
  // and the pointer never fires a leave — so nothing else closes the
  // panel, and worse, the tile now renders a card the reader never asked
  // for. The surface therefore re-feeds the id it is rendering and the
  // panel closes when it describes a different object. HandList's
  // supervise ('the object left the visible set') is a separate contract
  // and stays untouched above.

  it('closes a panel opened for object 16 when the tile now renders object 12 — the kept-instance re-target bug', () => {
    const { env, tick } = fakeTimer();
    const h = new HoverCard(env);
    h.arm(16); // the pointer dwells on the tile showing object 16
    tick(DWELL); // dwell completes: the panel opens on 16
    expect(h.show).toBe(true);
    // object 16 is destroyed; the board re-renders and Svelte hands this
    // SAME tile instance to object 12. The pointer has not moved, so no
    // pointerleave fires — only this check can close the panel now.
    const closed = h.superviseRendering(12);
    expect(closed).toBe(true); // a live panel was closed
    expect(h.show).toBe(false);
  });

  it('stays open while the tile renders the same object the panel describes', () => {
    const { env, tick } = fakeTimer();
    const h = new HoverCard(env);
    h.arm(16);
    tick(DWELL);
    expect(h.show).toBe(true);
    // the surface re-feeds the same id on an unrelated re-render
    const closed = h.superviseRendering(16);
    expect(closed).toBe(false);
    expect(h.show).toBe(true);
  });

  it('re-points an armed dwell at the object the tile renders now instead of cancelling it', () => {
    const { env, tick } = fakeTimer();
    const h = new HoverCard(env);
    h.arm(16); // dwell pending on 16...
    h.superviseRendering(12); // ...when the tile is handed object 12 before the timer fires
    expect(h.show).toBe(false); // nothing was open, so nothing closed
    tick(DWELL);
    expect(h.show).toBe(true); // the dwell still completes
    expect(h.superviseRendering(12)).toBe(false); // for the object being rendered
    expect(h.show).toBe(true);
    expect(h.superviseRendering(16)).toBe(true); // the old id is now the wrong one
    expect(h.show).toBe(false);
  });

  it('open (keyboard focus path) learns the object too, so the same check holds', () => {
    const h = new HoverCard();
    h.open(16);
    expect(h.show).toBe(true);
    expect(h.superviseRendering(16)).toBe(false);
    expect(h.superviseRendering(12)).toBe(true);
    expect(h.show).toBe(false);
  });
});

describe('placePanel', () => {
  const anchor: AnchorRect = { left: 500, top: 400, right: 590 };

  it('opens to the right of the anchor when there is room, staying top-aligned', () => {
    const p = placePanel(anchor, 1200, 800);
    expect(p.x).toBe(590 + 8);
    expect(p.y).toBe(400);
    expect(p.x + 264).toBeLessThanOrEqual(1200 - 8);
    expect(p.y).toBeGreaterThanOrEqual(8);
  });

  it('flips to the left when the right side would overflow', () => {
    const p = placePanel(anchor, 700, 800); // 590 + 8 + 264 = 862 > 692
    expect(p.x).toBe(500 - 8 - 264);
    expect(p.x).toBeGreaterThanOrEqual(8);
  });

  it('clamps vertically so the panel never leaves the viewport', () => {
    // A bottom card (top 700 in an 800 viewport) has only 92px below it but
    // 692px above. The panel FLIPS to open upward, bottom-anchored just above
    // the card, so it gets the tall room above instead of the cramped band
    // below — this is the fix that stops a hand card's detail being cut off.
    const low = placePanel({ left: 100, top: 700, right: 190 }, 1200, 800);
    expect(low.y).toBeUndefined();
    expect(low.bottom!).toBe(800 - (700 - 8)); // panel's bottom edge = card top - margin
    expect(800 - low.bottom!).toBe(700 - 8); // bottom edge sits just above the card
    expect(low.maxHeight).toBe(700 - 16); // the full room above, less the two margins
    expect(800 - low.bottom! - low.maxHeight).toBe(8); // never clips past the top margin

    const high = placePanel({ left: 100, top: -50, right: 190 }, 1200, 800);
    expect(high.y).toBe(8);
    expect(high.maxHeight).toBe(800 - 8 - 8);
    // mid-viewport keeps the card's own top: the panel is capped at what
    // fits below it, never overflowing the bottom
    const mid = placePanel({ left: 100, top: 400, right: 190 }, 1200, 800);
    expect(mid.y).toBe(400);
    expect(mid.y! + mid.maxHeight).toBe(800 - 8);
  });

  it('opens upward for a bottom card so the detail has the room above it, not the cramped band below', () => {
    // The hand-fan shape: the card sits low, so the room below is a fixed,
    // small band while the room above is tall. The old code top-aligned the
    // panel at the card and capped it to the room below (here 337px), so a
    // card's content taller than that was cut and scrolled. The fix gives it
    // the room above instead.
    const fan = placePanel({ left: 106, top: 555, right: 234 }, 1440, 900);
    expect(fan.y).toBeUndefined();
    const bottomEdge = 900 - fan.bottom!;
    expect(bottomEdge).toBe(555 - 8); // just above the card
    const roomBelow = 900 - 8 - 555;
    expect(fan.maxHeight).toBeGreaterThan(roomBelow); // strictly more room than the old downward cap
    expect(bottomEdge - fan.maxHeight).toBe(8); // top stays at the margin, never clipped
  });

  it('keeps the panel within the viewport on both axes at every card position', () => {
    const vw = 1200;
    const vh = 800;
    for (const top of [0, 50, 400, 750, 800]) {
      const p = placePanel({ left: 100, top, right: 190 }, vw, vh);
      // Downward: top = y, bottom = y + maxHeight. Upward: bottom-anchored, so
      // bottom = vh - bottom and, at full maxHeight, top = (vh - bottom) - maxHeight.
      const isUp = p.bottom !== undefined;
      const bottomEdge = isUp ? vh - p.bottom! : p.y! + p.maxHeight;
      const topEdge = isUp ? vh - p.bottom! - p.maxHeight : p.y!;
      expect(topEdge, `top ${top}: panel top must be >= margin`).toBeGreaterThanOrEqual(8);
      expect(bottomEdge, `top ${top}: panel bottom must stay within viewport`).toBeLessThanOrEqual(vh - 8);
    }
  });
});

// --- CardDetail rendering -------------------------------------------------

const card = (overrides: Partial<CardView> = {}): CardView => ({
  id: 42, name: 'Squire', types: 'Creature — Human Soldier', mana_cost: '1 W',
  printing: { name: 'Squire' }, token: '', tapped: false, power: 3, toughness: 2,
  damage: 0, attacking: false, controller: 1, owner: 1, summon_sick: false,
  ...overrides,
});

const anchor = { left: 100, top: 100, right: 190 };

describe('CardDetail (wire-only)', () => {
  it('renders every wire field with no catalog', () => {
    const { html } = render(CardDetail, {
      props: { card: card({ damage: 2, counters: { '+1/+1': 2, flying: 3 }, keywords: ['flying', 'vigilance'], tapped: true, summon_sick: true }), anchor },
    });
    expect(html).toContain('Squire');
    expect(html).toContain('Creature — Human Soldier');
    expect(html).toContain('3/2'); // current power/toughness
    expect(html).toContain('damage marked');
    expect(html).toContain('title="2 +1/+1"'); // counter chip: count + kind
    expect(html).toContain('title="3 flying"');
    expect(html).toContain('+1/+1');
    expect(html).toContain('vigilance');
    expect(html).toContain('#42');
    expect(html).toContain('seat 1');
    expect(html).toContain('tapped');
    expect(html).toContain('summoning sick');
    // no catalog: no oracle block, no spinner, no error
    expect(html).not.toContain('class="card-detail__oracle"');
  });

  it('renders no oracle block when the resolver returns null', () => {
    const { html } = render(CardDetail, {
      props: { card: card(), anchor, resolver: () => null },
    });
    expect(html).not.toContain('card-detail__oracle');
  });
});

describe('CardDetail (with a catalog resolver)', () => {
  it('renders oracle text when the resolver supplies it', () => {
    const { html } = render(CardDetail, {
      props: {
        card: card(),
        anchor,
        resolver: () => ({
          name: 'Squire', mana_cost: '1 W', type_line: 'Creature — Human Soldier',
          oracle_text: 'Vigilance', power: '1', toughness: '2',
        }),
      },
    });
    expect(html).toContain('card-detail__oracle');
    expect(html).toContain('Vigilance');
  });

  it('states current vs printed plainly when they disagree, without inventing a base', () => {
    const { html } = render(CardDetail, {
      props: {
        card: card({ power: 3, toughness: 2 }),
        anchor,
        resolver: () => ({
          name: 'Squire', oracle_text: 'Vigilance', power: '1', toughness: '2',
        }),
      },
    });
    expect(html).toContain('Shown P/T is current; the printed card reads 1/2');
    // no fake base anywhere: the wire's own 3/2 is still shown as current
    expect(html).toContain('3/2');
  });
});

// --- wiring presence -------------------------------------------------------

describe('CardTile hover wiring', () => {
  it('keeps the data-obj anchor arrows read, and is keyboard focusable', () => {
    const { html } = render(CardTile, { props: { card: card() } });
    expect(html).toContain('data-obj="42"');
    expect(html).toContain('tabindex="0"');
    // panel is closed on first render: no detail, no anchor change
    expect(html).not.toContain('card-detail');
  });
});
