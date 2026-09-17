import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView, EventBody, PlayerView, SeatInfo, View } from '../protocol';
import ResolvedCard from './ResolvedCard.svelte';
import { HoverCard, type AnchorRect } from '../lib/carddetail.svelte';
import { SEAT_COLOURS } from '../lib/colours';

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

describe('ResolvedCard — the row names the card, is hoverable, and reads its owner (fb-20260917T131253Z-41d199c8)', () => {
  const anchor: AnchorRect = { left: 10, top: 20, right: 66 };
  const seats: SeatInfo[] = [
    { name: 'Ari', deck: 'deck-a', colour: '#ff0000' },
    { name: 'Bo', deck: 'deck-b', colour: '#00ff00' },
  ];

  it('renders the card name as TEXT, not only as the art alt text', () => {
    const v = viewWithGraveyard(42, 'Grizzly Bears');
    const { html } = render(ResolvedCard, {
      props: { view: v, events: [ev(1, 'stack_resolve', 42)] },
    });
    expect(html).toContain('resolved__name'); // a dedicated name span, so the name survives the art failing to load
    expect(html).toContain('Grizzly Bears');
  });

  it('colours the row border by the resolving seat — server-known colour wins', () => {
    const v = baseView({ players: [spectatorPlayer(0, 'Ari'), spectatorPlayer(1, 'Bo', { graveyard: [card({ id: 5, name: 'Bolt', controller: 1, owner: 1 })] })] });
    const { html } = render(ResolvedCard, {
      props: { view: v, events: [ev(1, 'stack_resolve', 5)], seats },
    });
    expect(html).toContain('border-color: #00ff00'); // seat 1's colour from the seat list
  });

  it('falls back to the shared palette when seats is not passed', () => {
    const v = baseView({ players: [spectatorPlayer(0, 'Ari'), spectatorPlayer(1, 'Bo', { graveyard: [card({ id: 5, name: 'Bolt', controller: 1, owner: 1 })] })] });
    const { html } = render(ResolvedCard, {
      props: { view: v, events: [ev(1, 'stack_resolve', 5)] },
    });
    expect(html).toContain(`border-color: ${SEAT_COLOURS[1]}`);
  });

  it('renders the CardDetail inspector while the hover state is open (dwell/focus), like every other card surface', () => {
    const v = viewWithGraveyard(42, 'Grizzly Bears');
    const hover = new HoverCard();
    hover.open(42);
    const { html } = render(ResolvedCard, {
      props: { view: v, events: [ev(1, 'stack_resolve', 42)], hover, anchor },
    });
    expect(html).toContain('card-detail');
    expect(html).toContain('aria-describedby="card-detail-42"');
  });

  it('renders no inspector while the hover state is closed', () => {
    const v = viewWithGraveyard(42, 'Grizzly Bears');
    const hover = new HoverCard();
    hover.arm(42); // armed (dwell started), not opened — no panel yet
    const { html } = render(ResolvedCard, {
      props: { view: v, events: [ev(1, 'stack_resolve', 42)], hover, anchor },
    });
    expect(html).not.toContain('card-detail');
  });

  it('supervise closes a live panel when a newer resolve replaces this one', () => {
    const v = viewWithGraveyard(42, 'Old Resolve');
    const hover = new HoverCard();
    hover.open(42);
    const { html } = render(ResolvedCard, {
      props: { view: v, events: [ev(1, 'stack_resolve', 42)], hover, anchor },
    });
    expect(html).toContain('card-detail');
    // The row's supervise list is the card it is CURRENTLY showing (the $effect's
    // [card]); asserted directly, as StackTile's test does — the next view shows a
    // different object, so the panel for 42 must close.
    expect(hover.supervise(42, [card({ id: 43, name: 'New Resolve' })])).toBe(true);
    expect(hover.show).toBe(false);
  });
});
