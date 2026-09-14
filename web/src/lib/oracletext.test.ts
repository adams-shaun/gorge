import { describe, expect, it } from 'vitest';
import { oracleSegments } from './oracletext';

// The tokenizer's contract: split on Scryfall's braced notation, classify the
// mana family through lib/mana.ts's own classifier (never a second one), give
// the non-mana icons their own kinds, keep unrecognised tokens literal, and
// preserve every character between tokens verbatim — the paragraph's
// white-space: pre-wrap line breaks live in those runs.
describe('oracleSegments', () => {
  it('classifies a plain colour token as a mana segment via lib/mana.ts', () => {
    expect(oracleSegments('Deals 3 damage to any target.')).toEqual([
      { kind: 'text', text: 'Deals 3 damage to any target.' },
    ]);
    expect(oracleSegments('{B}: Target player loses 2 life.')).toEqual([
      { kind: 'mana', symbol: { kind: 'colour', colour: 'B', text: 'B' } },
      { kind: 'text', text: ': Target player loses 2 life.' },
    ]);
  });

  it('classifies hybrid, twobrid, phyrexian, snow, variable and digit pips', () => {
    expect(oracleSegments('{2/B}{W/U}{W/P}{S}{X}{3}')).toEqual([
      { kind: 'mana', symbol: { kind: 'twobrid', colour: 'B', text: '2/B' } },
      { kind: 'mana', symbol: { kind: 'hybrid', a: 'W', b: 'U', text: 'W/U' } },
      { kind: 'mana', symbol: { kind: 'phyrexian', colour: 'W', text: 'W/P' } },
      { kind: 'mana', symbol: { kind: 'snow', text: 'S' } },
      { kind: 'mana', symbol: { kind: 'variable', letter: 'X', text: 'X' } },
      { kind: 'mana', symbol: { kind: 'generic', value: 3, text: '3' } },
    ]);
  });

  it('gives the non-mana icons their own kinds', () => {
    expect(oracleSegments('{T}: Create a 1/1 token.')).toEqual([
      { kind: 'tap' },
      { kind: 'text', text: ': Create a 1/1 token.' },
    ]);
    expect(oracleSegments('{Q}: Untap target permanent.')).toEqual([{ kind: 'untap' }, { kind: 'text', text: ': Untap target permanent.' }]);
    expect(oracleSegments('Gain {E}.')).toEqual([
      { kind: 'text', text: 'Gain ' },
      { kind: 'energy' },
      { kind: 'text', text: '.' },
    ]);
  });

  it('keeps an unrecognised token as its literal braced text', () => {
    expect(oracleSegments('Roll {A} six-sided die.')).toEqual([
      { kind: 'text', text: 'Roll ' },
      { kind: 'text', text: '{A}' },
      { kind: 'text', text: ' six-sided die.' },
    ]);
    expect(oracleSegments('{TK}, {∞}, {HR} stay literal')).toEqual([
      { kind: 'text', text: '{TK}' },
      { kind: 'text', text: ', ' },
      { kind: 'text', text: '{∞}' },
      { kind: 'text', text: ', ' },
      { kind: 'text', text: '{HR}' },
      { kind: 'text', text: ' stay literal' },
    ]);
  });

  it('preserves every whitespace character between tokens verbatim', () => {
    const text = 'Flying, vigilance\nWhen {T}ing, if {W}\n was spent — keep\tit.';
    const segs = oracleSegments(text);
    expect(segs.map((s) => (s.kind === 'text' ? s.text : '')).join('')).toContain('\n');
    // The runs stitched together with the tokens re-braced reproduce the input.
    expect(
      segs
        .map((s) => {
          if (s.kind === 'text') return s.text;
          if (s.kind === 'mana') return `{${s.symbol.text}}`;
          if (s.kind === 'tap') return '{T}';
          if (s.kind === 'untap') return '{Q}';
          return '{E}';
        })
        .join(''),
    ).toBe(text);
  });

  it('answers empty text with no segments', () => {
    expect(oracleSegments('')).toEqual([]);
  });
});
