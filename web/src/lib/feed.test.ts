import { describe, expect, it } from 'vitest';
import { pushFeed, type FeedLine } from './feed';

const l = (table: string, seq: number, line = 'x'): FeedLine => ({ table, match: 1, seq, line });

describe('feed', () => {
  it('appends newest last, dedupes by table/match/seq, and caps', () => {
    let f: FeedLine[] = [];
    f = pushFeed(f, l('t1', 1, 'a'));
    f = pushFeed(f, l('t2', 1, 'b'));
    f = pushFeed(f, l('t1', 1, 'a again'));
    expect(f.map((x) => x.line)).toEqual(['a', 'b']);
    for (let i = 2; i < 500; i++) f = pushFeed(f, l('t1', i), 100);
    expect(f.length).toBe(100);
    expect(f[f.length - 1].seq).toBe(499);
  });
  it('drops empty lines', () => {
    expect(pushFeed([], l('t1', 1, ''))).toEqual([]);
  });
});
