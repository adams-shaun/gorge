import { describe, expect, it, vi } from 'vitest';
import { render } from 'svelte/server';
import type { CardView, Decision, Option, PlayerView, SeatInfo, View } from '../protocol';
import SeatPanel from './SeatPanel.svelte';

// SSR via svelte/server, the repo's component-test pattern: onMount and
// $effect never run, so the panel renders from the view it was handed and
// nothing reaches the network. ./api is stubbed anyway so an accidental
// fetch would fail loudly rather than hang.
vi.mock('../lib/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/api')>()),
  postIntent: vi.fn(),
  fetchPending: vi.fn(),
}));
// images.url resolves the Scryfall art; in SSR the promise never settles
// before render, so CardImage draws its text face. Stubbed to keep the test
// off the network entirely.
vi.mock('../lib/images', () => ({ images: { url: () => new Promise<string | null>(() => {}), offline: () => false } }));

const ctx = { seat: 1, token: 'tok' };
const seats: SeatInfo[] = [
  { name: 'alice', deck: 'burn', colour: '#e5484d' },
  { name: 'bob', deck: 'stompy', colour: '#30a46c' },
];

const card = (id: number, name: string): CardView => ({
  id, name, types: 'Creature Bear', printing: { name }, token: `#${id}`,
  tapped: false, power: 2, toughness: 2, damage: 0, attacking: false,
  controller: 1, owner: 1, summon_sick: false,
});

const player = (seat: number, hand: CardView[], pool: Record<string, number> = {}): PlayerView => ({
  seat, name: seats[seat].name, life: 20, lost: false, library_size: 53,
  hand_size: hand.length, graveyard_size: 0, hand, battlefield: [], graveyard: [], exile: [], pool,
});

const opt = (index: number, kind: string, label: string, obj?: number): Option =>
  ({ index, kind, label, obj, player: 1 });

// Seat 1 is the viewer and deliberately sits SECOND in the players array, so
// a panel indexing by array position instead of by seat number reads alice's
// row and the tests catch it.
const view = (decision: Decision | null, hand: CardView[] = [], pool: Record<string, number> = {}): View => ({
  viewer: 1, visibility: 'seat', turn: 3, step: 'upkeep', phase: 'beginning',
  active: 0, priority: 1, over: false, draw: false, winner: null,
  players: [player(0, []), player(1, hand, pool)],
  stack: [], pending: [], decision,
});

const props = (v: View) => ({ view: v, seats, ctx, table: 't1', match: 1 });

const priority: Decision = {
  seq: 5, player: 1, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1,
  options: [opt(0, 'cast', 'Cast Grizzly Bears'), opt(1, 'pass', 'Pass priority'), opt(2, 'concede', 'Concede')],
};
const keepAsk: Decision = {
  seq: 6, player: 1, kind: 'mulligan', min: 1, max: 1,
  prompt: 'London mulligan: bob keeps 7 and bottoms 1, or mulligans',
  options: [opt(0, 'keep', 'keep'), opt(1, 'mulligan', 'mulligan')],
};
const hand = [card(101, 'Grizzly Bears'), card(102, 'Forest'), card(103, 'Llanowar Elves')];
const bottomAsk: Decision = {
  seq: 7, player: 1, kind: 'mulligan', min: 2, max: 2,
  prompt: 'London mulligan: bob bottoms 2 card(s)',
  options: hand.map((c, j) => opt(j, 'bottom', c.name, c.id)),
};

describe('SeatPanel — the test contract', () => {
  it('a live decision carries every data hook the seat surface is tested through', () => {
    const { html } = render(SeatPanel, { props: props(view(priority)) });
    for (const hook of ['data-seat-panel', 'data-prompt', 'data-options', 'data-option="0"', 'data-primary', 'data-concede']) {
      expect(html).toContain(hook);
    }
  });

  it('with nothing to answer the panel waits, and names who on', () => {
    const { html } = render(SeatPanel, { props: props(view(null)) });
    expect(html).toContain('data-waiting');
    expect(html).toContain('waiting for bob'); // view.priority is seat 1
    expect(html).not.toContain('data-prompt');
  });

  it('the primary is the pass option resolved by kind, never the last option', () => {
    const { html } = render(SeatPanel, { props: props(view(priority)) });
    const primaryLabel = /data-primary[^>]*>\s*([^<]+?)\s*</.exec(html)?.[1];
    expect(primaryLabel).toBe('Pass priority');
    expect(primaryLabel).not.toBe('Concede');
  });
});

