import { describe, expect, it } from 'vitest';
import { BotPickMarks } from './botpick.svelte';

describe('BotPickMarks: the paired bot\'s pick, read by the controls it names', () => {
  const pick = { seq: 7, options: [1], payment: 'pay-shock', objs: [41], chance: 65 };

  it('answers only for the decision it was made for', () => {
    const m = new BotPickMarks();
    expect(m.of(7)).toBeNull();
    m.set(pick);
    expect(m.of(7)).toBe(pick);
    expect(m.of(8)).toBeNull();
    expect(m.of(null)).toBeNull();
    expect(m.of(undefined)).toBeNull();
  });

  it('names its option, its planned cast and its card', () => {
    const m = new BotPickMarks();
    m.set(pick);
    expect(m.option(7, 1)).toBe(pick);
    expect(m.option(7, 0)).toBeNull();
    expect(m.option(8, 1)).toBeNull();
    expect(m.action(7, 'pay-shock')).toBe(pick);
    expect(m.action(7, 'pay-bolt')).toBeNull();
    expect(m.obj(41)).toBe(pick);
    expect(m.obj(42)).toBeNull();
  });

  it('marks nothing once cleared', () => {
    const m = new BotPickMarks();
    m.set(pick);
    m.set(null);
    expect(m.option(7, 1)).toBeNull();
    expect(m.obj(41)).toBeNull();
  });
});
