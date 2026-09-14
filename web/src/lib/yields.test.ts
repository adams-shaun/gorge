import { describe, expect, it } from 'vitest';
import { loadYields, saveYields, stackYieldKey, yieldKey, yieldsStorageKey } from './yields';

/**
 * The always-yield store (prio6): keys are `${controller}:${name}:${text}`,
 * scope is per GAME (table id + match number — review r2: table-only let a
 * yield from game N auto-pass in game N+1), the layers are memory +
 * sessionStorage, and a browser that refuses site data keeps the memory
 * copy. The seat panel's reactive yieldList is seeded from loadYields and
 * written through saveYields — what these tests hold down is the store's
 * own contract: the key format, the per-game scoping, the memory-cache
 * identity within a tab, and the round trip through a (fake) sessionStorage.
 */

/** fakeStorage is a minimal in-memory Storage, the SSR-test stand-in. */
function fakeStorage(): Storage {
  const map = new Map<string, string>();
  return {
    get length() { return map.size; },
    clear: () => map.clear(),
    getItem: (k: string) => map.get(k) ?? null,
    key: (i: number) => [...map.keys()][i] ?? null,
    removeItem: (k: string) => { map.delete(k); },
    setItem: (k: string, v: string) => { map.set(k, v); },
  };
}

describe('yield keys', () => {
  it('yieldKey is controller:name:text — the brief\u2019s exact format', () => {
    expect(yieldKey(1, 'Blood Artist', 'Whenever a creature dies…')).toBe(
      '1:Blood Artist:Whenever a creature dies…',
    );
  });

  it('stackYieldKey is yieldKey over the wire StackView\u2019s three rendered facts', () => {
    expect(stackYieldKey({ controller: 2, name: 'Soul Warden', text: 'gain 1 life' })).toBe(
      '2:Soul Warden:gain 1 life',
    );
    expect(stackYieldKey({ controller: 2, name: 'Soul Warden', text: 'gain 1 life' })).toBe(
      yieldKey(2, 'Soul Warden', 'gain 1 life'),
    );
  });

  it('the key excludes the object id, so a yield survives the source leaving and returning', () => {
    const a = stackYieldKey({ controller: 1, name: 'Blood Artist', text: 'drain' });
    const b = stackYieldKey({ controller: 1, name: 'Blood Artist', text: 'drain' });
    expect(a).toBe(b); // different objects (ids), same key by construction
  });
});

describe('the per-game store', () => {
  it('is scoped by table id: two tables hold independent sets', () => {
    const storage = fakeStorage();
    saveYields('table-a', 1, new Set(['1:A:x']), storage);
    saveYields('table-b', 1, new Set(['2:B:y']), storage);
    expect([...loadYields('table-a', 1, storage)]).toEqual(['1:A:x']);
    expect([...loadYields('table-b', 1, storage)]).toEqual(['2:B:y']);
    expect(yieldsStorageKey('table-a', 1)).toBe('gorge.yields.table-a:1');
    expect(yieldsStorageKey('table-b', 1)).toBe('gorge.yields.table-b:1');
  });

  it('is scoped by match: a new match on the same table reads EMPTY (review r2 — a game N yield never auto-passes in game N+1)', () => {
    const storage = fakeStorage();
    saveYields('table-a', 1, new Set(['1:A:x']), storage);
    expect([...loadYields('table-a', 2, storage)]).toEqual([]);
    // …and the new match's set is independent of the old one's.
    saveYields('table-a', 2, new Set(['3:C:z']), storage);
    expect([...loadYields('table-a', 1, storage)]).toEqual(['1:A:x']);
    expect([...loadYields('table-a', 2, storage)]).toEqual(['3:C:z']);
  });

  it('round-trips through sessionStorage (a fresh load with a cold memory cache reads the mirror)', () => {
    const storage = fakeStorage();
    saveYields('t1', 1, new Set(['1:Blood Artist:drain']), storage);
    // A fresh module-level memory would not exist in a new tab; simulate one
    // by loading a scope this tab has never saved — the value it reads
    // can only come from the sessionStorage mirror.
    const other = fakeStorage();
    other.setItem('gorge.yields.t1b:1', storage.getItem('gorge.yields.t1:1')!);
    expect([...loadYields('t1b', 1, other)]).toEqual(['1:Blood Artist:drain']);
  });

  it('a corrupt stored value yields an empty set, not a throw', () => {
    const storage = fakeStorage();
    storage.setItem(yieldsStorageKey('t2', 1), '{not json');
    expect([...loadYields('t2', 1, storage)]).toEqual([]);
    storage.setItem(yieldsStorageKey('t3', 1), '["ok", 7]');
    expect([...loadYields('t3', 1, storage)]).toEqual([]);
  });

  it('null storage never throws: memory is the copy (a browser refusing site data still yields)', () => {
    saveYields('t4', 1, new Set(['1:A:x']), null);
    expect([...loadYields('t4', 1, null)]).toEqual(['1:A:x']);
  });

  it('the memory cache is authoritative within a tab: a later load returns the latest save', () => {
    const storage = fakeStorage();
    saveYields('t5', 1, new Set(['1:A:x']), storage);
    expect([...loadYields('t5', 1, storage)]).toEqual(['1:A:x']);
    saveYields('t5', 1, new Set<string>(), storage);
    expect([...loadYields('t5', 1, storage)]).toEqual([]);
  });
});
