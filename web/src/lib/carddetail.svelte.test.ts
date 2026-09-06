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
    const low = placePanel({ left: 100, top: 700, right: 190 }, 1200, 800);
    // not enough room below 700 for even a short panel: pull up to 632 and
    // cap the height at what remains (160)
    expect(low.y).toBe(800 - 8 - 160);
    expect(low.y + low.maxHeight).toBe(800 - 8);
    const high = placePanel({ left: 100, top: -50, right: 190 }, 1200, 800);
    expect(high.y).toBe(8);
    expect(high.maxHeight).toBe(800 - 8 - 8);
    // mid-viewport keeps the card's own top: the panel is capped at what
    // fits below it, never overflowing the bottom
    const mid = placePanel({ left: 100, top: 400, right: 190 }, 1200, 800);
    expect(mid.y).toBe(400);
    expect(mid.y + mid.maxHeight).toBe(800 - 8);
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
