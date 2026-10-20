import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { getSeat, initSeatContext } from './seat';

// A private play-vs-bot table's match links carry no query. The join URL's
// seat is remembered per table so a reload or new tab on the replay link
// stays seated instead of falling back to a spectator 404.
function fakeStorage(): Storage {
  const items = new Map<string, string>();
  return {
    get length() { return items.size; },
    clear: () => items.clear(),
    getItem: (k: string) => items.get(k) ?? null,
    key: (i: number) => [...items.keys()][i] ?? null,
    removeItem: (k: string) => { items.delete(k); },
    setItem: (k: string, v: string) => { items.set(k, v); },
  };
}

describe('private table seat survives a hard reload', () => {
  beforeEach(() => vi.stubGlobal('localStorage', fakeStorage()));
  afterEach(() => {
    initSeatContext('');
    vi.unstubAllGlobals();
  });

  it('remembers the join URL seat per table and restores it on a bare match replay path', () => {
    initSeatContext('?seat=0&token=g1-token', '/t/g1');
    expect(getSeat()).toEqual({ seat: 0, token: 'g1-token' });

    initSeatContext('', '/t/g1/m/1');
    expect(getSeat()).toEqual({ seat: 0, token: 'g1-token' });
  });

  it('does not lend one table seat to another table', () => {
    initSeatContext('?seat=0&token=g1-token', '/t/g1');
    initSeatContext('', '/t/other/m/1');
    expect(getSeat()).toBeNull();
  });

  it('ignores a corrupt remembered seat', () => {
    localStorage.setItem('gorge.table.seat.g1', '{"seat":"zero","token":7}');
    initSeatContext('', '/t/g1/m/1');
    expect(getSeat()).toBeNull();
  });
});
