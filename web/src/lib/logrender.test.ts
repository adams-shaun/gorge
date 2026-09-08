import { describe, expect, it } from 'vitest';
import { parseLogLine, buildCardOwnerColour, type CardOwnerColour } from './logrender';

// fXC is the SEAT_COLOURS palette (lib/colours.ts), the same palette that
// colours a seat's name in the line. A card's colour is its OWNER seat's
// colour (lc1), so the fixture below maps each object id to an owner seat and
// resolves it through this palette.
const PALETTE = ['#e5484d', '#3b82f6', '#22c55e', '#eab308'] as const;
const seatColourOf = (seat: number): string => PALETTE[seat] ?? '#777777';

// A card colour resolver over a fixture view of cards, the same shape
// buildCardOwnerColour returns. It maps by OBJECT ID to the owner seat's
// colour (so two copies of the same card owned by different seats differ, the
// case name-keying could never express), and carries the view's exact
// card-name key set, so parseLogLine resolves objects by longest exact match
// rather than word shape.
const ownerColour: CardOwnerColour = buildCardOwnerColour([
  { id: 12, name: 'Lightning Bolt', owner: 1 },
  { id: 3, name: 'Counterspell', owner: 0 },
  { id: 2, name: 'Lightning Bolt', owner: 1 },
  { id: 1, name: 'Grizzly Bears', owner: 0 },
  { id: 7, name: 'Grizzly Bears', owner: 0 },
  { id: 4, name: 'Jace, the Mind Sculptor', owner: 0 },
  { id: 9, name: 'Birds of Paradise', owner: 1 },
  { id: 5, name: 'Island', owner: 0 },
  { id: 40, name: 'Rakdos Guildmage', owner: 2 },
  { id: 13, name: 'Storm Cauldron', owner: 3 },
  { id: 8, name: 'Æther Vial', owner: 1 },
  { id: 6, name: 'Sign in Blood', owner: 0 },
], seatColourOf);

const identities = [
  { name: 'Ann', colour: '#e5484d' },
  { name: 'Bob', colour: '#3b82f6' },
  { name: 'Storm', colour: '#eab308' },
];

describe('logrender mana symbols (B1)', () => {
  it('renders mono mana as a mana piece with the inner token, not the braces', () => {
    const pieces = parseLogLine('Ann adds {G}{G}', { identities });
    expect(pieces).toEqual([
      { kind: 'seat', text: 'Ann', colour: '#e5484d' },
      { kind: 'text', text: ' adds ' },
      { kind: 'mana', token: 'G' },
      { kind: 'mana', token: 'G' },
    ]);
  });

  it('recognises a hybrid symbol as one token to feed the hybrid pip', () => {
    const pieces = parseLogLine('Ann adds {W/U}', { identities });
    expect(pieces.filter((p) => p.kind === 'mana')).toEqual([{ kind: 'mana', token: 'W/U' }]);
  });

  it('renders generic and variable pips too', () => {
    const pieces = parseLogLine('Ann spends {2}{X}', {});
    expect(pieces.filter((p) => p.kind === 'mana').map((p) => p.token)).toEqual(['2', 'X']);
  });
});

