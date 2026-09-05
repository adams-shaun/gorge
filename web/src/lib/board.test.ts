import { describe, expect, it } from 'vitest';
import { groupBattlefield, quadrantFor, recentlyMattered, visibleHand } from './board';
import type { CardView, EventBody, PlayerView } from '../protocol';

const card = (id: number, types: string): CardView => ({ id, name: `c${id}`, types, tapped: false, power: 0, toughness: 0, damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false, printing: { name: `c${id}` }, token: `#${id}` });

// protocol.ts types PlayerView.hand as CardView[] (non-nullable), but the
// wire can and does send null (Spectator: Public hides every seat's hand as
// a Go nil slice, which JSON-encodes to null). The cast below reproduces
// that real shape for the test.
const player = (hand: CardView[] | null): PlayerView => ({
  seat: 0, name: 'p', life: 20, lost: false, library_size: 0, hand_size: 0, graveyard_size: 0,
  hand: hand as CardView[], battlefield: [], graveyard: [], exile: [], pool: {},
});

describe('board', () => {
  it('groups lands, creatures and the rest, ordered by id', () => {
    const g = groupBattlefield([card(9, 'Creature Goblin'), card(3, 'Basic Land Mountain'), card(5, 'Artifact'), card(2, 'Creature Human'), card(7, 'Artifact Creature Golem')]);
    expect(g.lands.map((c) => c.id)).toEqual([3]);
    expect(g.creatures.map((c) => c.id)).toEqual([2, 7, 9]);
    expect(g.others.map((c) => c.id)).toEqual([5]);
  });
  it('places seats in quadrants', () => {
    expect([0, 1, 2, 3].map((s) => quadrantFor(s, 4))).toEqual(['bl', 'tl', 'tr', 'br']);
    expect([0, 1].map((s) => quadrantFor(s, 2))).toEqual(['l', 'r']);
  });
  it('finds the last resolved object', () => {
    const ev = (seq: number, kind: string, obj?: number): EventBody => ({ event: { seq, kind, player: 0, obj }, line: '' });
    expect(recentlyMattered([ev(1, 'stack_push', 4), ev(2, 'stack_resolve', 4), ev(3, 'tap', 9)])).toBe(4);
    expect(recentlyMattered([ev(1, 'tap', 9)])).toBeNull();
  });
  it('treats a wire-null hand as invisible, not an empty array', () => {
    expect(visibleHand(player(null))).toBeNull();
    expect(visibleHand(player([]))).toEqual([]);
    expect(visibleHand(player([card(1, 'Creature')]))).toEqual([card(1, 'Creature')]);
  });
});
