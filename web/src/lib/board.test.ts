import { describe, expect, it } from 'vitest';
import { attachedTo, everyVisibleCard, groupBattlefield, quadrantFor, RECENT_RESOLVE_WINDOW, recentlyMattered, stackFaces, stackIdentical, visibleHand } from './board';
import type { CardView, EventBody, PlayerView } from '../protocol';

const card = (id: number, types: string): CardView => ({ id, name: `c${id}`, types, tapped: false, power: 0, toughness: 0, damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false, printing: { name: `c${id}` }, token: `#${id}` });

// protocol.ts types PlayerView.hand as CardView[] (non-nullable), but the
// wire can and does send null (Spectator: Public hides every seat's hand as
// a Go nil slice, which JSON-encodes to null). The cast below reproduces
// that real shape for the test.
const player = (hand: CardView[] | null): PlayerView => ({
  seat: 0, name: 'p', life: 20, lost: false, library_size: 0, hand_size: 0, graveyard_size: 0,
  hand: hand as CardView[], battlefield: [], graveyard: [], exile: [], pool: {}, command: [], commanders: [], commander_casts: [],
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
  it('shows a fresh resolve that is inside the recency window', () => {
    const ev = (seq: number, kind: string, obj?: number): EventBody => ({ event: { seq, kind, player: 0, obj }, line: '' });
    // A resolve a few events back from the newest is still within the trailing
    // RECENT_RESOLVE_WINDOW, so the strip shows it.
    const arr = Array.from({ length: RECENT_RESOLVE_WINDOW + 1 }, (_, i) => ev(i, 'tap', i));
    arr[RECENT_RESOLVE_WINDOW - 5] = ev(RECENT_RESOLVE_WINDOW - 5, 'stack_resolve', 42);
    expect(recentlyMattered(arr)).toBe(42);
  });
  it('shows a resolve at the very newest position regardless of the window', () => {
    const ev = (seq: number, kind: string, obj?: number): EventBody => ({ event: { seq, kind, player: 0, obj }, line: '' });
    expect(recentlyMattered([ev(0, 'stack_resolve', 7)])).toBe(7);
  });
  it('clears a resolve that is further back than the window (shows NOTHING)', () => {
    const ev = (seq: number, kind: string, obj?: number): EventBody => ({ event: { seq, kind, player: 0, obj }, line: '' });
    // The only resolve is the OLDEST event of a buffer one longer than the
    // window, so it lies just outside the trailing RECENT_RESOLVE_WINDOW.
    const arr = Array.from({ length: RECENT_RESOLVE_WINDOW + 1 }, (_, i) => ev(i, 'tap', i));
    arr[0] = ev(0, 'stack_resolve', 99);
    expect(recentlyMattered(arr)).toBeNull();
  });
  it('treats the window boundary as inclusive: W events back still shows', () => {
    const ev = (seq: number, kind: string, obj?: number): EventBody => ({ event: { seq, kind, player: 0, obj }, line: '' });
    // Exactly RECENT_RESOLVE_WINDOW events, the resolve at index 0 is the
    // oldest of the trailing window -> still shown.
    const atBoundary = Array.from({ length: RECENT_RESOLVE_WINDOW }, (_, i) => ev(i, 'tap', i));
    atBoundary[0] = ev(0, 'stack_resolve', 11);
    expect(recentlyMattered(atBoundary)).toBe(11);
    // One event more, the same resolve at index 0 falls just outside the
    // trailing window -> cleared.
    const pastBoundary = Array.from({ length: RECENT_RESOLVE_WINDOW + 1 }, (_, i) => ev(i, 'tap', i));
    pastBoundary[0] = ev(0, 'stack_resolve', 11);
    expect(recentlyMattered(pastBoundary)).toBeNull();
  });
  it('treats a wire-null hand as invisible, not an empty array', () => {
    expect(visibleHand(player(null))).toBeNull();
    expect(visibleHand(player([]))).toEqual([]);
    expect(visibleHand(player([card(1, 'Creature')]))).toEqual([card(1, 'Creature')]);
  });
});