describe('logrender object references (B2)', () => {
  it('splits a card reference whole, with its owner-seat colour and its id carried separately', () => {
    const pieces = parseLogLine('Bob casts Lightning Bolt #12', { identities, cardColour: ownerColour });
    expect(pieces).toEqual([
      { kind: 'seat', text: 'Bob', colour: '#3b82f6' },
      { kind: 'text', text: ' casts ' },
      // Lightning Bolt's id 12 is owned by seat 1 (Bob), so it shares Bob's colour
      { kind: 'card', name: 'Lightning Bolt', id: '12', colour: '#3b82f6' },
    ]);
    // no text lost: re-joining the displayable text reproduces the line minus the ids
    expect(pieces.map((p) => (p.kind === 'card' ? p.name : (p as { text?: string }).text ?? '')).join('')).toBe('Bob casts Lightning Bolt');
  });

  it('colours a card by its OWNER seat colour via the buildCardOwnerColour lookup, not its mana identity', () => {
    // Counterspell (id 3) is owned by seat 0 (Ann) => Ann's colour, not its blue mana.
    const pieces = parseLogLine('Bob casts Counterspell #3', { identities, cardColour: ownerColour });
    expect(pieces[pieces.length - 1]).toMatchObject({ kind: 'card', name: 'Counterspell', colour: '#e5484d' });
  });

  it('two seats owning the same card name render in different colours (the lc1 id-keying case)', () => {
    // One Island owned by seat 2, one owned by seat 3, in one line. Name-keying
    // could never express two colours for one printed name.
    const twoIslands = buildCardOwnerColour([
      { id: 20, name: 'Island', owner: 2 },
      { id: 21, name: 'Island', owner: 3 },
    ], seatColourOf);
    const pieces = parseLogLine('Island #20 taps and Island #21 taps', { cardColour: twoIslands });
    const cards = pieces.filter((p) => p.kind === 'card');
    expect(cards.map((p) => [p.name, p.colour])).toEqual([
      ['Island', '#22c55e'],
      ['Island', '#eab308'],
    ]);
  });

  it("renders a bare \"an ability #id\" token (fallback) as an ability, not a card", () => {
    const pieces = parseLogLine('an ability #12 triggers', { cardColour: ownerColour });
    expect(pieces[0]).toMatchObject({ kind: 'ability', name: 'an ability', id: '12' });
    expect(pieces.at(-1)).toEqual({ kind: 'text', text: ' triggers' });
  });

  it("renders a source-named ability \"<Source>'s ability #id\" as an ability naming its source", () => {
    // lc1: describe.go now names a faceless ability's source. The client must
    // recognise the possessive shape and keep the "an ability" invention out.
    const pieces = parseLogLine("Goblin Balloon Brigade's ability #217 resolves", { cardColour: ownerColour });
    expect(pieces[0]).toMatchObject({ kind: 'ability', name: "Goblin Balloon Brigade's ability", id: '217' });
    expect(pieces.at(-1)).toEqual({ kind: 'text', text: ' resolves' });
    // no invented words and no card classification
    expect(pieces.some((p) => p.kind === 'card')).toBe(false);
  });

  it("keeps just the source possessive for a mid-line ability reference", () => {
    // StackCopy-style line: the ability is not the sentence subject, so the
    // parser must not swallow the words before the possessive.
    const pieces = parseLogLine("Ann copies Goblin Balloon Brigade's ability #217", { identities, cardColour: ownerColour });
    expect(pieces[0]).toEqual({ kind: 'seat', text: 'Ann', colour: '#e5484d' });
    expect(pieces[1]).toEqual({ kind: 'text', text: ' copies ' });
    expect(pieces[2]).toMatchObject({ kind: 'ability', name: "Goblin Balloon Brigade's ability", id: '217' });
    expect(pieces.some((p) => p.kind === 'card')).toBe(false);
  });

  it('recognises multiple objects in one line (attackers, blocks)', () => {
    const pieces = parseLogLine('Lightning Bolt #2 blocks Grizzly Bears #1', { cardColour: ownerColour });
    const cards = pieces.filter((p) => p.kind === 'card');
    expect(cards.map((p) => [p.name, p.id])).toEqual([['Lightning Bolt', '2'], ['Grizzly Bears', '1']]);
    expect(pieces[1]).toEqual({ kind: 'text', text: ' blocks ' });
  });

  it('a legendary comma name is matched whole, not split on the comma', () => {
    // fix-round-1 regression (F2): ", " is outside the old word-shape match,
    // so "Jace, the Mind Sculptor" used to render as "Mind Sculptor" with a
    // wrong hover title and a partial colour. The exact key set gets it whole.
    const pieces = parseLogLine('Ann casts Jace, the Mind Sculptor #4', { identities, cardColour: ownerColour });
    const card = pieces.find((p) => p.kind === 'card');
    expect(card).toMatchObject({ kind: 'card', name: 'Jace, the Mind Sculptor', id: '4', colour: '#e5484d' });
    expect(pieces.filter((p) => p.kind === 'card')).toHaveLength(1);
    expect((card as { name: string }).name).toBe('Jace, the Mind Sculptor');
  });

  it('a connector outside the old allowlist ("in") is part of a whole match', () => {
    // fix-round-1 regression (F2): "Sign in Blood" used to split to "Blood".
    const pieces = parseLogLine('Ann casts Sign in Blood #6', { identities, cardColour: ownerColour });
    const card = pieces.find((p) => p.kind === 'card');
    expect(card).toMatchObject({ kind: 'card', name: 'Sign in Blood', id: '6', colour: '#e5484d' });
  });

  it('a non-ASCII letter inside a name is matched whole (Æther Vial)', () => {
    const pieces = parseLogLine('Ann plays Æther Vial #8', { identities, cardColour: ownerColour });
    const card = pieces.find((p) => p.kind === 'card');
    expect(card).toMatchObject({ kind: 'card', name: 'Æther Vial', id: '8', colour: '#3b82f6' });
  });

  it("a card the view resolves by id takes its owner's seat colour (Island, id 5, seat 0)", () => {
    const pieces = parseLogLine('Bob plays Island #5', { cardColour: ownerColour });
    expect(pieces.at(-1)).toMatchObject({ kind: 'card', name: 'Island', colour: '#e5484d' });
  });

  it('an id whose owner is not a known identity still takes that seat palette colour', () => {
    // Rakdos Guildmage (id 40) is owned by seat 2, which is not in `identities`;
    // it still takes seat 2's palette colour (never a guess, and never a mana
    // identity).
    expect(ownerColour(40)).toBe('#22c55e');
  });
});

