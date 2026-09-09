import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView, StackView, View } from '../protocol';
import { HoverCard, type AnchorRect } from '../lib/carddetail.svelte';
import StackTile from './StackTile.svelte';

const anchor: AnchorRect = { left: 40, top: 40, right: 80 };

// SSR via svelte/server, the repo's component-test pattern (see CardTile.
// svelte.test.ts, whose own note applies here word for word): no DOM, so
// pointerenter never fires and $effect never runs — StackTile's own
// $effect (`hover.supervise(stack.card.id, stackCards)`, the reaction that
// closes a stale panel when the entry's card leaves the stack) cannot be
// exercised by rendering alone. Every test below drives the shared
// HoverCard directly instead — arm/open to reach the open state, and
// hover.supervise(...) itself for the closing behaviour, exactly the call
// the $effect makes — and asserts what StackTile renders from that state.

const card = (over: Partial<CardView> = {}): CardView => ({
  id: 40, name: 'Lightning Bolt', types: 'Instant', mana_cost: 'R',
  printing: { name: 'Lightning Bolt' }, token: '', tapped: false, power: 0, toughness: 0,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
  ...over,
});

const spell = (over: Partial<StackView> = {}): StackView => ({
  id: 40, kind: 'spell', name: 'Lightning Bolt', text: 'Lightning Bolt deals 3 damage to any target.',
  controller: 0, targets: [], card: card(), ...over,
  optional: over.optional ?? false,
});

const view = (stack: StackView[]): View => ({
  viewer: 255, visibility: 'omniscient', turn: 1, round: 1, step: 'main1', phase: 'main1', active: 0, priority: 0,
  over: false, draw: false, winner: null, stack, pending: [],
  players: [{
    seat: 0, name: 'P0', life: 40, lost: false, library_size: 90, hand_size: 7, graveyard_size: 0,
    hand: [], battlefield: [], graveyard: [], exile: [], pool: {}, command: [], commanders: [], commander_casts: [],
  }],
});

describe('StackTile', () => {
  it('renders the card face when stack.card is present (Task 2a: use it whenever it is there)', () => {
    const s = spell();
    const { html } = render(StackTile, { props: { stack: s, view: view([s]) } });
    expect(html).toContain('card-image');
    expect(html).toContain('Lightning Bolt');
  });

  it('degrades gracefully with no card face — no image, no crash, no placeholder box (Task 2a)', () => {
    const s = spell({ card: null, kind: 'trigger', name: 'Some Permanent' });
    const { html } = render(StackTile, { props: { stack: s, view: view([s]) } });
    expect(html).not.toContain('card-image');
    expect(html).toContain('Some Permanent');
  });

  it('renders the detail panel while the hover state is open — the state the guard closes', () => {
    const s = spell();
    const hover = new HoverCard();
    hover.open(40); // dwell completed / keyboard focus: the panel opened for object 40
    const { html } = render(StackTile, { props: { stack: s, view: view([s]), hover, anchor } });
    expect(html).toContain('card-detail');
    expect(html).toContain('#40'); // the panel names the object it describes
  });

  it('supervise closes the panel when this entry\'s card is no longer on the stack (resolved while hovered)', () => {
    const s = spell();
    const hover = new HoverCard();
    hover.open(40);
    // The independent list is every card CURRENTLY on the stack — here, none
    // (the entry resolved). This is exactly what StackTile's own $effect
    // computes as `stackCards` and passes to hover.supervise; asserted
    // directly (see file-level note) and then rendered from the result.
    expect(hover.supervise(40, [])).toBe(true); // a live panel was closed
    const { html } = render(StackTile, { props: { stack: s, view: view([]), hover, anchor } });
    expect(html).not.toContain('card-detail');
  });

  it('supervise keeps the panel open while the card is still present, even under a DIFFERENT entry', () => {
    const s = spell();
    const other = spell({ id: 41, card: card({ id: 41, name: 'Shock' }) });
    const hover = new HoverCard();
    hover.open(40);
    // present list independent of the id being checked: id 40 is still on
    // the stack (as part of `other`'s sibling `s`), so nothing closes.
    expect(hover.supervise(40, [card({ id: 41, name: 'Shock' }), card()])).toBe(false);
    const { html } = render(StackTile, { props: { stack: s, view: view([s, other]), hover, anchor } });
    expect(html).toContain('card-detail');
  });
});
