import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView, EventBody, PlayerView, View } from '../protocol';
import ResolvedCard from './ResolvedCard.svelte';

// SSR via svelte/server, the repo's component-test pattern (see
// Rail.svelte.test.ts / CardTile.svelte.test.ts): no DOM, no $effect, no
// timers. ResolvedCard is deliberately derived purely from its props — the
// wall-clock expiry died with the overlay — so these tests exercise exactly
// what production renders.

const card = (over: Partial<CardView> = {}): CardView => ({
  id: 1, name: 'Lightning Bolt', types: 'Instant', mana_cost: 'R',
  printing: { name: 'Lightning Bolt' }, token: '', tapped: false, power: 0, toughness: 0,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
  ...over,
});

const spectatorPlayer = (seat: number, name: string, over: Partial<PlayerView> = {}): PlayerView => ({
  seat, name, life: 40, lost: false, library_size: 60, hand_size: 7, graveyard_size: 0,
  hand: null as unknown as CardView[],
  pool: null as unknown as Record<string, number>,
  battlefield: [], graveyard: [], exile: [], command: [], commanders: [], commander_casts: [],
  ...over,
});

const baseView = (over: Partial<View> = {}): View => ({
  viewer: 255, visibility: 'public', turn: 3, round: 3, step: 'main1', phase: 'main1', active: 0, priority: 0,
  over: false, draw: false, winner: null, stack: [], pending: [],
  players: [spectatorPlayer(0, 'Ari'), spectatorPlayer(1, 'Bo')],
  ...over,
});

// EventBody events as the DVR list carries them; the component's contract is
// structural (event.kind + event.obj), which these literals satisfy exactly.
const ev = (seq: number, kind: string, obj?: number): EventBody =>
  ({ event: { seq, kind, player: 0, ...(obj !== undefined ? { obj } : {}) }, line: kind } as EventBody);

const viewWithGraveyard = (id: number, name: string): View =>
  baseView({ players: [spectatorPlayer(0, 'Ari', { graveyard: [card({ id, name })] }), spectatorPlayer(1, 'Bo')] });

describe('ResolvedCard — the resolved card lives in the stack frame (fb-20260916T225456Z)', () => {
  it('renders the last resolved card, labelled, when a resolve sits inside the event window', () => {
    const v = viewWithGraveyard(42, 'Grizzly Bears');
    const { html } = render(ResolvedCard, { props: { view: v, events: [ev(1, 'stack_push', 42), ev(2, 'stack_resolve', 42), ev(3, 'tap', 9)] } });
    expect(html).toContain('data-resolved="42"');
    expect(html).toContain('just resolved');
    expect(html).toContain('Grizzly Bears'); // the card face it landed on
    expect(html).toContain('card-image'); // the artwork, not text only
  });

  it('shows the NEWEST resolve when a later one replaces an earlier one', () => {
    const v = baseView({ players: [
      spectatorPlayer(0, 'Ari', { graveyard: [card({ id: 7, name: 'Old Resolve' }), card({ id: 11, name: 'New Resolve' })] }),
      spectatorPlayer(1, 'Bo'),
    ] });
    const { html } = render(ResolvedCard, {
      props: { view: v, events: [ev(1, 'stack_resolve', 7), ev(2, 'tap', 0), ev(3, 'stack_resolve', 11)] },
    });
    expect(html).toContain('data-resolved="11"');
    expect(html).toContain('New Resolve');
    expect(html).not.toContain('data-resolved="7"');
  });

  it('renders nothing when nothing has resolved recently', () => {
    const { html } = render(ResolvedCard, { props: { view: baseView(), events: [ev(1, 'tap', 9), ev(2, 'stack_push', 3)] } });
    expect(html).not.toContain('data-resolved');
  });

  it('renders nothing when the only resolve is older than the RECENT_RESOLVE_WINDOW bound', () => {
    // The window is a safety bound, unchanged from the strip: 100 trailing
    // events, so this resolve at the boundary's far edge is already stale.
    const events: EventBody[] = [ev(0, 'stack_resolve', 42)];
    for (let i = 1; i <= 100; i++) events.push(ev(i, 'tap', i));
    const v = viewWithGraveyard(42, 'Stale Card');
    const { html } = render(ResolvedCard, { props: { view: v, events } });
    expect(html).not.toContain('data-resolved');
  });
});