describe('attachments', () => {
  const perm = (id: number, types: string, attachedTo?: number): CardView =>
    ({ id, name: `c${id}`, types, printing: { name: `c${id}` }, token: '', tapped: false,
       power: 0, toughness: 0, damage: 0, attacking: false, controller: 0, owner: 0,
       summon_sick: false, attached_to: attachedTo }) as CardView;

  it('an attachment rides under its host instead of taking a slot of its own', () => {
    const bf = [perm(1, 'Creature'), perm(2, 'Artifact Equipment', 1)];
    const groups = groupBattlefield(bf);
    expect(groups.creatures.map((c) => c.id)).toEqual([1]);
    expect(groups.others).toEqual([]);
    expect(attachedTo(bf, 1).map((c) => c.id)).toEqual([2]);
  });

  it('an attachment whose host is not on this battlefield still gets drawn', () => {
    // A stolen host, or one that left: the attachment must not vanish.
    const bf = [perm(2, 'Enchantment Aura', 99)];
    expect(groupBattlefield(bf).others.map((c) => c.id)).toEqual([2]);
  });

  it('an unattached permanent is unaffected', () => {
    const bf = [perm(1, 'Creature'), perm(2, 'Land')];
    const groups = groupBattlefield(bf);
    expect(groups.creatures.map((c) => c.id)).toEqual([1]);
    expect(groups.lands.map((c) => c.id)).toEqual([2]);
    expect(attachedTo(bf, 1)).toEqual([]);
  });
});

// Stacking fixtures: same-printing Zombie tokens with every key field present
// as a default, so a test can flip one field and watch the group split.
type StackOver = Partial<Pick<CardView, 'name' | 'types' | 'printing' | 'tapped' | 'summon_sick' | 'attacking' | 'damage' | 'power' | 'toughness' | 'counters' | 'keywords' | 'controller' | 'owner' | 'attacking_player' | 'blocked_by' | 'attached_to'>>;
const zombie = (id: number, o: StackOver = {}): CardView => ({
  id, name: 'Zombie', types: 'Creature Zombie',
  printing: { name: 'Zombie', set: 'TOK', number: '25' }, token: `#${id}`,
  tapped: false, power: 2, toughness: 2, damage: 0, attacking: false,
  controller: 0, owner: 0, summon_sick: false, ...o,
});

describe('stackIdentical', () => {
  it('four identical tokens collapse into one group of four, id-sorted', () => {
    const groups = stackIdentical([zombie(9), zombie(4), zombie(7), zombie(1)]);
    expect(groups).toHaveLength(1);
    expect(groups[0].cards.map((c) => c.id)).toEqual([1, 4, 7, 9]);
  });

  it('one tapped separates two otherwise identical permanents', () => {
    const groups = stackIdentical([zombie(1), zombie(2, { tapped: true }), zombie(3)]);
    expect(groups.map((g) => g.cards.map((c) => c.id))).toEqual([[1, 3], [2]]);
  });

  it('different counters separate two otherwise identical permanents', () => {
    const groups = stackIdentical([
      zombie(1, { counters: { 'p1p1': 2 } }),
      zombie(2, { counters: { 'p1p1': 1 } }),
      zombie(3, { counters: { 'p1p1': 2 } }),
    ]);
    expect(groups.map((g) => g.cards.map((c) => c.id))).toEqual([[1, 3], [2]]);
  });

  it('counter keys are compared as a set, not by object identity or wire order', () => {
    const groups = stackIdentical([
      zombie(1, { counters: { a: 1, b: 2 } }),
      zombie(2, { counters: { b: 2, a: 1 } }),
    ]);
    expect(groups).toHaveLength(1);
    expect(groups[0].cards.map((c) => c.id)).toEqual([1, 2]);
  });

  it('different derived power separates two otherwise identical permanents', () => {
    // power on the wire is derived: an anthem on one of them showed up there.
    const groups = stackIdentical([
      zombie(1, { power: 3 }),
      zombie(2, { power: 2 }),
      zombie(3, { power: 3 }),
    ]);
    expect(groups.map((g) => g.cards.map((c) => c.id))).toEqual([[1, 3], [2]]);
  });

  it('a permanent with an attachment on it never joins a group', () => {
    const groups = stackIdentical([
      zombie(1),
      zombie(2, { attached_to: 1 }), // a rider on 1
      zombie(3),
    ]);
    // 1 hosts 2, so 1 is individual; 2 is itself attached, so it is too. Only
    // 3 is freely mergeable, but there is nothing left for it to merge with.
    expect(groups.map((g) => g.cards.map((c) => c.id))).toEqual([[1], [2], [3]]);
    expect(groups.every((g) => g.cards.length === 1)).toBe(true);
  });

  it('a permanent that is itself attached never joins a group', () => {
    const groups = stackIdentical([
      zombie(1),
      zombie(2, { attached_to: 9 }), // host 9 is off this row (stolen/elsewhere)
    ]);
    expect(groups.map((g) => g.cards.map((c) => c.id))).toEqual([[1], [2]]);
  });

  it('different controller separates two otherwise identical permanents', () => {
    const groups = stackIdentical([zombie(1, { controller: 0 }), zombie(2, { controller: 1 })]);
    expect(groups).toHaveLength(2);
  });

  it('a blocked attacker is not the same board object as an unblocked one', () => {
    const unblocked = stackIdentical([zombie(1), zombie(2, { attacking: true, attacking_player: 1 })]);
    expect(unblocked).toHaveLength(2);
    const blocked = stackIdentical([
      zombie(1, { attacking: true, attacking_player: 1 }),
      zombie(2, { attacking: true, attacking_player: 1, blocked_by: [5] }),
    ]);
    expect(blocked).toHaveLength(2);
    const sameBlocker = stackIdentical([
      zombie(1, { attacking: true, attacking_player: 1, blocked_by: [5] }),
      zombie(2, { attacking: true, attacking_player: 1, blocked_by: [5] }),
    ]);
    expect(sameBlocker).toHaveLength(1);
  });

  it('different keywords separate two otherwise identical permanents', () => {
    const groups = stackIdentical([zombie(1, { keywords: ['flying'] }), zombie(2, { keywords: ['flying', 'vigilance'] })]);
    expect(groups).toHaveLength(2);
    // same keyword set in a different wire order is still one group
    const same = stackIdentical([zombie(1, { keywords: ['flying', 'vigilance'] }), zombie(2, { keywords: ['vigilance', 'flying'] })]);
    expect(same).toHaveLength(1);
  });

  it('group and member order is stable and id-sorted from unsorted input', () => {
    const groups = stackIdentical([
      zombie(6), zombie(2), zombie(4, { tapped: true }), zombie(3), zombie(1), zombie(5),
    ]);
    expect(groups.map((g) => g.cards.map((c) => c.id))).toEqual([[1, 2, 3, 5, 6], [4]]);
  });

  it('a board with no duplicates produces one group of one per card, in id order', () => {
    const groups = stackIdentical([
      zombie(8, { printing: { name: 'A', set: 'S1', number: '1' } }),
      zombie(3, { printing: { name: 'B', set: 'S1', number: '1' } }),
      zombie(5, { printing: { name: 'A', set: 'S1', number: '2' } }),
    ]);
    expect(groups.map((g) => g.cards.map((c) => c.id))).toEqual([[3], [5], [8]]);
    expect(groups.every((g) => g.cards.length === 1)).toBe(true);
  });

  it('stackFaces: collapsed shows only the first card; expanded shows every member', () => {
    const g = stackIdentical([zombie(2), zombie(5), zombie(9)])[0];
    expect(stackFaces(g, false).map((c) => c.id)).toEqual([2]);
    expect(stackFaces(g, true).map((c) => c.id)).toEqual([2, 5, 9]);
  });
});

