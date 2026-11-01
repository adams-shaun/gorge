import { describe, expect, it } from 'vitest';
import { cardView, costTokens, factsOf, mvColumns, openedBy, passDirection, passesFilter, seatLabel, tint, UNKNOWN_FACTS } from './draftcards';
import { card } from '../test/eventfixtures';

const F = (name: string) => factsOf(card('x', name));

describe('cost parsing (display only)', () => {
  it('reads the Forge spaced form the server sends, and the brace form alike', () => {
    expect(costTokens('{2}{W}{W}')).toEqual(['2', 'W', 'W']);
    expect(costTokens('2 W W')).toEqual(['2', 'W', 'W']);
    expect(costTokens('{X}{R}')).toEqual(['X', 'R']);
    expect(costTokens('{1}{W/U}')).toEqual(['1', 'W/U']);
    expect(costTokens('{1}{G} // {3}{U}')).toEqual(['1', 'G']); // a split card sorts by its front half
    expect(costTokens('')).toEqual([]);
    expect(costTokens('no cost')).toEqual([]);
  });
});

describe('factsOf reads the wire, never computes', () => {
  it('takes mana value, colours, cost and type line straight from the DraftCard', () => {
    expect(F('Serra Angel')).toEqual({ mv: 5, colours: ['W'], land: false, creature: true, cost: '3 W W', types: 'Creature Angel' });
    expect(F('Fusion Elemental').colours).toEqual(['W', 'U', 'B', 'R', 'G']);
  });
  it('the mana value is the server\'s even when the cost would say otherwise', () => {
    // An X spell, a split card: whatever the server says is the column.
    expect(factsOf({ ...card('x', 'Odd'), mana_cost: 'X R', mana_value: 7, type_line: 'Sorcery' }).mv).toBe(7);
    // Colours are the server's colour identity, not the cost's symbols.
    expect(factsOf({ ...card('x', 'Odd'), mana_cost: '2', mana_value: 2, colors: 'G', type_line: 'Artifact' }).colours).toEqual(['G']);
  });
  it('a known card with no mana_value (omitempty) is a zero; a land is a land', () => {
    expect(F('Ornithopter')).toMatchObject({ mv: 0, land: false, creature: true, cost: '' });
    expect(F('Evolving Wilds')).toMatchObject({ mv: 0, land: true });
  });
  it('a card the server sent no facts for is unknown', () => {
    expect(factsOf(card('x', 'Mystery Card'))).toBe(UNKNOWN_FACTS);
  });
  it('cardView shows the wire cost and type line', () => {
    expect(cardView(card('c1', 'Serra Angel'))).toMatchObject({ name: 'Serra Angel', types: 'Creature Angel', mana_cost: '3 W W' });
  });
});

describe('mvColumns', () => {
  it('buckets 0–1 … 6+ and Land from the wire facts, and adds "?" only for a card with none', () => {
    const cards = ['Serra Angel', 'Lightning Bolt', 'Shivan Dragon', 'Evolving Wilds', 'Ornithopter'].map((n, i) => card(`c${i}`, n));
    const cols = mvColumns(cards, factsOf);
    expect(cols.map((c) => c.label)).toEqual(['0–1', '2', '3', '4', '5', '6+', 'Land']);
    expect(cols[0].cards.map((c) => c.name)).toEqual(['Lightning Bolt', 'Ornithopter']);
    expect(cols[4].cards.map((c) => c.name)).toEqual(['Serra Angel']);
    expect(cols[5].cards.map((c) => c.name)).toEqual(['Shivan Dragon']);
    expect(cols[6].cards.map((c) => c.name)).toEqual(['Evolving Wilds']);
    const withUnknown = mvColumns([card('c1', 'Serra Angel'), card('c2', 'Mystery Card')], factsOf);
    expect(withUnknown.at(-1)).toMatchObject({ label: '?' });
    expect(withUnknown.at(-1)!.cards.map((c) => c.name)).toEqual(['Mystery Card']);
  });
});

describe('pod geometry', () => {
  it('packs 1 and 3 pass left, pack 2 right', () => {
    expect([1, 2, 3].map(passDirection)).toEqual(['left', 'right', 'left']);
  });
  it('names the seat that opened the pack at a pick', () => {
    expect(openedBy(0, 1, 1)).toBe(0); // your own pack
    expect(openedBy(0, 1, 2)).toBe(3); // left pass: it came from seat 3
    expect(openedBy(0, 1, 3)).toBe(2);
    expect(openedBy(0, 2, 2)).toBe(1); // right pass: it came from seat 1
    expect(openedBy(1, 3, 5)).toBe(1); // a full lap
  });
});

describe('filters and labels', () => {
  it('colour chips', () => {
    const gold = { ...UNKNOWN_FACTS, colours: ['W', 'U'] as const } as unknown as typeof UNKNOWN_FACTS;
    expect(passesFilter('all', UNKNOWN_FACTS)).toBe(true);
    expect(passesFilter('W', F('Serra Angel'))).toBe(true);
    expect(passesFilter('R', F('Serra Angel'))).toBe(false);
    expect(passesFilter('multi', gold)).toBe(true);
    expect(passesFilter('colourless', F('Evolving Wilds'))).toBe(true);
    expect(tint(gold)).toBe('M');
    expect(tint(F('Lightning Bolt'))).toBe('R');
    expect(tint(UNKNOWN_FACTS)).toBe('C');
  });
  it('seat labels', () => {
    expect(seatLabel(0, 'Ada', 0)).toBe('You');
    expect(seatLabel(2, 'Bot 3 (bot)', 0)).toBe('Seat 3 · Bot 3 (bot)');
    expect(seatLabel(1, '', null)).toBe('Seat 2');
  });
});
