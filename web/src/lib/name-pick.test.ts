import { describe, expect, it } from 'vitest';
import type { Decision, Option } from '../protocol';
import { isNamePick, nameOptions, NAME_PICK_RENDER_LIMIT } from './name-pick';
const option = (index: number, label: string): Option => ({ index, kind: 'name', label, player: 1 });
const ask: Decision = { seq: 1, player: 1, kind: 'choose', prompt: 'Name a card', min: 1, max: 1, options: [option(0, 'Zulu'), option(1, 'Alpha'), option(2, 'Beta')] };
describe('name picker display helpers', () => {
  it('recognizes only exact name choices with no object', () => {
    expect(isNamePick(ask)).toBe(true);
    expect(isNamePick({ ...ask, options: [{ ...option(0, 'X'), obj: 9 }] })).toBe(false);
    expect(isNamePick({ ...ask, min: 0 })).toBe(false);
    expect(isNamePick({ ...ask, options: [{ index: 0, kind: 'mode', label: 'Mode', player: 1 }] })).toBe(false);
  });
  it('filters and sorts without changing wire indexes; render cap is 200', () => {
    expect(nameOptions(ask, '').map((o) => o.index)).toEqual([1, 2, 0]);
    expect(nameOptions(ask, 'ET').map((o) => o.index)).toEqual([2]);
    expect(nameOptions(ask, 'missing')).toEqual([]);
    expect(NAME_PICK_RENDER_LIMIT).toBe(200);
    const many = { ...ask, options: Array.from({ length: 205 }, (_, i) => option(i, `Card ${i}`)) };
    expect(nameOptions(many, '').slice(0, NAME_PICK_RENDER_LIMIT)).toHaveLength(200);
  });
});
