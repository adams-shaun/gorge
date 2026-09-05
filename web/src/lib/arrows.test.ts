import { describe, expect, it } from 'vitest';
import { arrowsFor } from './arrows';
import type { CardView, View } from '../protocol';

const card = (id: number, extra: Partial<CardView> = {}): CardView => ({ id, name: `c${id}`, types: 'Creature', tapped: false, power: 1, toughness: 1, damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false, printing: { name: `c${id}` }, token: `#${id}`, ...extra });

describe('arrowsFor', () => {
  it('draws target, attack and block arrows in a fixed order', () => {
    const view = {
      players: [
        { seat: 0, battlefield: [card(1, { attacking: true, attacking_player: 1, blocked_by: [2] })], hand: null, graveyard: [], exile: [], pool: null },
        { seat: 1, battlefield: [card(2)], hand: null, graveyard: [], exile: [], pool: null },
      ],
      stack: [{ id: 9, kind: 'spell', name: 'Bolt', text: '', controller: 1, targets: [{ player: 0, is_player: true, label: 'any' }, { obj: 1, player: 0, is_player: false }] }],
      pending: [],
    } as unknown as View;
    expect(arrowsFor(view)).toEqual([
      { from: { obj: 9 }, to: { seat: 0 }, kind: 'target' },
      { from: { obj: 9 }, to: { obj: 1 }, kind: 'target' },
      { from: { obj: 1 }, to: { seat: 1 }, kind: 'attack' },
      { from: { obj: 2 }, to: { obj: 1 }, kind: 'block' },
    ]);
  });

  it('returns no arrows for an empty stack and no combat', () => {
    const view = {
      players: [
        { seat: 0, battlefield: [card(1)], hand: null, graveyard: [], exile: [], pool: null },
        { seat: 1, battlefield: [card(2)], hand: null, graveyard: [], exile: [], pool: null },
      ],
      stack: [],
      pending: [],
    } as unknown as View;
    expect(arrowsFor(view)).toEqual([]);
  });

  it('emits one target arrow per target on a spell with several targets, players and objects mixed', () => {
    const view = {
      players: [
        { seat: 0, battlefield: [card(1)], hand: null, graveyard: [], exile: [], pool: null },
        { seat: 1, battlefield: [card(2)], hand: null, graveyard: [], exile: [], pool: null },
      ],
      stack: [{
        id: 9, kind: 'spell', name: 'Chain Lightning', text: '', controller: 0,
        targets: [
          { player: 1, is_player: true, label: 'any' },
          { obj: 1, player: 0, is_player: false },
          { obj: 2, player: 1, is_player: false },
        ],
      }],
      pending: [],
    } as unknown as View;
    expect(arrowsFor(view)).toEqual([
      { from: { obj: 9 }, to: { seat: 1 }, kind: 'target' },
      { from: { obj: 9 }, to: { obj: 1 }, kind: 'target' },
      { from: { obj: 9 }, to: { obj: 2 }, kind: 'target' },
    ]);
  });

  it('emits one block arrow per blocker when an attacker has several', () => {
    const view = {
      players: [
        { seat: 0, battlefield: [card(1, { attacking: true, attacking_player: 1, blocked_by: [2, 3, 4] })], hand: null, graveyard: [], exile: [], pool: null },
        { seat: 1, battlefield: [card(2), card(3), card(4)], hand: null, graveyard: [], exile: [], pool: null },
      ],
      stack: [],
      pending: [],
    } as unknown as View;
    expect(arrowsFor(view)).toEqual([
      { from: { obj: 1 }, to: { seat: 1 }, kind: 'attack' },
      { from: { obj: 2 }, to: { obj: 1 }, kind: 'block' },
      { from: { obj: 3 }, to: { obj: 1 }, kind: 'block' },
      { from: { obj: 4 }, to: { obj: 1 }, kind: 'block' },
    ]);
  });

  it('emits no attack or block arrows when there is no combat', () => {
    const view = {
      players: [
        { seat: 0, battlefield: [card(1)], hand: null, graveyard: [], exile: [], pool: null },
        { seat: 1, battlefield: [card(2)], hand: null, graveyard: [], exile: [], pool: null },
      ],
      stack: [{ id: 9, kind: 'spell', name: 'Bolt', text: '', controller: 0, targets: [{ obj: 2, player: 1, is_player: false }] }],
      pending: [],
    } as unknown as View;
    expect(arrowsFor(view)).toEqual([
      { from: { obj: 9 }, to: { obj: 2 }, kind: 'target' },
    ]);
  });
});
