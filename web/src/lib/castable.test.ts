import { describe, expect, it } from 'vitest';
import { cardCastableAfterTap, castableAfterTap } from './castable';
import type { CardView, PlayerView, View } from '../protocol';

/**
 * castable.test.ts pins the affordability question the auto-pass logic asks
 * (lib/castable): after Available joins Pool, is any nonland hand card
 * castable? Every case below is the client-side mirror of the engine's own
 * offer gate (rules/mana.go resolveMana, the float-then-cast payment model):
 * the post-land Lava Spike window is the reason this module exists, and the
 * pip shapes are what the repo decks' hand cards actually carry plus the
 * shapes mana.ts parses.
 */

/** card builds one minimal hand CardView; every field the helper reads is named. */
const card = (over: Partial<CardView>): CardView =>
  ({
    id: 1,
    name: 'Card',
    types: 'Instant',
    mana_cost: 'R',
    printing: { name: 'Card' },
    token: '',
    tapped: false,
    power: 0,
    toughness: 0,
    damage: 0,
    attacking: false,
    controller: 0,
    owner: 0,
    summon_sick: false,
    ...over,
  }) as CardView;

/** player builds one minimal PlayerView for seat 0 with a hand, pool and availability. */
const player = (over: Partial<PlayerView>): PlayerView =>
  ({
    seat: 0,
    name: 'p0',
    life: 20,
    lost: false,
    library_size: 0,
    hand_size: 0,
    graveyard_size: 0,
    hand: [],
    battlefield: [],
    graveyard: [],
    exile: [],
    pool: {},
    command: [],
    commanders: [],
    commander_casts: [],
    ...over,
  }) as PlayerView;

/** view builds a View whose seat-0 player is `p` (and optionally a second seat with NO hand — a hidden zone). */
const view = (p: PlayerView, secondSeat = true): View =>
  ({
    viewer: 0,
    active: 0,
    step: 'main1',
    players: secondSeat ? [p, { seat: 1 } as PlayerView] : [p],
  }) as unknown as View;

describe('castableAfterTap — the post-land window', () => {
  it('Lava Spike: {R} in hand, pool empty, one untapped Mountain (Available {R}) stops', () => {
    const p = player({ hand: [card({ mana_cost: 'R' })], pool: {}, available: { R: 1 } });
    expect(castableAfterTap(view(p), 0)).toBe(true);
  });

  it('the same {R} with the mana already FLOATING (pool {R}) is still true — the helper does not double-count, the kind test already caught that window', () => {
    const p = player({ hand: [card({ mana_cost: 'R' })], pool: { R: 1 }, available: { R: 1 } });
    expect(castableAfterTap(view(p), 0)).toBe(true);
  });

  it('Available {G} only cannot pay {R}: not stop-worthy', () => {
    const p = player({ hand: [card({ mana_cost: 'R' })], available: { G: 1 } });
    expect(castableAfterTap(view(p), 0)).toBe(false);
  });

  it('a dead-mana hand (uncastable card) with a Mountain in play: not stop-worthy', () => {
    const p = player({ hand: [card({ mana_cost: '4 U' })], available: { R: 1 } });
    expect(castableAfterTap(view(p), 0)).toBe(false);
  });

  it('a LAND in hand never stops the window — the land drop is the action that creates the mana', () => {
    const p = player({ hand: [card({ types: 'Land Mountain', mana_cost: '' })], available: { R: 1 } });
    expect(castableAfterTap(view(p), 0)).toBe(false);
  });

  it('a card with no printed cost does not stop the window (the engine already offers it when castable)', () => {
    const p = player({ hand: [card({ mana_cost: undefined })], available: { R: 1 } });
    expect(castableAfterTap(view(p), 0)).toBe(false);
  });
});

