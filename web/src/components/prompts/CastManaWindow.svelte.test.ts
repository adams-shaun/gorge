import { describe, expect, it, vi } from 'vitest';
import { render } from 'svelte/server';
import type { CardView, Decision, PlayerView, View } from '../../protocol';
import { SeatPanelState } from '../../lib/seatpanel.svelte';
import CastManaWindow from './CastManaWindow.svelte';
import SeatPanel from '../SeatPanel.svelte';

// SSR via svelte/server, the repo's component-test pattern (see
// ManaPaymentPanel.svelte.test.ts): no DOM, no effects, nothing reaches the
// network. Every assertion is a wire fact: the option indices, kinds and
// cost markers the engine offered, and where the decision's source sits.
vi.mock('../../lib/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../lib/api')>()),
  postIntent: vi.fn(),
  fetchPending: vi.fn(),
}));
vi.mock('../../lib/images', () => ({ images: { url: () => new Promise<string | null>(() => {}), offline: () => false } }));

const ctx = { seat: 0, token: 'tok' };
const seats = [{ name: 'you', deck: 'rakdos', colour: '#e5484d' }, { name: 'bot', deck: 'stompy', colour: '#30a46c' }];

const altar = (id: number, name: string): CardView => ({
  id, name, types: 'Artifact', printing: { name }, token: '', tapped: false, power: 0, toughness: 0,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
  produces: { colour: [0, 0, 0, 0, 0, 2], any: false },
});
const creature = (id: number, name: string): CardView => ({
  id, name, types: 'Creature', printing: { name }, token: '', tapped: false, power: 2, toughness: 2,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
});
const spell = (id: number, name: string): CardView => ({
  id, name, types: 'Creature', mana_cost: '1 R R', printing: { name }, token: '', tapped: false, power: 0, toughness: 0,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
});

const player = (seat: number, battlefield: CardView[], pool: Record<string, number>): PlayerView => ({
  seat, name: seats[seat].name, life: 20, lost: false, spells_cast_this_turn: 0, life_gained_this_turn: 0, life_lost_this_turn: 0, tokens_created_this_turn: 0,
  library_size: 40, hand_size: 0, graveyard_size: 0, completed_dungeons: 0,
  hand: [], battlefield, graveyard: [], exile: [], pool, command: [], commanders: [], commander_casts: [],
});

const stackEntry = {
  id: 30, kind: 'spell', name: 'Rakdos Pit Dragon', text: '', controller: 0, targets: [],
  card: spell(30, 'Rakdos Pit Dragon'), optional: false,
};
const board = [altar(41, "Ashnod's Altar"), altar(42, 'Phyrexian Altar'), creature(43, 'Grizzly Bears')];
const makeView = (d: Decision | null, pool: Record<string, number> = {}): View => ({
  viewer: 0, visibility: 'seat', turn: 3, round: 3, step: 'main1', phase: 'main1', active: 0, priority: 0,
  over: false, draw: false, winner: null,
  players: [player(0, board, pool), player(1, [], {})],
  stack: [stackEntry], pending: [], decision: d,
} as View);

// rules/cast_paymentplan.go manaWindowAsk: one activate per untapped source
// (the altars' beyond-tap cost on Option.cost), then Done; the spell is on
// the stack as the decision's source.
const windowDecision: Decision = {
  seq: 50, player: 0, kind: 'choose', prompt: 'Activate mana abilities to pay for Rakdos Pit Dragon',
  min: 1, max: 1, source: 30,
  options: [
    { index: 0, player: 0, kind: 'activate', obj: 41, label: "Activate Ashnod's Altar for mana", cost: 'T Sac<1/Creature>' },
    { index: 1, player: 0, kind: 'activate', obj: 42, label: 'Activate Phyrexian Altar for mana', cost: 'T Sac<1/Lion\'s Eye Diamond>' },
    { index: 2, player: 0, kind: 'done', label: 'Done' },
  ],
};

// rules/pay/manacostwalk.go ManaCostChoiceStages: the election one altar
// activation hands back.
const sacAsk: Decision = {
  seq: 51, player: 0, kind: 'choose', prompt: 'Choose permanents to sacrifice for the mana ability',
  min: 2, max: 2, source: 41,
  options: [
    { index: 0, player: 0, kind: 'sacrifice', obj: 43, label: 'Grizzly Bears' },
    { index: 1, player: 0, kind: 'sacrifice', obj: 44, label: 'Grizzly Bears' },
  ],
};

describe('CastManaWindow — the mid-cast mana window', () => {
  it('renders the cost readout, one row per source with its mana and cost text, and Done', () => {
    const state = new SeatPanelState('t1', 1, ctx, null, null);
    state.adoptView(windowDecision);
    const { html } = render(CastManaWindow, { props: { decision: windowDecision, view: makeView(windowDecision, { R: 1 }), logic: state } });
    expect(html).toContain('data-cast-mana');
    expect(html).toContain('data-cmp-cost');
    expect(html).toContain('data-cmp-owed');
    expect(html).toContain('data-cmp-pool');
    // Rows in offer order, each carrying its mana and its beyond-tap cost
    // words read from Option.cost — the engine's own marker, prettified.
    const sources = [...html.matchAll(/data-mana-source="(\d+)"/g)].map((m) => Number(m[1]));
    expect(sources).toEqual([41, 42]);
    expect(html).toContain('Sacrifice 1 Creature');
    expect(html).toContain("Sacrifice 1 Lion's Eye Diamond");
    // The answers are the engine's offered options, by their own index.
    expect(html).toContain('data-option="0"');
    expect(html).toContain('data-option="1"');
    expect(html).toContain('data-cmp-done="2"');
    // The floating pool reads off the view; the owed cell shows what is left.
    expect(html).toContain('Still owed (estimate)');
  });

  it('is the seat panel body for the cast window, never the generic option list', () => {
    const state = new SeatPanelState('t1', 1, ctx, null, null);
    state.adoptView(windowDecision);
    const { html } = render(SeatPanel, { props: { view: makeView(windowDecision, {}), seats, ctx, table: 't1', match: 1, state } });
    expect(html).toContain('data-cast-mana');
    expect(html).not.toContain('data-list-prompt');
  });
});

describe('CastManaWindow — the inline sacrifice election', () => {
  it('renders the candidates as rows of the same panel, not a generic list', () => {
    const state = new SeatPanelState('t1', 1, ctx, null, null);
    state.adoptView(sacAsk);
    const { html } = render(CastManaWindow, { props: { decision: sacAsk, view: makeView(sacAsk), logic: state } });
    expect(html).toContain('data-cast-mana-sac');
    expect(html).toContain('Choose permanents to sacrifice for the mana ability');
    const candidates = [...html.matchAll(/data-mana-source="(\d+)"/g)].map((m) => Number(m[1]));
    expect(candidates).toEqual([43, 44]);
    expect(html).toContain('Grizzly Bears');
    expect(html).toContain('The candidate permanents are highlighted on the board.');
    // A min==max==2 election toggles and commits: the footer carries the
    // submit button, not the window's Done.
    expect(html).toContain('data-submit');
    expect(html).not.toContain('data-cmp-done');
    expect(html).not.toMatch(/data-cast-mana(?!-sac)/);
  });
});
