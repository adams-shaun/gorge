import { describe, expect, it } from 'vitest';
import { parseLogLine, buildCardColour, cardColourKey, cardColourVar } from './logrender';

// A card colour resolver over a fixture view of cards, the same shape
// buildCardColour returns. It carries the view's exact card-name key set, so
// parseLogLine resolves objects by longest exact match rather than word shape.
const cardColour = buildCardColour([
  { name: 'Lightning Bolt', mana_cost: 'R' },
  { name: 'Counterspell', mana_cost: 'U U' },
  { name: 'Grizzly Bears', mana_cost: '1 G' },
  { name: 'Birds of Paradise', mana_cost: 'G' },
  { name: 'Island', mana_cost: undefined },
  { name: 'Rakdos Guildmage', mana_cost: 'B R' },
  { name: 'Storm Cauldron', mana_cost: undefined },
  { name: 'Jace, the Mind Sculptor', mana_cost: '2 U U' },
  { name: 'Sign in Blood', mana_cost: '1 B' },
  { name: 'Æther Vial', mana_cost: '1' },
]);

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
  it('splits a card reference whole, with its colour and its id carried separately', () => {
    const pieces = parseLogLine('Bob casts Lightning Bolt #12', { identities, cardColour });
    expect(pieces).toEqual([
      { kind: 'seat', text: 'Bob', colour: '#3b82f6' },
      { kind: 'text', text: ' casts ' },
      { kind: 'card', name: 'Lightning Bolt', id: '12', colour: 'R' },
    ]);
    // no text lost: re-joining the displayable text reproduces the line minus the ids
    expect(pieces.map((p) => (p.kind === 'card' ? p.name : (p as { text?: string }).text ?? '')).join('')).toBe('Bob casts Lightning Bolt');
  });

  it('colours a card by its mana identity via the buildCardColour lookup', () => {
    const pieces = parseLogLine('Bob casts Counterspell #3', { cardColour });
    expect(pieces[pieces.length - 1]).toMatchObject({ kind: 'card', name: 'Counterspell', colour: 'U' });
  });

  it('renders an ability token (the faceless "an ability #id" shape) as an ability, not a card', () => {
    const pieces = parseLogLine('an ability #12 triggers', { cardColour });
    expect(pieces[0]).toMatchObject({ kind: 'ability', name: 'an ability', id: '12' });
    expect(pieces.at(-1)).toEqual({ kind: 'text', text: ' triggers' });
  });

  it('recognises multiple objects in one line (attackers, blocks)', () => {
    const pieces = parseLogLine('Lightning Bolt #2 blocks Grizzly Bears #1', { cardColour });
    const cards = pieces.filter((p) => p.kind === 'card');
    expect(cards.map((p) => [p.name, p.id])).toEqual([['Lightning Bolt', '2'], ['Grizzly Bears', '1']]);
    expect(pieces[1]).toEqual({ kind: 'text', text: ' blocks ' });
  });

  it('a legendary comma name is matched whole, not split on the comma', () => {
    // fix-round-1 regression (F2): ", " is outside the old word-shape match,
    // so "Jace, the Mind Sculptor" used to render as "Mind Sculptor" with a
    // wrong hover title and a partial colour. The exact key set gets it whole.
    const pieces = parseLogLine('Ann casts Jace, the Mind Sculptor #4', { identities, cardColour });
    const card = pieces.find((p) => p.kind === 'card');
    expect(card).toMatchObject({ kind: 'card', name: 'Jace, the Mind Sculptor', id: '4', colour: 'U' });
    expect(pieces.filter((p) => p.kind === 'card')).toHaveLength(1);
    expect((card as { name: string }).name).toBe('Jace, the Mind Sculptor');
  });

  it('a connector outside the old allowlist ("in") is part of a whole match', () => {
    // fix-round-1 regression (F2): "Sign in Blood" used to split to "Blood".
    const pieces = parseLogLine('Ann casts Sign in Blood #2', { identities, cardColour });
    const card = pieces.find((p) => p.kind === 'card');
    expect(card).toMatchObject({ kind: 'card', name: 'Sign in Blood', id: '2', colour: 'B' });
  });

  it('a non-ASCII letter inside a name is matched whole (Æther Vial)', () => {
    const pieces = parseLogLine('Ann plays Æther Vial #8', { identities, cardColour });
    const card = pieces.find((p) => p.kind === 'card');
    expect(card).toMatchObject({ kind: 'card', name: 'Æther Vial', id: '8', colour: 'C' });
  });

  it('a colourless / land card resolves to C (colourless), not a made-up hue', () => {
    const pieces = parseLogLine('Bob plays Island #5', { cardColour });
    expect(pieces.at(-1)).toMatchObject({ kind: 'card', name: 'Island', colour: 'C' });
  });

  it('a multicolour card resolves to M (gold)', () => {
    expect(cardColour('Rakdos Guildmage')).toBe('M');
  });
});

