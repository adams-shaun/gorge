import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView, PlayerView } from '../protocol';
import HandFan from './HandFan.svelte';

// SSR via svelte/server, the repo's component-test pattern: no DOM, no
// $effect, no pointer/hover lifecycle. The fan's per-card `left` is derived
// from the pure handFanLayout over the `width` prop, so overlap (second card's
// left < card width) is observable in the rendered HTML without a browser.

const card = (id: number, name: string): CardView => ({
  id, name, types: 'Instant', mana_cost: 'U',
  printing: { name }, token: '', tapped: false, power: 0, toughness: 0,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
});

const player = (hand: CardView[] | null): PlayerView => ({
  seat: 0, name: 'You', life: 40, lost: false, library_size: 60, hand_size: hand?.length ?? 0,
  graveyard_size: 0, hand: hand as CardView[], battlefield: [], graveyard: [], exile: [],
  pool: {}, command: [], commanders: [], commander_casts: [],
});

const hand = (n: number): CardView[] => Array.from({ length: n }, (_, i) => card(i + 1, `Card ${i + 1}`));

// The room a fan must fit: a typical 1440px board minus the rail. Same number
// the pure layout tests use, so the component's overlap observation agrees.
const BOARD_W = 1180;
const CARD_W = 128;

function lefts(html: string): number[] {
  // the rendered inline style ends with a `;`, so the match stops at `px`
  return [...html.matchAll(/style="left:\s*([0-9.]+)px/g)].map((m) => parseFloat(m[1]));
}

describe('HandFan — the seated player\'s own hand (Task ui17)', () => {
  it('an empty hand renders nothing (no fan, no cards, no crash)', () => {
    const { html } = render(HandFan, { props: { player: player([]), width: BOARD_W } });
    // an empty array is a real empty hand: no fan container, no card faces
    // (Svelte 5 SSRs empty blocks as comment markers, so assert on content)
    expect(html).not.toContain('handfan');
    expect(html).not.toContain('data-obj');
  });

  it('a null hand (a Go nil slice, literal JSON null) renders nothing and does not throw', () => {
    // The shape that once broke the whole client (see e2e/smoke.spec.ts). A
    // member of a public spectator's view is exactly this: `hand: null`.
    expect(() =>
      render(HandFan, { props: { player: player(null as unknown as CardView[]), width: BOARD_W } }),
    ).not.toThrow();
    const { html } = render(HandFan, { props: { player: player(null as unknown as CardView[]), width: BOARD_W } });
    expect(html).not.toContain('handfan');
    expect(html).not.toContain('data-obj');
  });

  it('a 7-card hand renders all seven card faces, side by side (no overlap on a wide board)', () => {
    const { html } = render(HandFan, { props: { player: player(hand(7)), width: BOARD_W } });
    expect(lefts(html)).toHaveLength(7);
    for (let i = 0; i < 7; i++) {
      expect(html).toContain(`data-obj="${i + 1}"`);
    }
    // second face starts at card width + gutter — spaced, not overlapped
    const l = lefts(html);
    expect(l[1]).toBeGreaterThanOrEqual(CARD_W);
  });

  it('a large (12+) hand engages overlap: the row still fits the room and later faces slide under', () => {
    const { html } = render(HandFan, { props: { player: player(hand(12)), width: BOARD_W } });
    const l = lefts(html);
    expect(l).toHaveLength(12);
    // overlap engaged: the second face begins before a full card width
    expect(l[1]).toBeLessThan(CARD_W);
    expect(l[1]).toBeGreaterThan(l[0]);
    // the fan is monotonically stepped and bounded by the card width
    for (let i = 1; i < l.length; i++) expect(l[i]).toBeGreaterThan(l[i - 1]);
  });
});