describe('SeatPanel — tone', () => {
  it('a priority window is cool: passing is a legal answer', () => {
    expect(render(SeatPanel, { props: props(view(priority)) }).html).toContain('data-tone="offered"');
  });

  it('a decision the game is blocked on is warm', () => {
    expect(render(SeatPanel, { props: props(view(keepAsk, hand)) }).html).toContain('data-tone="initiative"');
  });

  it('waiting on someone else carries no state colour at all', () => {
    expect(render(SeatPanel, { props: props(view(null)) }).html).toContain('data-tone="idle"');
  });
});

describe('SeatPanel — the mana pool', () => {
  it("reads THIS seat's pool by seat number, not by position in the players array", () => {
    const { html } = render(SeatPanel, { props: props(view(priority, [], { G: 2 })) });
    expect(html).toContain('data-mana="G"');
    expect(html).toContain('data-mana-pool');
  });

  it('an empty pool puts nothing on the panel', () => {
    expect(render(SeatPanel, { props: props(view(priority)) }).html).not.toContain('data-mana-pool');
  });

  it('a view with no row for this seat renders the panel without a pool rather than throwing', () => {
    const v = view(priority);
    v.players = [player(0, [])];
    const { html } = render(SeatPanel, { props: props(v) });
    expect(html).toContain('data-seat-panel');
    expect(html).not.toContain('data-mana-pool');
  });
});

describe('SeatPanel — the mulligan round', () => {
  it('the keep half draws the hand as cards and the two choices as buttons, keyed by option kind', () => {
    const { html } = render(SeatPanel, { props: props(view(keepAsk, hand)) });
    expect(html).toContain('data-prompt');
    expect(html).toContain('data-options');
    // the hand, as cards: CardTile's per-object anchor, one per card
    for (const c of hand) expect(html).toContain(`data-obj="${c.id}"`);
    // the choices, labelled by what pressing them does
    expect(html).toContain('data-option="0"');
    expect(html).toContain('Keep this hand');
    expect(html).toContain('data-option="1"');
    expect(html).toContain('Mulligan');
    // no generic scrolling option list, and no primary: a mulligan has no pass
    expect(html).not.toContain('data-primary');
  });

  it('a spent mulligan allowance leaves the single keep choice', () => {
    const d: Decision = { ...keepAsk, options: [opt(0, 'keep', 'keep')] };
    const { html } = render(SeatPanel, { props: props(view(d, hand)) });
    expect(html).toContain('Keep this hand');
    expect(html).not.toContain('data-option="1"');
  });

  it('the bottoming half makes every card a control, with the submit that commits them', () => {
    const { html } = render(SeatPanel, { props: props(view(bottomAsk, hand)) });
    expect([...html.matchAll(/data-option="\d+"/g)]).toHaveLength(hand.length);
    expect(html).toContain('aria-pressed="false"');
    expect(html).toContain('data-submit');
    expect(html).toContain('Bottom 2 cards');
    // each control names its card for a screen reader
    for (const c of hand) expect(html).toContain(`aria-label="${c.name}"`);
  });

  it('bottoming one card says card, not cards', () => {
    const d: Decision = { ...bottomAsk, min: 1, max: 1 };
    expect(render(SeatPanel, { props: props(view(d, hand)) }).html).toContain('Bottom 1 card<');
  });

  // A bottoming option whose obj is not in the hand still has to be
  // answerable: the option, not the hand, is the contract.
  it('an option with no matching card falls back to its label and stays clickable', () => {
    const d: Decision = { ...bottomAsk, options: [opt(0, 'bottom', 'Unknown Card', 999)] };
    const { html } = render(SeatPanel, { props: props(view(d, hand)) });
    expect(html).toContain('data-option="0"');
    expect(html).toContain('Unknown Card');
  });

  // FL-101: the layout is driven by kinds. An option kind this body does not
  // cover sends the whole decision to the generic list, where it is reachable.
  it('an unrecognised option kind falls back to the generic list rather than being dropped', () => {
    const d: Decision = { ...keepAsk, options: [opt(0, 'keep', 'keep'), opt(1, 'mulligan', 'mulligan'), opt(2, 'surprise', 'Something new')] };
    const { html } = render(SeatPanel, { props: props(view(d, hand)) });
    expect(html).toContain('data-option="2"');
    expect(html).toContain('Something new');
    expect(html).toContain('keep'); // the server's own labels, verbatim
    expect(html).not.toContain('Keep this hand');
  });
});

describe('SeatPanel — match over', () => {
  it('names the winner and adds no colour of its own', () => {
    const v = { ...view(null), over: true, winner: 1 };
    const { html } = render(SeatPanel, { props: props(v) });
    expect(html).toContain('bob wins');
    expect(html).toContain('Match over');
  });

  it('a draw says draw', () => {
    const v = { ...view(null), over: true, draw: true, winner: null };
    expect(render(SeatPanel, { props: props(v) }).html).toContain('Draw');
  });
});
