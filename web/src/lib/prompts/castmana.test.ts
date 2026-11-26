import { describe, expect, it } from 'vitest';
import type { CardView, Decision, PlayerView, View } from '../../protocol';
import { activationCostText, castManaRows, isCastManaWindow, isManaSacrificeAsk, manaWindowDone } from './castmana';

/**
 * castmana.test.ts builds the mid-cast CR 601.2g window the engine poses
 * (rules/cast_paymentplan.go manaWindowAsk) and its follow-up sacrifice
 * election (rules/pay/manacostwalk.go) by hand, beside the sibling windows
 * the same activate+done shape must NOT capture, and asserts only wire
 * facts: kinds, option shapes and where the decision's source sits.
 */

const altar = (id: number, name: string, colour: [number, number, number, number, number, number], any = false): CardView => ({
  id, name, types: 'Artifact', printing: { name }, token: '', tapped: false, power: 0, toughness: 0,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
  produces: { colour, any },
});
const creature = (id: number, name: string): CardView => ({
  id, name, types: 'Creature', printing: { name }, token: '', tapped: false, power: 2, toughness: 2,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
});
const spell = (id: number, name: string, manaCost: string): CardView => ({
  id, name, types: 'Creature', mana_cost: manaCost, printing: { name }, token: '', tapped: false, power: 0, toughness: 0,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
});

const player = (seat: number, battlefield: CardView[], pool: Record<string, number>): PlayerView => ({
  seat, name: seats[seat].name, life: 20, lost: false, spells_cast_this_turn: 0, life_gained_this_turn: 0, life_lost_this_turn: 0, tokens_created_this_turn: 0,
  library_size: 40, hand_size: 0, graveyard_size: 0, completed_dungeons: 0,
  hand: [], battlefield, graveyard: [], exile: [], pool, command: [], commanders: [], commander_casts: [],
});
const seats = [{ name: 'you', deck: 'rakdos', colour: '#e5484d' }, { name: 'bot', deck: 'stompy', colour: '#30a46c' }];
const view = (d: Decision | null, battlefield: CardView[] = [], stack: View['stack'] = [], pool: Record<string, number> = {}): View => ({
  viewer: 0, visibility: 'seat', turn: 3, round: 3, step: 'main1', phase: 'main1', active: 0, priority: 0,
  over: false, draw: false, winner: null,
  players: [player(0, battlefield, pool), player(1, [], {})],
  stack, pending: [], decision: d,
} as View);

// A Rakdos-tier cast: the spell is on the stack (id 30), and the only mana
// sources are two sacrifice altars.
const onStack: View['stack'] = [{
  id: 30, kind: 'spell', name: 'Rakdos Pit Dragon', text: '', controller: 0, targets: [],
  card: spell(30, 'Rakdos Pit Dragon', '1 R R'), optional: false,
}];
const board = [
  altar(41, "Ashnod's Altar", [0, 0, 0, 0, 0, 2]),
  altar(42, 'Phyrexian Altar', [0, 0, 0, 0, 0, 0], true),
  creature(43, 'Grizzly Bears'),
];

const activate = (index: number, obj: number, cost?: string) => ({
  index, player: 0, kind: 'activate', obj, label: `Activate source for mana`, ...(cost ? { cost } : {}),
});
const windowDecision: Decision = {
  seq: 50, player: 0, kind: 'choose', prompt: 'Activate mana abilities to pay for Rakdos Pit Dragon',
  min: 1, max: 1, source: 30,
  options: [
    activate(0, 41, 'T Sac<1/Creature>'),
    activate(1, 42, 'T Sac<1/Creature>'),
    { index: 2, player: 0, kind: 'done', label: 'Done' },
  ],
};

