import { describe, expect, it, vi } from 'vitest';
import { render } from 'svelte/server';
import type { CardView, PlayerView } from '../protocol';
import type { CardOptions } from '../lib/cardoptions';
import { optionsByObj, optionsByPlayer } from '../lib/cardoptions';
import { PLAY_CARD_WIDTH } from '../lib/handfan';
import HandFan from './HandFan.svelte';

// SSR via svelte/server, the repo's component-test pattern: no DOM, no
// $effect, no pointer/hover lifecycle. The fan's per-card `left` is derived
// from the pure handFanLayout over the `width` prop, so overlap (second card's
// left < card width) is observable in the rendered HTML without a browser.
// The options affordance (ui23) is reached by the same seed CardTile uses
// (`open0`): the harness cannot click a badge to open a menu, so the test
// drives the menu's own state by hand.

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
const CARD_W = PLAY_CARD_WIDTH;

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

// ---------------------------------------------------------------------------
// ui23 — the hand gets the same options index a board tile does. One mechanism
// (cardoptions.ts), one index, one post path (R-E4-1). The suite asserts the
// affordance RENDERS, driven via the open0 seed (a badge click cannot happen
// in an SSR harness); the real click->menu->index path is guarded end-to-end
// in web/e2e/smoke.spec.ts.
// ---------------------------------------------------------------------------
function bundle(over: Partial<CardOptions> = {}): CardOptions {
  const decisions = {
    seq: 1, player: 0, kind: 'priority', prompt: 'main1', min: 1, max: 1,
    options: [
      { index: 3, kind: 'cast', label: 'Cast Walking Ballista', obj: 16, player: 0 },
      { index: 8, kind: 'cast', label: 'Cast Eldrazi Mimic', obj: 17, player: 0 },
    ],
  };
  return {
    byObj: optionsByObj(decisions as never),
    byPlayer: optionsByPlayer(decisions as never),
    picked: [],
    tone: 'offered',
    post: vi.fn(),
    ...over,
  };
}

const ballistaHand = player([card(16, 'Walking Ballista')]);

describe('HandFan options affordance (ui23)', () => {
  it('one hand-card option is a direct cast icon with the wire label as its accessible name', () => {
    const { html } = render(HandFan, { props: { player: ballistaHand, width: BOARD_W, options: bundle(), open0: 16 } });
    expect(html).toContain('data-single-action');
    expect(html).toContain('data-action-icon="cast"');
    expect(html).toContain('aria-label="Cast Walking Ballista"');
    expect(html).not.toContain('aria-haspopup');
    expect(html).not.toContain('role="menu"');
    expect(html).not.toContain('badge__n');
  });

  it('two options keep the count badge and popout with server labels VERBATIM', () => {
    const two = bundle();
    two.byObj.set(16, [
      { index: 3, kind: 'cast', label: 'Cast Walking Ballista', obj: 16, player: 0 },
      { index: 11, kind: 'ability', label: 'Walking Ballista: remove a counter', obj: 16, player: 0 },
    ]);
    const { html } = render(HandFan, { props: { player: ballistaHand, width: BOARD_W, options: two, open0: 16 } });
    expect(html).toContain('aria-haspopup');
    expect(html).toContain('2 actions for Walking Ballista');
    expect(html).toContain('Cast Walking Ballista');
    expect(html).toContain('Walking Ballista: remove a counter');
    expect(html).toContain('role="menu"');
    expect(html).toContain('role="menuitem"');
  });

  it('a card with no options offer renders no badge and no menu (the no-mark state)', () => {
    const { html } = render(HandFan, { props: { player: ballistaHand, width: BOARD_W, options: null } });
    expect(html).not.toContain('aria-haspopup');
    expect(html).not.toContain('tile-actions');
  });

  it('the mark wears the decision tone: initiative for a blocked decision, offered for an open window', () => {
    const initiative = render(HandFan, { props: { player: ballistaHand, width: BOARD_W, options: bundle({ tone: 'initiative' }) } });
    expect(initiative.html).toContain('data-tone="initiative"');
    expect(initiative.html).toContain('badge--initiative');

    const offered = render(HandFan, { props: { player: ballistaHand, width: BOARD_W, options: bundle({ tone: 'offered' }) } });
    expect(offered.html).toContain('data-tone="offered"');
    expect(offered.html).toContain('badge--offered');
  });

  it('a picked option is visibly selected on the hand card with its pick order', () => {
    const { html } = render(HandFan, { props: { player: ballistaHand, width: BOARD_W, options: bundle({ picked: [3] }) } });
    expect(html).toContain('data-selected="1"');
    expect(html).toContain('class="sel data');
  });
});
