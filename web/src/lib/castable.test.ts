import { describe, expect, it } from 'vitest';
import { cardCastableAfterTap, cardRespondableAfterTap, castableAfterTap, respondableAfterTap } from './castable';
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
  it('a generic pip is paid from what the pips LEFT: {1}{R} from one R is NOT affordable, {1}{R} from two R is', () => {
    // {1}{R} is two mana; one R pays the pip and nothing is left for {1}.
    const one = player({ hand: [card({ mana_cost: '1 R' })], available: { R: 1 } });
    expect(castableAfterTap(view(one), 0)).toBe(false);
    const two = player({ hand: [card({ mana_cost: '1 R' })], available: { R: 2 } });
    expect(castableAfterTap(view(two), 0)).toBe(true);
  });

  it('coloured pips are reserved before generic (resolveMana\u2019s coloured-first): {R}{R} with one R fails, {2}{R} with two R fails — the pip\u2019s unit is gone', () => {
    // {R}{R} needs two R units; one R is one pip and nothing left for the second.
    const p = player({ hand: [card({ mana_cost: 'R R' })], available: { R: 1 } });
    expect(castableAfterTap(view(p), 0)).toBe(false);
    // {2}{R} is THREE mana: one R for the pip leaves one unit, and {2} needs two.
    const twoR = player({ hand: [card({ mana_cost: '2 R' })], available: { R: 2 } });
    expect(castableAfterTap(view(twoR), 0)).toBe(false);
    // Three units: the pip takes one, two remain for {2}.
    const threeR = player({ hand: [card({ mana_cost: '2 R' })], available: { R: 3 } });
    expect(castableAfterTap(view(threeR), 0)).toBe(true);
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

  it('a twobrid pip ({2/W}) is paid by the colour OR falls back to two generic — and the fallback still owes the real remainder', () => {
    const byW = player({ hand: [card({ mana_cost: '2/W' })], available: { W: 1 } });
    expect(castableAfterTap(view(byW), 0)).toBe(true);
    const byGeneric = player({ hand: [card({ mana_cost: '2/W' })], available: { R: 2 } });
    expect(castableAfterTap(view(byGeneric), 0)).toBe(true);
    const neither = player({ hand: [card({ mana_cost: '2/W' })], available: { R: 1 } });
    expect(castableAfterTap(view(neither), 0)).toBe(false);
    // The twobrid fallback adds {2} to the generic requirement, paid from what
    // the OTHER mana left: {1}{2/W} from {W:2} is the pip by colour plus one
    // unit left for {1} -- affordable -- but from {W:1} the pip's unit is gone
    // and {1} is unpaid, and from {R:2} the pip falls back to two generic plus
    // the printed {1} = three owed with two held -- not affordable.
    const mixedOk = player({ hand: [card({ mana_cost: '1 2/W' })], available: { W: 2 } });
    expect(castableAfterTap(view(mixedOk), 0)).toBe(true);
    const mixedNoW = player({ hand: [card({ mana_cost: '1 2/W' })], available: { W: 1 } });
    expect(castableAfterTap(view(mixedNoW), 0)).toBe(false);
    const mixedNo = player({ hand: [card({ mana_cost: '1 2/W' })], available: { R: 2 } });
    expect(castableAfterTap(view(mixedNo), 0)).toBe(false);
    const mixedThree = player({ hand: [card({ mana_cost: '1 2/W' })], available: { R: 3 } });
    expect(castableAfterTap(view(mixedThree), 0)).toBe(true);
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

describe('castableAfterTap — decision tap offers (fb-20260914T125925Z)', () => {
  const tapDecision = (obj: number): import('../protocol').Decision => ({
    seq: 1, player: 0, kind: 'priority', prompt: 'priority', min: 1, max: 1,
    options: [
      { index: 0, kind: 'activate', label: 'tap', obj, player: 0 },
      { index: 1, kind: 'pass', label: 'pass', player: 0 },
      { index: 2, kind: 'concede', label: 'concede', player: 0 },
    ],
  });

  it('uses an offered Tundra tap, not Available, to pay a white hand card', () => {
    const p = player({
      hand: [card({ mana_cost: 'W' })], pool: {}, available: { C: 1 },
      battlefield: [card({ id: 7, name: 'Tundra', types: 'Land Plains Island', produces: { colour: [1, 1, 0, 0, 0, 0], any: false } })],
    });
    expect(castableAfterTap(view(p), 0, tapDecision(7))).toBe(true);
  });

  it('treats an offered any-producer as every colour for this safe stop bound', () => {
    const p = player({
      hand: [card({ mana_cost: 'W' })], pool: {},
      battlefield: [card({ id: 7, name: 'Cavern', types: 'Land', produces: { colour: [0, 0, 0, 0, 0, 1], any: true } })],
    });
    expect(castableAfterTap(view(p), 0, tapDecision(7))).toBe(true);
  });

  it('uses the same offered Tundra tap for an instant-speed response', () => {
    const p = player({
      hand: [card({ mana_cost: 'U' })], pool: {},
      battlefield: [card({ id: 7, name: 'Tundra', types: 'Land Plains Island', produces: { colour: [1, 1, 0, 0, 0, 0], any: false } })],
    });
    expect(respondableAfterTap(view(p), 0, tapDecision(7))).toBe(true);
  });

  it('does not stop for a hand card that the offered taps cannot pay for', () => {
    const p = player({
      hand: [card({ mana_cost: 'U' })], pool: {},
      battlefield: [card({ id: 7, name: 'Mountain', types: 'Land Mountain', produces: { colour: [0, 0, 0, 1, 0, 0], any: false } })],
    });
    expect(castableAfterTap(view(p), 0, tapDecision(7))).toBe(false);
  });

  it('recognises a non-mana ability payable after an offered tap, but skips sacrifice and sick tap abilities', () => {
    const base = player({
      pool: {},
      battlefield: [
        card({ id: 7, name: 'Rock', types: 'Artifact', produces: { colour: [0, 0, 0, 0, 0, 1], any: false } }),
        card({ id: 8, name: 'Ability Rock', types: 'Artifact', ability_costs: ['1 T'] } as unknown as Partial<CardView>),
      ],
    });
    const d = tapDecision(7);
    expect(castableAfterTap(view(base), 0, d)).toBe(true);

    // ability_costs is the engine's effective offer-time projection: this
    // source's printed Cost$ is 2 T, reduced by Forensic Gadgeteer's live
    // ReduceCost$ 1 | Type$ Ability static to the projected 1 T below.
    const reduced = player({ ...base, battlefield: [base.battlefield[0], card({ id: 8, name: 'Printed 2 T ability', ability_costs: ['1 T'] } as unknown as Partial<CardView>)] });
    expect(castableAfterTap(view(reduced), 0, d)).toBe(true);

    const sacrifice = player({ ...base, battlefield: [base.battlefield[0], card({ id: 8, ability_costs: ['1 T Sac<1/Artifact>'] } as unknown as Partial<CardView>)] });
    expect(castableAfterTap(view(sacrifice), 0, d)).toBe(false);

    const sick = player({ ...base, battlefield: [base.battlefield[0], card({ id: 8, types: 'Creature', summon_sick: true, ability_costs: ['1 T'] } as unknown as Partial<CardView>)] });
    expect(castableAfterTap(view(sick), 0, d)).toBe(false);

    const hasty = player({ ...base, battlefield: [base.battlefield[0], card({ id: 8, types: 'Creature', summon_sick: true, keywords: ['Haste'], ability_costs: ['1 T'] } as unknown as Partial<CardView>)] });
    expect(castableAfterTap(view(hasty), 0, d)).toBe(true);

    const tapped = player({ ...base, battlefield: [base.battlefield[0], card({ id: 8, tapped: true, ability_costs: ['1 T'] } as unknown as Partial<CardView>)] });
    expect(castableAfterTap(view(tapped), 0, d)).toBe(false);
  });
});

describe('respondableAfterTap — the instant-speed response question (fb-20260914T114244Z)', () => {
  it('the report shape: Mana Leak ({1}{U}) in hand, pool empty, two untapped Islands (Available {U}) stops', () => {
    const p = player({ hand: [card({ name: 'Mana Leak', mana_cost: '1 U' })], available: { U: 2 } });
    expect(respondableAfterTap(view(p), 0)).toBe(true);
  });

  it('a SORCERY that becomes affordable after tapping does NOT count — a sorcery cannot respond to a resolving spell', () => {
    const p = player({ hand: [card({ types: 'Sorcery', mana_cost: '1 U' })], available: { U: 2 } });
    expect(respondableAfterTap(view(p), 0)).toBe(false);
  });

  it('a Flash creature counts (the exact spelling the engine projects, view/view.go ch.Keywords)', () => {
    const p = player({ hand: [card({ types: 'Creature Bear', mana_cost: '1 U', keywords: ['Flash'] })], available: { U: 2 } });
    expect(respondableAfterTap(view(p), 0)).toBe(true);
  });

  it('a creature without Flash does not count', () => {
    const p = player({ hand: [card({ types: 'Creature Bear', mana_cost: '1 U', keywords: ['Vigilance'] })], available: { U: 2 } });
    expect(respondableAfterTap(view(p), 0)).toBe(false);
  });

  it('an instant that is NOT affordable after tapping does not count (the money half still applies)', () => {
    const p = player({ hand: [card({ mana_cost: '4 U' })], available: { U: 2 } });
    expect(respondableAfterTap(view(p), 0)).toBe(false);
  });

  it('a land in hand never counts, even with an instant-speed type word absent', () => {
    const p = player({ hand: [card({ types: 'Land Island', mana_cost: '' })], available: { U: 2 } });
    expect(respondableAfterTap(view(p), 0)).toBe(false);
  });

  it('fails closed exactly like castableAfterTap: no readable hand, or a seat the view does not carry', () => {
    const withCard = player({ hand: [card({ mana_cost: 'U' })], available: { U: 1 } });
    expect(respondableAfterTap(view(withCard, false), 1)).toBe(false);
    const hidden = player({ hand: null as unknown as CardView[] });
    expect(respondableAfterTap(view(hidden), 0)).toBe(false);
  });

  it('cardRespondableAfterTap is per-card: timing and money are visible separately', () => {
    const p = player({ available: { U: 2 } });
    expect(cardRespondableAfterTap(p, card({ mana_cost: '1 U' }))).toBe(true);
    expect(cardRespondableAfterTap(p, card({ types: 'Sorcery', mana_cost: '1 U' }))).toBe(false);
    expect(cardRespondableAfterTap(p, card({ types: 'Sorcery', mana_cost: '1 U', keywords: ['Flash'] }))).toBe(true);
    expect(cardRespondableAfterTap(p, card({ mana_cost: '4 U' }))).toBe(false);
  });
});