describe('isCastManaWindow — the mid-cast mana window by shape', () => {
  it('captures the cast window: choose, activate+done options, source on the stack', () => {
    expect(isCastManaWindow(windowDecision, view(windowDecision, board, onStack))).toBe(true);
  });

  it('does not capture a priority window that also offers bare taps', () => {
    const prio: Decision = {
      seq: 51, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1,
      options: [activate(0, 41), { index: 1, player: 0, kind: 'pass', label: 'Pass priority' }],
    };
    expect(isCastManaWindow(prio, view(prio, board, onStack))).toBe(false);
  });

  it('does not capture the announced select-mana window (it has mana_payment)', () => {
    const announced: Decision = {
      ...windowDecision,
      mana_payment: { card: 30, cost: { generic: 1, mana: [0, 0, 0, 2, 0, 0] }, owed: { generic: 1, mana: [0, 0, 0, 2, 0, 0] }, pool: [0, 0, 0, 0, 0, 0] },
    } as Decision;
    expect(isCastManaWindow(announced, view(announced, board, onStack))).toBe(false);
  });

  it('does not capture the sibling payment windows, whose source is not on the stack', () => {
    // cumulative upkeep / echo / triggered cost (paymentManaAskClass) and
    // ward: the same activate+done shape, Source = a battlefield permanent.
    const sibling: Decision = {
      ...windowDecision, source: 41,
      prompt: 'Activate mana abilities to pay cumulative upkeep',
    };
    expect(isCastManaWindow(sibling, view(sibling, board, onStack))).toBe(false);
  });

  it('does not capture a choose whose options are not activate+done', () => {
    const modes: Decision = {
      seq: 52, player: 0, kind: 'choose', prompt: 'Choose a mode', min: 1, max: 1, source: 30,
      options: [
        { index: 0, player: 0, kind: 'mode', label: 'Mode one' },
        { index: 1, player: 0, kind: 'mode', label: 'Mode two' },
      ],
    };
    expect(isCastManaWindow(modes, view(modes, board, onStack))).toBe(false);
  });

  it('does not capture the sacrifice election that follows an activation', () => {
    const sac: Decision = {
      seq: 53, player: 0, kind: 'choose', prompt: 'Choose permanents to sacrifice for the mana ability',
      min: 1, max: 1, source: 41,
      options: [{ index: 0, player: 0, kind: 'sacrifice', obj: 43, label: 'Grizzly Bears' }],
    };
    expect(isCastManaWindow(sac, view(sac, board, onStack))).toBe(false);
    expect(isManaSacrificeAsk(sac)).toBe(true);
  });
});

describe('isManaSacrificeAsk — the mana-cost sacrifice election by shape', () => {
  const sacAsk: Decision = {
    seq: 54, player: 0, kind: 'choose', prompt: 'Choose permanents to sacrifice for the mana ability',
    min: 2, max: 2, source: 42,
    options: [
      { index: 0, player: 0, kind: 'sacrifice', obj: 43, label: 'Grizzly Bears' },
      { index: 1, player: 0, kind: 'sacrifice', obj: 44, label: 'Sable' },
    ],
  };

  it('captures the walk\'s ask: every option a sacrifice candidate with an object', () => {
    expect(isManaSacrificeAsk(sacAsk)).toBe(true);
  });

  it('refuses a list with any non-sacrifice option and one with no candidates', () => {
    const mixed: Decision = { ...sacAsk, options: [...sacAsk.options, { index: 2, player: 0, kind: 'done', label: 'Done' }] };
    expect(isManaSacrificeAsk(mixed)).toBe(false);
    expect(isManaSacrificeAsk({ ...sacAsk, options: [] })).toBe(false);
  });

  it('refuses a non-exact-N ask (min and max disagree)', () => {
    expect(isManaSacrificeAsk({ ...sacAsk, min: 0, max: 2 })).toBe(false);
  });
});

describe('castManaRows — the window\'s source rows', () => {
  it('groups the activate options by source in offer order, with each source\'s mana and cost text', () => {
    const d = view(windowDecision, board, onStack);
    const rows = castManaRows(windowDecision, d);
    expect(rows.map((r) => r.obj)).toEqual([41, 42]);
    expect(rows.map((r) => r.name)).toEqual(["Ashnod's Altar", 'Phyrexian Altar']);
    expect(rows[0].mana).toBe('C C');
    expect(rows[1].any).toBe(true);
    expect(rows[0].options.map((o) => o.index)).toEqual([0]);
  });

  it('keeps the wire cost marker on the row option for the panel to show', () => {
    const rows = castManaRows(windowDecision, view(windowDecision, board, onStack));
    expect(rows[0].options[0].cost).toBe('T Sac<1/Creature>');
  });

  it('names a source the view does not show as "Source <id>"', () => {
    const rows = castManaRows(windowDecision, null);
    expect(rows[0].name).toBe('Source 41');
  });
});

describe('manaWindowDone and activationCostText', () => {
  it('finds the window\'s done option', () => {
    expect(manaWindowDone(windowDecision)?.index).toBe(2);
  });

  it('reads the beyond-tap cost marker as payment words', () => {
    expect(activationCostText('T Sac<1/Lion\'s Eye Diamond>')).toBe("Tap Sacrifice 1 Lion's Eye Diamond");
    expect(activationCostText('T Sac<1/Creature>')).toBe('Tap Sacrifice 1 Creature');
    expect(activationCostText('T PayLife<1>')).toBe('Tap Pay 1 life');
    expect(activationCostText('')).toBe('');
    expect(activationCostText('T 2 PayEnergy<1>')).toBe('Tap 2 Pay 1 energy');
  });
});
