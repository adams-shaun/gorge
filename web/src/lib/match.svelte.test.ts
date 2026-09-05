import { describe, expect, it, vi } from 'vitest';
import type { MatchStart, View } from '../protocol';

// match.svelte.ts calls fetchView (PL-16's post-decision/match_end refresh);
// stub api so this stays a hermetic test of apply()/dispatch() with
// hand-built frames, no network.
const { fetchViewMock } = vi.hoisted(() => ({ fetchViewMock: vi.fn() }));
vi.mock('./api', () => ({ fetchView: fetchViewMock }));

const { MatchState } = await import('./match.svelte');

const seats = [
  { name: 'Ari', deck: 'mono-red', colour: '#e5484d' },
  { name: 'Bo', deck: 'mono-green', colour: '#22c55e' },
];

const view = (turn = 1): View => ({
  viewer: 0, visibility: 'omniscient', turn, step: 'main', phase: 'main1', active: 0, priority: 0,
  over: false, draw: false, winner: null, players: [], stack: [], pending: [],
});

const matchStart = (): MatchStart => ({ seats, seed: 1, spectator: '' });

function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => { resolve = r; });
  return { promise, resolve };
}

describe('MatchState', () => {
  it('routes match_start, snapshot, and a contiguous event; a decision then triggers exactly one fetch', async () => {
    fetchViewMock.mockReset();
    fetchViewMock.mockResolvedValue(view(2));
    const m = new MatchState('t1');

    m.apply({ v: 1, t: 'match_start', table: 't1', match: 1, seq: 0, body: matchStart() });
    expect(m.match).toBe(1);
    expect(m.seats).toEqual(seats);

    m.apply({ v: 1, t: 'snapshot', table: 't1', match: 1, seq: 0, body: { view: view(1), turn_starts: [0], head: 10 } });
    expect(m.view).toEqual(view(1));
    expect(m.dvr.head).toBe(10);

    m.apply({ v: 1, t: 'event', table: 't1', match: 1, seq: 11, body: { event: { seq: 11, kind: 'tap', player: 0 }, line: 'x taps' } });
    expect(m.dvr.head).toBe(11);
    expect(m.dvr.gap).toBe(false);

    m.apply({ v: 1, t: 'decision', table: 't1', match: 1, seq: 12, body: { player: 0, kind: 'priority', prompt: 'You have priority.' } });
    expect(m.decision).toEqual({ player: 0, kind: 'priority', prompt: 'You have priority.' });
    await Promise.resolve();
    await Promise.resolve();
    expect(fetchViewMock).toHaveBeenCalledTimes(1);
    expect(fetchViewMock).toHaveBeenCalledWith('t1', 1, 11);
    expect(m.view).toEqual(view(2));
  });

  it('coalesces a burst of decisions into one in-flight fetch plus at most one follow-up', async () => {
    fetchViewMock.mockReset();
    const d = deferred<View>();
    fetchViewMock.mockReturnValueOnce(d.promise).mockResolvedValue(view(9));
    const m = new MatchState('t2');
    m.apply({ v: 1, t: 'match_start', table: 't2', match: 1, seq: 0, body: matchStart() });
    m.apply({ v: 1, t: 'snapshot', table: 't2', match: 1, seq: 0, body: { view: view(1), turn_starts: [0], head: 5 } });

    m.apply({ v: 1, t: 'decision', table: 't2', match: 1, seq: 6, body: { player: 0, kind: 'priority', prompt: 'p1' } });
    m.apply({ v: 1, t: 'decision', table: 't2', match: 1, seq: 6, body: { player: 0, kind: 'priority', prompt: 'p2' } });
    m.apply({ v: 1, t: 'decision', table: 't2', match: 1, seq: 6, body: { player: 0, kind: 'priority', prompt: 'p3' } });
    expect(fetchViewMock).toHaveBeenCalledTimes(1); // second and third are coalesced, not new requests

    d.resolve(view(5));
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
    expect(fetchViewMock).toHaveBeenCalledTimes(2); // exactly one coalesced follow-up, not three
    expect(m.view).toEqual(view(9));
  });

  it('ignores frames addressed to another table', () => {
    const m = new MatchState('t1');
    m.apply({ v: 1, t: 'match_start', table: 'other', match: 9, seq: 0, body: matchStart() });
    expect(m.match).toBeNull();
    expect(m.seats).toEqual([]);
  });

  it('an out-of-order event sets gap, and a fresh snapshot re-seeds and clears it', () => {
    const m = new MatchState('t1');
    m.apply({ v: 1, t: 'snapshot', table: 't1', match: 1, seq: 0, body: { view: view(1), turn_starts: [0], head: 5 } });
    m.apply({ v: 1, t: 'event', table: 't1', match: 1, seq: 9, body: { event: { seq: 9, kind: 'tap', player: 0 }, line: 'x' } });
    expect(m.dvr.gap).toBe(true);

    m.apply({ v: 1, t: 'snapshot', table: 't1', match: 1, seq: 0, body: { view: view(2), turn_starts: [0, 6], head: 9 } });
    expect(m.dvr.gap).toBe(false);
    expect(m.dvr.head).toBe(9);
    expect(m.dvr.events).toEqual([]);
    expect(m.view).toEqual(view(2));
  });

  it('match_end clears the decision and refreshes the live view; table_halted records the reason', async () => {
    fetchViewMock.mockReset();
    fetchViewMock.mockResolvedValue(view(3));
    const m = new MatchState('t1');
    m.apply({ v: 1, t: 'snapshot', table: 't1', match: 1, seq: 0, body: { view: view(1), turn_starts: [0], head: 4 } });
    m.apply({ v: 1, t: 'decision', table: 't1', match: 1, seq: 5, body: { player: 0, kind: 'priority', prompt: 'p' } });
    expect(m.decision).not.toBeNull();
    await Promise.resolve();
    await Promise.resolve();

    m.apply({ v: 1, t: 'match_end', table: 't1', match: 1, seq: 6, body: { result: 'win', winner: 0, head: 'h' } });
    expect(m.decision).toBeNull();
    await Promise.resolve();
    await Promise.resolve();
    expect(m.view).toEqual(view(3));

    m.apply({ v: 1, t: 'table_halted', table: 't1', seq: 7, body: { reason: 'panic' } });
    expect(m.halted).toBe('panic');
  });
});
