import { describe, expect, it } from 'vitest';
import { defaultStops, loadStops, saveStops, stopsKey, toggleStop } from './stops';
import type { Stops } from './autopilot';

/** fakeStorage is a minimum Storage: a Map with getItem/setItem. Uses ServerResponse-like map semantics for determinism. */
function fakeStorage(): Storage {
  const store = new Map<string, string>();
  return {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, v),
    removeItem: (k: string) => void store.delete(k),
    clear: () => void store.clear(),
    key: (i: number) => [...store.keys()][i] ?? null,
    get length() {
      return store.size;
    },
  } as unknown as Storage;
}

/** throwingStorage simulates private-mode localStorage: every access throws, and it lives outside the module's guard, so a caller must not see a throw anyway. */
function throwingStorage(): Storage {
  return {
    getItem: () => { throw new Error('denied'); },
    setItem: () => { throw new Error('denied'); },
  } as unknown as Storage;
}

const SUM: Stops = { yours: new Set(['upkeep', 'draw']), opponents: new Set(['end']) };
const equal = (a: Stops, b: Stops) =>
  [...a.yours].sort().join() === [...b.yours].sort().join() && [...a.opponents].sort().join() === [...b.opponents].sort().join();

describe('stops storage', () => {
  it('returns the defaults when nothing is stored', () => {
    expect(equal(loadStops(fakeStorage(), 'a', 0), defaultStops())).toBe(true);
    expect(equal(loadStops(null, 'a', 0), defaultStops())).toBe(true);
  });

  it('returns the defaults when the stored value is corrupt', () => {
    const corrupt = [
      'oops',           // not JSON
      'null',           // JSON non-object
      '42',
      '"main1"',
      '{}',             // missing both sets
      '{"yours":[]}',   // missing opponents
      '{"yours":"main1","opponents":[]}',   // yours not an array
      '{"yours":["main1"],"opponents":["declare-blockers",1]}', // non-string element
    ];
    for (const raw of corrupt) {
      const st = fakeStorage();
      st.setItem(stopsKey('a', 0), raw);
      expect(equal(loadStops(st, 'a', 0), defaultStops()), `corrupt value ${JSON.stringify(raw)} must yield defaults`).toBe(true);
    }
  });

  it('round-trips a saved stop set', () => {
    const st = fakeStorage();
    saveStops(st, 'a', 0, SUM);
    expect(equal(loadStops(st, 'a', 0), SUM)).toBe(true);
    // rewriting replaces, it does not merge
    saveStops(st, 'a', 0, defaultStops());
    expect(equal(loadStops(st, 'a', 0), defaultStops())).toBe(true);
  });

  it('keys per table AND per seat, so two seats in one browser never share settings', () => {
    const st = fakeStorage();
    saveStops(st, 'a', 0, { yours: new Set(['main1']), opponents: new Set([]) });
    saveStops(st, 'a', 1, { yours: new Set(['main2']), opponents: new Set(['end']) });
    saveStops(st, 'b', 0, { yours: new Set(['draw']), opponents: new Set(['end']) });
    expect(loadStops(st, 'a', 0)).toEqual({ yours: new Set(['main1']), opponents: new Set([]) });
    expect(loadStops(st, 'a', 1)).toEqual({ yours: new Set(['main2']), opponents: new Set(['end']) });
    expect(loadStops(st, 'b', 0)).toEqual({ yours: new Set(['draw']), opponents: new Set(['end']) });
    expect(stopsKey('a', 0)).toBe('gorge.stop.a.0');
    expect(stopsKey('a', 1)).not.toBe(stopsKey('a', 0));
    expect(stopsKey('b', 0)).not.toBe(stopsKey('a', 0));
  });

  it('does not throw out of the module when storage throws', () => {
    const bad = throwingStorage();
    expect(() => loadStops(bad, 'a', 0)).not.toThrow();
    expect(equal(loadStops(bad, 'a', 0), defaultStops())).toBe(true);
    expect(() => saveStops(bad, 'a', 0, SUM)).not.toThrow();
  });

  it('defaultStops returns fresh sets every call', () => {
    const first = defaultStops();
    first.yours.add('end');
    expect(defaultStops().yours.has('end')).toBe(false);
    expect(defaultStops()).toEqual({ yours: new Set(['main1', 'declare-attackers', 'main2']), opponents: new Set(['declare-attackers', 'declare-blockers']) });
  });
});

describe('toggleStop', () => {
  const empty = (): Stops => ({ yours: new Set(), opponents: new Set() });

  it('adds and removes a step on the named side only', () => {
    const on = toggleStop(empty(), 'yours', 'main1');
    expect([...on.yours]).toEqual(['main1']);
    expect([...on.opponents]).toEqual([]);
    const off = toggleStop(on, 'yours', 'main1');
    expect([...off.yours]).toEqual([]);
  });

  it('the two sides are independent: the same step can be set on one and not the other', () => {
    const theirs = toggleStop(empty(), 'opponents', 'declare-blockers');
    expect(theirs.opponents.has('declare-blockers')).toBe(true);
    expect(theirs.yours.has('declare-blockers')).toBe(false);
  });

  it('returns a new value and never mutates the one it was given', () => {
    const before = empty();
    const after = toggleStop(before, 'yours', 'end');
    expect(after).not.toBe(before);
    expect(before.yours.size).toBe(0);
  });

  it('refuses untap and cleanup, returning the value unchanged', () => {
    const before = empty();
    expect(toggleStop(before, 'yours', 'untap')).toBe(before);
    expect(toggleStop(before, 'opponents', 'cleanup')).toBe(before);
  });
});