describe('logrender id suppression (B3)', () => {
  it('a card name carries no "#id" in its display text — the id is a separate field for hover', () => {
    const pieces = parseLogLine('Bob casts Grizzly Bears #7', { cardColour: ownerColour });
    const card = pieces.find((p) => p.kind === 'card');
    expect(card).toMatchObject({ kind: 'card', name: 'Grizzly Bears', id: '7' });
    expect((card as { name: string }).name).not.toContain('#');
  });

  it('the seat-vs-card guard is preserved by construction: "Storm Cauldron #13" is one card, not the seat "Storm"', () => {
    const pieces = parseLogLine('Storm Cauldron #13 resolves', { identities, cardColour: ownerColour });
    // no seat piece at all — the whole object was consumed as a card token
    expect(pieces.some((p) => p.kind === 'seat')).toBe(false);
    expect(pieces[0]).toMatchObject({ kind: 'card', name: 'Storm Cauldron', id: '13', colour: '#eab308' });
  });

  it('a seat name that is a card-name prefix is not coloured inside that card token', () => {
    // fix-round-1 finding F4: with a seat literally named "Jace", the prefix
    // inside "Jace, the Mind Sculptor" must NOT be coloured as a player.
    const jaceSeat = [{ name: 'Jace', colour: '#eab308' }];
    const pieces = parseLogLine('Ann casts Jace, the Mind Sculptor #4', { identities: jaceSeat, cardColour: ownerColour });
    expect(pieces.some((p) => p.kind === 'seat')).toBe(false);
    expect(pieces.find((p) => p.kind === 'card')).toMatchObject({ kind: 'card', name: 'Jace, the Mind Sculptor' });
  });
});

describe('logrender identity', () => {
  it('no identities and no colour resolver: pieces carry structure but colour and seat stay neutral', () => {
    const pieces = parseLogLine('Ann casts Lightning Bolt #12', {});
    expect(pieces).toEqual([
      { kind: 'text', text: 'Ann casts ' },
      { kind: 'card', name: 'Lightning Bolt', id: '12', colour: null },
    ]);
  });

  it('an empty line yields no pieces', () => {
    expect(parseLogLine('', {})).toEqual([]);
  });

  it('an unresolvable object id renders the honest bare id, not the words "an ability"', () => {
    // fix-round-1 finding F1: describe.go's bare "#id" means an object the
    // game cannot resolve; the parser must invent no name for it.
    const pieces = parseLogLine('#77 dies', { identities, cardColour: ownerColour });
    expect(pieces).toEqual([{ kind: 'text', text: '#77' }, { kind: 'text', text: ' dies' }]);
    expect(pieces.some((p) => p.kind === 'ability')).toBe(false);
  });

  it('a bare id whose name is not in the view renders #id verbatim, without invention', () => {
    // the view does not know "Arwen Undómiel"; it must not become "an ability".
    const pieces = parseLogLine('Ann casts Arwen Undómiel #12', { identities, cardColour: ownerColour });
    expect(pieces.some((p) => p.kind === 'ability')).toBe(false);
    expect(pieces.some((p) => p.kind === 'card')).toBe(false);
    expect(pieces.some((p) => p.kind === 'text' && p.text === '#12')).toBe(true);
  });

  it('an id absent from the view resolves to null colour (uncoloured), never guessed', () => {
    // id 999 is not in the fixture view: the card still parses by name, but
    // its colour is null — never a made-up hue (the B2 promise, on ids).
    const pieces = parseLogLine('Bob casts Lightning Bolt #999', { cardColour: ownerColour });
    expect(pieces.at(-1)).toMatchObject({ kind: 'card', name: 'Lightning Bolt', id: '999', colour: null });
  });
});