describe('logrender id suppression (B3)', () => {
  it('a card name carries no "#id" in its display text — the id is a separate field for hover', () => {
    const pieces = parseLogLine('Bob casts Grizzly Bears #7', { cardColour });
    const card = pieces.find((p) => p.kind === 'card');
    expect(card).toMatchObject({ kind: 'card', name: 'Grizzly Bears', id: '7' });
    expect((card as { name: string }).name).not.toContain('#');
  });

  it('the seat-vs-card guard is preserved by construction: "Storm Cauldron #12" is one card, not the seat "Storm"', () => {
    const pieces = parseLogLine('Storm Cauldron #12 resolves', { identities, cardColour });
    // no seat piece at all — the whole object was consumed as a card token
    expect(pieces.some((p) => p.kind === 'seat')).toBe(false);
    expect(pieces[0]).toMatchObject({ kind: 'card', name: 'Storm Cauldron', id: '12', colour: 'C' });
  });

  it('a seat name that is a card-name prefix is not coloured inside that card token', () => {
    // fix-round-1 finding F4: with a seat literally named "Jace", the prefix
    // inside "Jace, the Mind Sculptor" must NOT be coloured as a player.
    const jaceSeat = [{ name: 'Jace', colour: '#eab308' }];
    const pieces = parseLogLine('Ann casts Jace, the Mind Sculptor #4', { identities: jaceSeat, cardColour });
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
    const pieces = parseLogLine('#77 dies', { identities, cardColour });
    expect(pieces).toEqual([{ kind: 'text', text: '#77' }, { kind: 'text', text: ' dies' }]);
    expect(pieces.some((p) => p.kind === 'ability')).toBe(false);
  });

  it('a bare id whose name is not in the view renders #id verbatim, without invention', () => {
    // the view does not know "Arwen Undómiel"; it must not become "an ability".
    const pieces = parseLogLine('Ann casts Arwen Undómiel #12', { identities, cardColour });
    expect(pieces.some((p) => p.kind === 'ability')).toBe(false);
    expect(pieces.some((p) => p.kind === 'card')).toBe(false);
    expect(pieces.some((p) => p.kind === 'text' && p.text === '#12')).toBe(true);
  });
});

describe('cardColourKey', () => {
  it('classifies mono, colourless, land and multicolour costs', () => {
    expect(cardColourKey('R')).toBe('R');
    expect(cardColourKey('1 G G')).toBe('G');
    expect(cardColourKey('W/U')).toBe('M'); // a hybrid contributes BOTH faces to the colour identity, so it is gold
    expect(cardColourKey('2')).toBe('C');   // generic alone is colourless
    expect(cardColourKey('')).toBe('C');
    expect(cardColourKey('B R')).toBe('M');
    expect(cardColourKey('W U B R G')).toBe('M');
  });
});

describe('cardColourVar', () => {
  it('maps every key to a CSS colour and null to null', () => {
    for (const k of ['W', 'U', 'B', 'R', 'G', 'C', 'M'] as const) {
      expect(cardColourVar(k)).toBeTruthy();
    }
    expect(cardColourVar(null)).toBeNull();
  });
});

describe('no-view fallback (word-shape regex)', () => {
  it('still renders a card reference when no key set is supplied', () => {
    const pieces = parseLogLine('Ann casts Lightning Bolt #12', {});
    expect(pieces.at(-1)).toMatchObject({ kind: 'card', name: 'Lightning Bolt', colour: null });
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

describe('buildCardColour carries its name key set', () => {
  it('attaches the exact card-name keys it was built over, so parseLogLine can exact-match', () => {
    const r = buildCardColour([{ name: 'Grizzly Bears', mana_cost: '1 G' }, { name: 'Island', mana_cost: undefined }]);
    expect(r.names).toEqual(expect.arrayContaining(['Grizzly Bears', 'Island']));
    expect(r('Grizzly Bears')).toBe('G');
    expect(r('Unknown')).toBeNull();
  });
});