describe('castableAfterTap — pip shapes', () => {
  it('a generic pip is paid by the remainder: 1 R is affordable from one Available R, 2 R is not', () => {
    const one = player({ hand: [card({ mana_cost: '1 R' })], available: { R: 1 } });
    expect(castableAfterTap(view(one), 0)).toBe(true);
    const two = player({ hand: [card({ mana_cost: '2 R' })], available: { R: 1 } });
    expect(castableAfterTap(view(two), 0)).toBe(false);
  });

  it('coloured pips are reserved before generic (resolveMana\u2019s coloured-first): {R}{R} with one R fails, {2}{R} with two R succeeds', () => {
    // {R}{R} needs two R units; one R is one pip and nothing left for the second.
    const p = player({ hand: [card({ mana_cost: 'R R' })], available: { R: 1 } });
    expect(castableAfterTap(view(p), 0)).toBe(false);
    // {2}{R} with two R: pip takes one, generic is paid from the second.
    const ok = player({ hand: [card({ mana_cost: '2 R' })], available: { R: 2 } });
    expect(castableAfterTap(view(ok), 0)).toBe(true);
  });

  it('a strict {C} pip needs the colourless slot and generic may not steal it', () => {
    const ok = player({ hand: [card({ mana_cost: 'C' })], available: { C: 1 } });
    expect(castableAfterTap(view(ok), 0)).toBe(true);
    const notRed = player({ hand: [card({ mana_cost: 'C' })], available: { R: 1 } });
    expect(castableAfterTap(view(notRed), 0)).toBe(false);
  });

  it('an X-cost card is NEVER stop-worthy (the client cannot know the announced value)', () => {
    const p = player({ hand: [card({ mana_cost: 'X R' })], available: { R: 5 } });
    expect(castableAfterTap(view(p), 0)).toBe(false);
  });

  it('a hybrid pip is paid by either face: a W/U hybrid with only U available stops', () => {
    const p = player({ hand: [card({ mana_cost: 'W/U' })], available: { U: 1 } });
    expect(castableAfterTap(view(p), 0)).toBe(true);
    const neither = player({ hand: [card({ mana_cost: 'W/U' })], available: { R: 1 } });
    expect(castableAfterTap(view(neither), 0)).toBe(false);
  });

  it('a hybrid\u2019s colourless face ({C/W}) accepts the C slot or the colour', () => {
    const byC = player({ hand: [card({ mana_cost: 'C/W' })], available: { C: 1 } });
    expect(castableAfterTap(view(byC), 0)).toBe(true);
    const byW = player({ hand: [card({ mana_cost: 'C/W' })], available: { W: 1 } });
    expect(castableAfterTap(view(byW), 0)).toBe(true);
    const notEither = player({ hand: [card({ mana_cost: 'C/W' })], available: { R: 1 } });
    expect(castableAfterTap(view(notEither), 0)).toBe(false);
  });

  it('a twobrid pip ({2/W}) is paid by the colour OR falls back to two generic', () => {
    const byW = player({ hand: [card({ mana_cost: '2/W' })], available: { W: 1 } });
    expect(castableAfterTap(view(byW), 0)).toBe(true);
    const byGeneric = player({ hand: [card({ mana_cost: '2/W' })], available: { R: 2 } });
    expect(castableAfterTap(view(byGeneric), 0)).toBe(true);
    const neither = player({ hand: [card({ mana_cost: '2/W' })], available: { R: 1 } });
    expect(castableAfterTap(view(neither), 0)).toBe(false);
  });

  it('a Phyrexian pip is paid by its colour or two life — the engine offers the cast on the life path too', () => {
    const byColour = player({ hand: [card({ mana_cost: 'RP' })], available: { R: 1 } });
    expect(castableAfterTap(view(byColour), 0)).toBe(true);
    const byLife = player({ hand: [card({ mana_cost: 'RP' })], life: 2 });
    expect(castableAfterTap(view(byLife), 0)).toBe(true);
    const neither = player({ hand: [card({ mana_cost: 'RP' })], life: 1, available: { G: 1 } });
    expect(castableAfterTap(view(neither), 0)).toBe(false);
  });

  it('a phyrexian hybrid ({G/U/P}) is paid by either colour or life', () => {
    const byU = player({ hand: [card({ mana_cost: 'GUP' })], available: { U: 1 } });
    expect(castableAfterTap(view(byU), 0)).toBe(true);
    const byLife = player({ hand: [card({ mana_cost: 'GUP' })], life: 2 });
    expect(castableAfterTap(view(byLife), 0)).toBe(true);
  });

  it('a snow pip ({S}) and an unknown pip are refused (the wire names no snow source)', () => {
    const snow = player({ hand: [card({ mana_cost: 'S' })], available: { R: 3 } });
    expect(castableAfterTap(view(snow), 0)).toBe(false);
    const unknown = player({ hand: [card({ mana_cost: 'Q' })], available: { R: 3 } });
    expect(castableAfterTap(view(unknown), 0)).toBe(false);
  });
});

describe('castableAfterTap — fails closed', () => {
  it('a seat with no readable hand (another viewer\u2019s hand is JSON null) never stops', () => {
    const p = player({ hand: null as unknown as CardView[] });
    expect(castableAfterTap(view(p), 0)).toBe(false);
  });

  it('a seat the view does not carry never stops', () => {
    const p = player({ hand: [card({ mana_cost: 'R' })], available: { R: 1 } });
    expect(castableAfterTap(view(p, false), 1)).toBe(false);
  });

  it('cardCastableAfterTap is per-card: the land and no-cost skips are visible', () => {
    const p = player({ available: { R: 1 } });
    expect(cardCastableAfterTap(p, card({ mana_cost: 'R' }))).toBe(true);
    expect(cardCastableAfterTap(p, card({ types: 'Land Forest', mana_cost: '' }))).toBe(false);
    expect(cardCastableAfterTap(p, card({ mana_cost: 'X G' }))).toBe(false);
  });
});