describe('no-view fallback (word-shape regex)', () => {
  it('still renders a card reference when no key set is supplied', () => {
    const pieces = parseLogLine('Ann casts Lightning Bolt #12', {});
    expect(pieces.at(-1)).toMatchObject({ kind: 'card', name: 'Lightning Bolt', colour: null });
  });

  it("recognises the source-named ability shape in the fallback too", () => {
    const pieces = parseLogLine("Goblin Balloon Brigade's ability #217 resolves", {});
    expect(pieces[0]).toMatchObject({ kind: 'ability', name: "Goblin Balloon Brigade's ability", id: '217' });
    expect(pieces.some((p) => p.kind === 'card')).toBe(false);
  });

  it('resolves the finding\'s named comma/connector/unicode names whole, even without a view (F2)', () => {
    // The transcript uses the view's exact key set; this is the no-view
    // fallback (the lobby rail, which has no match view). The improved
    // word-shape regex must still not split these on the comma, a connector
    // like "in", or a non-ASCII letter.
    const names = [
      'Avacyn, Angel of Hope', 'Ghalta, Primal Hunger', 'Giada, Font of Hope',
      'Goreclaw, Terror of Qal Sisma', 'Jace, the Mind Sculptor', 'Karn, the Great Creator',
      'Lathliss, Dragon Queen', 'Linvala, Keeper of Silence', 'Ryusei, the Falling Star',
      'Sai, Master Thopterist', 'Thalia, Guardian of Thraben', 'Ulamog, the Ceaseless Hunger',
      'Sign in Blood', 'Æther Vial',
    ];
    for (const n of names) {
      const pieces = parseLogLine(`Ann casts ${n} #4`, {});
      const card = pieces.find((p) => p.kind === 'card');
      expect({ name: n, got: (card && 'name' in card ? card.name : null) }).toEqual({ name: n, got: n });
    }
  });

  it('a bare id in the fallback also renders #id verbatim, not "an ability" (F1)', () => {
    const pieces = parseLogLine('#77 dies', {});
    expect(pieces.some((p) => p.kind === 'text' && p.text === '#77')).toBe(true);
    expect(pieces.some((p) => p.kind === 'ability')).toBe(false);
  });
});

describe('buildCardOwnerColour carries its id and name keys', () => {
  it('attaches the exact card-name keys it was built over, so parseLogLine can exact-match', () => {
    const r = buildCardOwnerColour([
      { id: 1, name: 'Grizzly Bears', owner: 1 },
      { id: 2, name: 'Island', owner: 0 },
    ], seatColourOf);
    expect(r.names).toEqual(expect.arrayContaining(['Grizzly Bears', 'Island']));
    expect(r(1)).toBe('#3b82f6'); // owner seat 1
    expect(r(2)).toBe('#e5484d'); // owner seat 0
    expect(r(999)).toBeNull();   // id absent from the view
  });

  it('skips a null card entry and a card without an id, so it does not crash on defended zone data', () => {
    const r = buildCardOwnerColour([null, { id: 4, name: 'Island', owner: 2 }, { name: 'no id', owner: 0 }], seatColourOf);
    expect(r(4)).toBe('#22c55e');
    expect(r.names).toEqual(['Island']);
    expect(() => r(999)).not.toThrow();
  });
});
