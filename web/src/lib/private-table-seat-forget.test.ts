import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// The remembered table seat is restored ONLY on a match replay path, and a
// seat the server refuses (401/403 after a redeploy) is forgotten before the
// redirect, so it cannot bounce every later load back to the lobby.
const { fetchViewMock, fetchEventsMock, fetchMatchesMock, ApiErrorMock } = vi.hoisted(() => {
  class ApiErrorMock extends Error {
    constructor(public status: number) { super(`HTTP ${status}`); }
  }
  return {
    fetchViewMock: vi.fn(),
    fetchEventsMock: vi.fn(),
    fetchMatchesMock: vi.fn(),
    ApiErrorMock,
  };
});
vi.mock('./api', () => ({ ApiError: ApiErrorMock, fetchView: fetchViewMock, fetchEvents: fetchEventsMock, fetchMatches: fetchMatchesMock }));

import { forgetTableSeat, getSeat, initSeatContext } from './seat';
const { MatchState } = await import('./match.svelte');

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

const KEY = 'gorge.table.seat.g1';

describe('private table seat restore and forget', () => {
  beforeEach(() => vi.stubGlobal('localStorage', fakeStorage()));
  afterEach(() => {
    initSeatContext('');
    vi.unstubAllGlobals();
  });

  it('restores the remembered seat on a match replay path, never on a bare live table path', () => {
    initSeatContext('?seat=0&token=g1-token', '/t/g1');
    expect(localStorage.getItem(KEY)).not.toBeNull();

    initSeatContext('', '/t/g1');
    expect(getSeat()).toBeNull(); // a live table keeps its pre-store spectator default

    initSeatContext('', '/t/g1/m/1');
    expect(getSeat()).toEqual({ seat: 0, token: 'g1-token' }); // the replay link still restores it
  });

  it('forgetTableSeat removes the remembered seat and clears the live context', () => {
    initSeatContext('?seat=0&token=g1-token', '/t/g1');
    expect(getSeat()).not.toBeNull();

    forgetTableSeat('g1');
    expect(localStorage.getItem(KEY)).toBeNull();
    expect(getSeat()).toBeNull();

    initSeatContext('', '/t/g1/m/1');
    expect(getSeat()).toBeNull(); // and it stays forgotten
  });

  for (const status of [401, 403] as const) {
    it(`a ${status} from a seat-scoped fetch forgets the remembered seat before the redirect`, async () => {
      initSeatContext('?seat=0&token=g1-token', '/t/g1');
      const seat = getSeat();
      expect(seat).not.toBeNull();
      expect(localStorage.getItem(KEY)).not.toBeNull();

      const assign = vi.fn();
      vi.stubGlobal('location', { assign });
      fetchViewMock.mockReset();
      fetchEventsMock.mockReset();
      fetchViewMock.mockRejectedValue(new ApiErrorMock(status));

      const m = new MatchState('g1', seat!);
      m.apply({ v: 1, t: 'match_start', table: 'g1', match: 1, seq: 0, body: { seats: [], seed: 1, spectator: '', bot_policy: 'bot' } });
      await m.refreshLive();

      expect(assign).toHaveBeenCalledWith('/');
      expect(localStorage.getItem(KEY)).toBeNull();
      expect(getSeat()).toBeNull();
    });
  }
});
