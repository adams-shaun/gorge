import { describe, expect, it } from 'vitest';
import { turnStartsFrom } from './turns';

describe('turnStartsFrom', () => {
  it('lists the seq of every turn event', () => {
    const ev = (seq: number, kind: string) => ({ event: { seq, kind, player: 0 }, line: '' });
    expect(turnStartsFrom([ev(0, 'game_start'), ev(4, 'turn'), ev(9, 'tap'), ev(30, 'turn')])).toEqual([4, 30]);
  });
});
