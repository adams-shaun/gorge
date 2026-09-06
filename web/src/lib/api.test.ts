import { describe, expect, it, vi } from 'vitest';
import { eventsURL, matchesURL, pendingURL, seatQuery, tablesURL, viewURL, fetchView, fetchEvents, fetchPending, postIntent } from './api';
import type { Intent } from '../protocol';

const ctx = { seat: 2, token: 'tok-abc' };

// fetch is a global in node 20+; stub it so the api module's GET/POST
// paths are testable hermetically.
const fetchMock = vi.fn();
vi.stubGlobal('fetch', fetchMock);

describe('api urls', () => {
  it('builds the documented paths', () => {
    expect(tablesURL()).toBe('/api/tables');
    expect(matchesURL('t1')).toBe('/api/tables/t1/matches');
    expect(viewURL('t1', 3)).toBe('/api/tables/t1/matches/3/view');
    expect(viewURL('t1', 3, 0)).toBe('/api/tables/t1/matches/3/view?seq=0');
    expect(eventsURL('t 1', 3, 42)).toBe('/api/tables/t%201/matches/3/events?since=42');
    expect(pendingURL('t1', 3)).toBe('/api/tables/t1/matches/3/pending');
    expect(seatQuery(ctx)).toBe('?seat=2&token=tok-abc');
  });

  it('threads seat and token onto the seat-scoped GETs and leaves the spectator URLs byte-identical', () => {
    expect(viewURL('t1', 3, 0, undefined)).toBe('/api/tables/t1/matches/3/view?seq=0');
    expect(viewURL('t1', 3, 5, ctx)).toBe('/api/tables/t1/matches/3/view?seq=5&seat=2&token=tok-abc');
    expect(viewURL('t1', 3, undefined, ctx)).toBe('/api/tables/t1/matches/3/view?seat=2&token=tok-abc');
    expect(eventsURL('t1', 3, 7, undefined)).toBe('/api/tables/t1/matches/3/events?since=7');
    expect(eventsURL('t1', 3, 7, ctx)).toBe('/api/tables/t1/matches/3/events?since=7&seat=2&token=tok-abc');
  });

  it('fetchPending GETs the pending decision with the seat query, and postIntent POSTs the intent under Bearer with no ?seat=', async () => {
    fetchMock.mockReset();
    const pending = { seq: 4, player: 2, kind: 'priority', prompt: 'p', min: 1, max: 1, options: [] };
    fetchMock.mockResolvedValueOnce({ ok: true, json: async () => pending });
    await expect(fetchPending('t1', 3, ctx)).resolves.toEqual(pending);
    expect(fetchMock).toHaveBeenCalledWith('/api/tables/t1/matches/3/pending?seat=2&token=tok-abc', expect.objectContaining({ headers: expect.objectContaining({ Accept: 'application/json' }) }));

    fetchMock.mockResolvedValueOnce({ ok: true, status: 204, json: async () => ({}) });
    const intent: Intent = { seq: 4, player: 2, choices: [1] };
    await postIntent('t1', 3, intent, ctx);
    expect(fetchMock).toHaveBeenLastCalledWith('/api/tables/t1/matches/3/intent', expect.objectContaining({
      method: 'POST',
      headers: expect.objectContaining({ 'Content-Type': 'application/json', Authorization: 'Bearer tok-abc' }),
      body: JSON.stringify(intent),
    }));
  });

  it('postIntent surfaces a server rejection as ApiError, and fetchView/fetchEvents route seat-scoped GETs', async () => {
    fetchMock.mockReset();
    const d = { seq: 4, player: 2, kind: 'priority', prompt: 'p', min: 1, max: 1, options: [] };
    fetchMock.mockResolvedValueOnce({ ok: true, json: async () => d });
    await expect(fetchView('t1', 3, 9, ctx)).resolves.toEqual(d);
    expect(fetchMock).toHaveBeenCalledWith('/api/tables/t1/matches/3/view?seq=9&seat=2&token=tok-abc', expect.anything());

    fetchMock.mockReset();
    fetchMock.mockResolvedValueOnce({ ok: true, json: async () => [] });
    await expect(fetchEvents('t1', 3, 0, ctx)).resolves.toEqual([]);
    expect(fetchMock).toHaveBeenCalledWith('/api/tables/t1/matches/3/events?since=0&seat=2&token=tok-abc', expect.anything());

    fetchMock.mockReset();
    fetchMock.mockResolvedValueOnce({ ok: false, status: 409, json: async () => ({ code: 'conflict', message: 'intent seq 4, pending decision seq 6' }) });
    await expect(postIntent('t1', 3, { seq: 4, player: 2, choices: [1] }, ctx)).rejects.toMatchObject({ status: 409, code: 'conflict', message: 'intent seq 4, pending decision seq 6' });
  });

  it('fetchPending 409 (nothing pending) rejects rather than returning a stale empty', async () => {
    fetchMock.mockReset();
    fetchMock.mockResolvedValueOnce({ ok: false, status: 409, json: async () => ({ code: 'conflict', message: 'no decision pending for this seat' }) });
    await expect(fetchPending('t1', 3, ctx)).rejects.toMatchObject({ status: 409, code: 'conflict' });
  });
});