describe('everyVisibleCard — the public-spectator null zones', () => {
  const seat = (over: Partial<PlayerView>): PlayerView =>
    ({
      seat: 0, name: 'Player 1', life: 40, lost: false,
      library_size: 90, hand_size: 7, graveyard_size: 0,
      hand: [], battlefield: [], graveyard: [], exile: [],
      pool: {}, command: [], commanders: [], commander_casts: [],
      ...over,
    }) as PlayerView;
  const named = (id: number, name: string) => ({ id, name }) as CardView;

  it('survives the null hand and null pool a public spectator really receives', () => {
    // Exactly the wire shape captured from a live `-spectator public` server:
    // hand and pool are literal JSON nulls, every other zone is an array.
    // Spreading that hand crashed the whole table view.
    const players = [
      seat({ hand: null as unknown as CardView[], pool: null as unknown as Record<string, number>, battlefield: [named(66, 'Mountain')] }),
      seat({ seat: 1, hand: null as unknown as CardView[], graveyard: [named(31, 'Hordeling Outburst')] }),
    ];
    const names = everyVisibleCard(players).map((c) => c.name);
    expect(names).toEqual(['Mountain', 'Hordeling Outburst']);
  });

  it('collects every zone for an omniscient view', () => {
    const p = seat({
      hand: [named(1, 'Ponder')], battlefield: [named(2, 'Island')], graveyard: [named(3, 'Brainstorm')],
      exile: [named(4, 'Delve')], command: [named(5, 'Lathliss, Dragon Queen')], commanders: [named(5, 'Lathliss, Dragon Queen')],
    });
    expect(everyVisibleCard([p]).map((c) => c.name)).toEqual([
      'Ponder', 'Island', 'Brainstorm', 'Delve', 'Lathliss, Dragon Queen', 'Lathliss, Dragon Queen',
    ]);
  });

  it('treats a missing players list as empty rather than throwing', () => {
    expect(everyVisibleCard(null)).toEqual([]);
    expect(everyVisibleCard(undefined)).toEqual([]);
  });
});
