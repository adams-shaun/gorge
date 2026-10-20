import { describe, expect, it, vi } from 'vitest';
import type { EventBody, MatchInfo, View } from '../protocol';

// The REST match routes fence a private play-vs-bot table behind the seat
// claim, so a finished-match load of that table must carry the token on both
// the match-list and event reads. api.ts is stubbed so this stays hermetic.
const { fetchViewMock, fetchEventsMock, fetchMatchesMock, ApiErrorMock } = vi.hoisted(() => {
  class ApiErrorMock extends Error {
    constructor(public status: number) { super(`HTTP ${status}`); }
  }
  return { fetchViewMock: vi.fn(), fetchEventsMock: vi.fn(), fetchMatchesMock: vi.fn(), ApiErrorMock };
});
vi.mock('./api', () => ({ ApiError: ApiErrorMock, fetchView: fetchViewMock, fetchEvents: fetchEventsMock, fetchMatches: fetchMatchesMock }));

const { MatchState } = await import('./match.svelte');

const ctx = { seat: 0, token: 'g1-token' };

const view = (): View => ({
  viewer: 0, visibility: 'omniscient', turn: 1, round: 1, step: 'main', phase: 'main1', active: 0, priority: 0,
  over: false, draw: false, winner: null, players: [], stack: [], pending: [],
});

const finished: MatchInfo = {
  table: 'g1', match: 1, seed: 1, seats: [], state: 'finished', result: 'win', winner: 0, events: 1, turns: 1, bot_policy: 'bot',
};

const ev = (seq: number): EventBody => ({ event: { seq, kind: 'tap', player: 0 }, line: `tap ${seq}` });

describe('private table match reads carry the seat claim', () => {
  it('loadFinished reads the match list and events with the seat claim', async () => {
    fetchMatchesMock.mockResolvedValue([finished]);
    fetchEventsMock.mockResolvedValue([ev(0)]);
    fetchViewMock.mockResolvedValue(view());

    const m = new MatchState('g1', ctx);
    await m.loadFinished(1);

    expect(fetchMatchesMock).toHaveBeenCalledWith('g1', ctx);
    expect(fetchEventsMock).toHaveBeenCalledWith('g1', 1, 0, ctx);
    expect(m.match).toBe(1);
  });
});
