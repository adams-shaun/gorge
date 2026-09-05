import { describe, expect, it } from 'vitest';
import { manaSymbols } from './mana';

describe('manaSymbols', () => {
  it('splits Forge cost notation into symbols', () => {
    expect(manaSymbols('1 W')).toEqual(['1', 'W']);
    expect(manaSymbols('X G G')).toEqual(['X', 'G', 'G']);
    expect(manaSymbols('W/U W/U')).toEqual(['W/U', 'W/U']);
    expect(manaSymbols('')).toEqual([]);
    expect(manaSymbols('no cost')).toEqual([]);
  });
});
