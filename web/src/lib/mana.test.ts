import { describe, expect, it } from 'vitest';
import { manaSymbols } from './mana';
import { CORPUS_MANA_SYMBOLS } from './corpus-mana-symbols';

/** shape helper: the fields a renderer cares about, minus boilerplate text. */
function shape(s: ReturnType<typeof manaSymbols>[number]) {
  const { text, ...rest } = s;
  void text;
  return rest;
}

describe('manaSymbols', () => {
  it('parses generic numerics', () => {
    expect(shape(manaSymbols('10 2')[0])).toEqual({ kind: 'generic', value: 10 });
    expect(shape(manaSymbols('10 2')[1])).toEqual({ kind: 'generic', value: 2 });
  });

  it('parses variable pips X, Y and Z', () => {
    expect(shape(manaSymbols('X Y Z')[0])).toEqual({ kind: 'variable', letter: 'X' });
    expect(shape(manaSymbols('X Y Z')[1])).toEqual({ kind: 'variable', letter: 'Y' });
    expect(shape(manaSymbols('X Y Z')[2])).toEqual({ kind: 'variable', letter: 'Z' });
  });

  it('parses single colours, colourless and snow', () => {
    expect(shape(manaSymbols('W')[0])).toEqual({ kind: 'colour', colour: 'W' });
    expect(shape(manaSymbols('C')[0])).toEqual({ kind: 'colourless' });
    expect(shape(manaSymbols('S')[0])).toEqual({ kind: 'snow' });
  });

  it('parses two-colour hybrid in both notations', () => {
    expect(shape(manaSymbols('W/U')[0])).toEqual({ kind: 'hybrid', a: 'W', b: 'U' });
    expect(shape(manaSymbols('WU')[0])).toEqual({ kind: 'hybrid', a: 'W', b: 'U' });
    expect(shape(manaSymbols('RG')[0])).toEqual({ kind: 'hybrid', a: 'R', b: 'G' });
  });

  it('parses colourless hybrids (corpus: Ulalek CW CU CB CR CG)', () => {
    expect(shape(manaSymbols('CW CG')[1])).toEqual({ kind: 'hybrid', a: 'C', b: 'G' });
    expect(shape(manaSymbols('CW CG')[0])).toEqual({ kind: 'hybrid', a: 'C', b: 'W' });
  });

  it('parses twobrid in both notations', () => {
    expect(shape(manaSymbols('2W')[0])).toEqual({ kind: 'twobrid', colour: 'W' });
    expect(shape(manaSymbols('2/B')[0])).toEqual({ kind: 'twobrid', colour: 'B' });
    expect(shape(manaSymbols('2R')[0])).toEqual({ kind: 'twobrid', colour: 'R' });
  });

  it('parses Phyrexian single-colour pips', () => {
    expect(shape(manaSymbols('WP')[0])).toEqual({ kind: 'phyrexian', colour: 'W' });
    expect(shape(manaSymbols('UP')[0])).toEqual({ kind: 'phyrexian', colour: 'U' });
  });

  it('parses hybrid-Phyrexian pips with P trailing and leading', () => {
    expect(shape(manaSymbols('GUP')[0])).toEqual({ kind: 'phyrexianHybrid', a: 'G', b: 'U' });
    expect(shape(manaSymbols('RWP')[0])).toEqual({ kind: 'phyrexianHybrid', a: 'R', b: 'W' });
    // the corpus writes Lukka's {R/G/P} with P leading.
    expect(shape(manaSymbols('PRG')[0])).toEqual({ kind: 'phyrexianHybrid', a: 'R', b: 'G' });
  });

  it('keeps the authored spelling as text', () => {
    expect(manaSymbols('W/U 2W GUP')[0].text).toBe('W/U');
    expect(manaSymbols('W/U 2W GUP')[1].text).toBe('2W');
    expect(manaSymbols('W/U 2W GUP')[2].text).toBe('GUP');
  });

  it('treats the whole-string "no cost" marker as an empty cost', () => {
    expect(manaSymbols('no cost')).toEqual([]);
    expect(manaSymbols('No CoSt')).toEqual([]);
  });

  it('returns [] for empty and blank input', () => {
    expect(manaSymbols('')).toEqual([]);
    expect(manaSymbols('   ')).toEqual([]);
    expect(manaSymbols(undefined as unknown as string)).toEqual([]);
  });

  describe('unknown pips guard', () => {
    it('renders an unknown pip instead of blanking the whole cost', () => {
      const s = manaSymbols('W 2W NOPE G');
      expect(s).toHaveLength(4);
      // the known pips survive, in order, and the unknown one is kept visibly.
      expect(shape(s[0])).toEqual({ kind: 'colour', colour: 'W' });
      expect(shape(s[1])).toEqual({ kind: 'twobrid', colour: 'W' });
      expect(shape(s[3])).toEqual({ kind: 'colour', colour: 'G' });
      expect(s[2].kind).toBe('unknown');
      expect(s[2].text).toBe('NOPE');
    });
  });

  describe('corpus coverage ratchet', () => {
    it(`every one of the ${CORPUS_MANA_SYMBOLS.length} corpus symbols parses to a non-empty pip`, () => {
      const failures: string[] = [];
      for (const sym of CORPUS_MANA_SYMBOLS) {
        const parsed = manaSymbols(sym);
        if (parsed.length !== 1 || parsed[0].kind === 'unknown') {
          failures.push(`${sym} -> ${JSON.stringify(parsed)}`);
        }
      }
      expect(failures, failures.join('\n')).toEqual([]);
    });

    it('every corpus symbol has the expected structured meaning', () => {
      const hybrid = new Set(['BG', 'BR', 'CB', 'CG', 'CR', 'CU', 'CW', 'GU', 'GW', 'RG', 'RW', 'UB', 'UR', 'WB', 'WU']);
      const phyrexianHybrid = new Set(['GUP', 'GWP', 'PRG', 'RWP']);
      const twobrid = new Set(['2/B', '2/G', '2/R', '2B', '2G', '2R', '2U', '2W']);
      const phyrexian = new Set(['BP', 'GP', 'RP', 'UP', 'WP']);
      for (const sym of CORPUS_MANA_SYMBOLS) {
        const [p] = manaSymbols(sym);
        let expected: string;
        if (/^\d+$/.test(sym)) expected = 'generic';
        else if (/^[WUBRG]$/.test(sym)) expected = 'colour';
        else if (sym === 'C') expected = 'colourless';
        else if (sym === 'S') expected = 'snow';
        else if (sym === 'X') expected = 'variable';
        else if (hybrid.has(sym)) expected = 'hybrid';
        else if (twobrid.has(sym)) expected = 'twobrid';
        else if (phyrexian.has(sym)) expected = 'phyrexian';
        else if (phyrexianHybrid.has(sym)) expected = 'phyrexianHybrid';
        else expected = 'unknown';
        expect(p.kind, `${sym} should be ${expected}`).toBe(expected);
      }
    });
  });
});
