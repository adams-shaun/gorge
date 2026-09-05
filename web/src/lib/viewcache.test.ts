import { describe, expect, it } from 'vitest';
import { ViewCache } from './viewcache';
import type { View } from '../protocol';

const v = (seq: number) => ({ turn: seq } as unknown as View);

describe('ViewCache', () => {
  it('dedupes in-flight loads and caches results', async () => {
    let calls = 0;
    const c = new ViewCache(async (seq) => { calls++; return v(seq); });
    const [a, b] = await Promise.all([c.get(5), c.get(5)]);
    expect(a).toBe(b);
    expect(calls).toBe(1);
    await c.get(5);
    expect(calls).toBe(1);
    expect(c.has(5)).toBe(true);
  });
  it('evicts the least recently used beyond cap', async () => {
    const c = new ViewCache(async (seq) => v(seq), 3);
    for (const s of [1, 2, 3]) await c.get(s);
    await c.get(1); // touch 1
    await c.get(4); // evicts 2
    expect(c.has(1)).toBe(true);
    expect(c.has(2)).toBe(false);
    expect(c.has(3)).toBe(true);
    expect(c.has(4)).toBe(true);
  });
  it('does not cache failures', async () => {
    let n = 0;
    const c = new ViewCache(async () => { if (n++ === 0) throw new Error('x'); return v(1); });
    await expect(c.get(1)).rejects.toThrow();
    await expect(c.get(1)).resolves.toBeTruthy();
  });
});
