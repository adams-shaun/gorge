import { describe, expect, it, vi } from 'vitest';
import type { MatchStart, View } from '../protocol';

// The seated half of MatchState (M2e-4): when constructed with a seat
// context, views and events are fetched seat-scoped (ViewAtSeat/EventsSeat
// over the wire), the pushed spectator snapshot/event bodies are never
// rendered, and the transcript is backfilled with the redacted lines on
// decision boundaries. The spectator half stays byte-identical — the
// existing match.svelte.test.ts pins its exact fetch arguments.
const { fetchViewMock, fetchEventsMock, fetchMatchesMock } = vi.hoisted(() => ({
  fetchViewMock: vi.fn(),
  fetchEventsMock: vi.fn(),
  fetchMatchesMock: vi.fn(),
}));
vi.mock('./api', () => ({ fetchView: fetchViewMock, fetchEvents: fetchEventsMock, fetchMatches: fetchMatchesMock }));

const { MatchState } = await import('./match.svelte');

const seat = { seat: 0, token: 'tok' } as const;

const matchStart = (): MatchStart => ({ seats: [], seed: 1, spectator: '' });
const spectatorView = (turn = 1): View => ({
  viewer: 99, visibility: 'omniscient', turn, step: 'main', phase: 'main1', active: 0, priority: 0,
  over: false, draw: false, winner: null, players: [], stack: [], pending: [],
});
const seatView = (turn = 1): View => ({
  viewer: 0, visibility: 'seat', turn, step: 'main', phase: 'main1', active: 0, priority: 0,
  over: false, draw: false, winner: null, players: [], stack: [], pending: [],
});

async function settle(predicate: () => boolean, maxTicks = 200): Promise<void> {
  for (let i = 0; i < maxTicks; i++) {
    if (predicate()) return;
    await Promise.resolve();
  }
  throw new Error(`settle: condition still false after ${maxTicks} microtask ticks`);
}

describe('MatchState — seated (M2e-4)', () => {
  it('a snapshot renders the seat-scoped view at head, never the pushed spectator god view', async () => {
    fetchViewMock.mockReset();
    fetchEventsMock.mockReset();
    fetchViewMock.mockResolvedValue(seatView(2));
    fetchEventsMock.mockResolvedValue([]);
    const m = new MatchState('t1', seat);

    m.apply({ v: 1, t: 'match_start', table: 't1', match: 1, seq: 0, body: matchStart() });
    m.apply({ v: 1, t: 'snapshot', table: 't1', match: 1, seq: 0, body: { view: spectatorView(1), turn_starts: [0], head: 10 } });

    await settle(() => m.view !== null);
    expect(m.view).toEqual(seatView(2)); // the seat's own redacted projection
    expect(fetchViewMock).toHaveBeenCalledWith('t1', 1, 10, seat); // ?seat= threaded
    expect(fetchEventsMock).toHaveBeenCalledWith('t1', 1, 0, seat); // redacted transcript from the top
    expect(m.dvr.head).toBe(10);
  });

  it('event frames chit the public head but never store spectator-redacted lines; the next decision boundary backfills the seat lines', async () => {
    fetchViewMock.mockReset();
    fetchEventsMock.mockReset();
    fetchViewMock.mockResolvedValue(seatView(3));
    fetchEventsMock.mockResolvedValueOnce([]); // snapshot backfill since 0, head 10
    const m = new MatchState('t1', seat);
    m.apply({ v: 1, t: 'match_start', table: 't1', match: 1, seq: 0, body: matchStart() });
    m.apply({ v: 1, t: 'snapshot', table: 't1', match: 1, seq: 0, body: { view: spectatorView(1), turn_starts: [0], head: 10 } });
    await settle(() => m.view !== null);

    // the spectator event body names the hidden card ("Lightning Bolt"); it
    // must not land in the transcript, only its seq chits the head
    fetchEventsMock.mockResolvedValueOnce([
      { event: { seq: 11, kind: 'draw', player: 1 }, line: 'Seat 1 draws a card' }, // the redacted seat line
    ]);
    m.apply({ v: 1, t: 'event', table: 't1', match: 1, seq: 11, body: { event: { seq: 11, kind: 'draw', player: 1 }, line: 'Seat 1 draws Lightning Bolt' } });
    expect(m.dvr.head).toBe(11);
    expect(m.dvr.events).toEqual([]); // the spectator line is not rendered

    m.apply({ v: 1, t: 'decision', table: 't1', match: 1, seq: 11, body: { player: 0, kind: 'priority', prompt: 'You have priority.' } });
    await settle(() => fetchEventsMock.mock.calls.length >= 2);
    expect(fetchEventsMock).toHaveBeenLastCalledWith('t1', 1, 11, seat); // since the last backfilled seq, not 0
    await settle(() => m.dvr.events.length === 1);
    expect(m.dvr.events[0].line).toBe('Seat 1 draws a card'); // the redacted line
  });

  it('unseated construction renders the pushed snapshot and fetches nothing (R-E4-4)', async () => {
    fetchViewMock.mockReset();
    fetchEventsMock.mockReset();
    fetchViewMock.mockResolvedValue(spectatorView(1));
    fetchEventsMock.mockResolvedValue([]);
    const m = new MatchState('t1');
    m.apply({ v: 1, t: 'snapshot', table: 't1', match: 1, seq: 0, body: { view: spectatorView(1), turn_starts: [0], head: 0 } });
    await settle(() => m.view !== null);
    expect(m.view).toEqual(spectatorView(1)); // the pushed snapshot renders as it always did
    expect(fetchViewMock).not.toHaveBeenCalled(); // no seat-scoped view GET on the spectator path
    expect(fetchEventsMock).not.toHaveBeenCalled(); // no transcript backfill for a spectator
  });
});
