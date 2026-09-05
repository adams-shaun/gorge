import { describe, expect, it, vi } from 'vitest';
import type { EventBody, MatchInfo, MatchStart, View } from '../protocol';

// match.svelte.ts calls fetchView (PL-16's post-decision/match_end refresh,
// and Task 21's cursor-driven fetch), fetchEvents (backfill, finished-match
// transcript) and fetchMatches (finished-match load); stub api so this stays
// a hermetic test of apply()/dispatch()/showCursor()/loadFinished() with
// hand-built frames, no network.
const { fetchViewMock, fetchEventsMock, fetchMatchesMock } = vi.hoisted(() => ({
  fetchViewMock: vi.fn(),
  fetchEventsMock: vi.fn(),
  fetchMatchesMock: vi.fn(),
}));
vi.mock('./api', () => ({ fetchView: fetchViewMock, fetchEvents: fetchEventsMock, fetchMatches: fetchMatchesMock }));

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

const ev = (seq: number, kind = 'tap'): EventBody => ({ event: { seq, kind, player: 0 }, line: `${kind} ${seq}` });

function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

// settle polls until predicate() is true instead of guessing how many
// microtask hops a promise chain needs (a ViewCache round trip alone is
// fetchViewMock's promise -> .then -> .finally -> showCursor's own await ->
// its .catch wrapper, and a backfill compounds that with a recursive
// dispatch/showCursor) — bounded so a real regression still fails fast
// rather than hanging.
async function settle(predicate: () => boolean, maxTicks = 200): Promise<void> {
  for (let i = 0; i < maxTicks; i++) {
    if (predicate()) return;
    await Promise.resolve();
  }
  throw new Error(`settle: condition still false after ${maxTicks} microtask ticks`);
}

// drain lets whatever is already queued run, for asserting that nothing
// further happens (no specific end-state to poll for).
async function drain(ticks = 20): Promise<void> {
  for (let i = 0; i < ticks; i++) await Promise.resolve();
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

describe('MatchState — DVR cursor fetching (Task 21)', () => {
  it('pause then step back fetches exactly one view for the new cursor', async () => {
    fetchViewMock.mockReset();
    fetchEventsMock.mockReset();
    fetchViewMock.mockImplementation((_t: string, _k: number, seq: number) => Promise.resolve(view(seq)));
    const m = new MatchState('t1');
    m.apply({ v: 1, t: 'snapshot', table: 't1', match: 1, seq: 0, body: { view: view(1), turn_starts: [0], head: 5 } });
    for (const seq of [6, 7, 8]) m.apply({ v: 1, t: 'event', table: 't1', match: 1, seq, body: ev(seq) });
    expect(m.dvr.head).toBe(8);

    m.dispatch({ type: 'pause' }); // cursor stays at head (8): one fetch for it
    await settle(() => m.view?.turn === 8);
    expect(fetchViewMock).toHaveBeenCalledTimes(1);
    expect(fetchViewMock).toHaveBeenLastCalledWith('t1', 1, 8);

    fetchViewMock.mockClear();
    m.dispatch({ type: 'step', by: -1 }); // cursor -> 7, cached events already cover it: no backfill
    await settle(() => m.view?.turn === 7);
    expect(fetchEventsMock).not.toHaveBeenCalled();
    expect(fetchViewMock).toHaveBeenCalledTimes(1);
    expect(fetchViewMock).toHaveBeenCalledWith('t1', 1, 7);
  });

  it('rapid scrubs are latest-wins: a stale response never overwrites a fresher one', async () => {
    fetchViewMock.mockReset();
    fetchEventsMock.mockReset();
    const pending = new Map<number, ReturnType<typeof deferred<View>>>();
    fetchViewMock.mockImplementation((_t: string, _k: number, seq: number) => {
      const d = deferred<View>();
      pending.set(seq, d);
      return d.promise;
    });
    const m = new MatchState('t1');
    m.apply({ v: 1, t: 'snapshot', table: 't1', match: 1, seq: 0, body: { view: view(1), turn_starts: [0], head: 5 } });
    for (const seq of [6, 7, 8, 9, 10]) m.apply({ v: 1, t: 'event', table: 't1', match: 1, seq, body: ev(seq) });
    m.dispatch({ type: 'pause' });
    await settle(() => pending.has(10));
    pending.get(10)?.resolve(view(10)); // the pause fetch (cursor is still at head=10): let it settle out of the way
    await settle(() => m.view?.turn === 10);

    m.dispatch({ type: 'scrub', seq: 7 });
    m.dispatch({ type: 'scrub', seq: 8 });
    m.dispatch({ type: 'scrub', seq: 9 }); // the cursor settles here
    expect(m.dvr.cursor).toBe(9);
    await settle(() => pending.has(7) && pending.has(8) && pending.has(9));

    // resolve out of order: the oldest request last, the newest first
    pending.get(9)?.resolve(view(9));
    await settle(() => m.view?.turn === 9);
    pending.get(7)?.resolve(view(7));
    pending.get(8)?.resolve(view(8));
    await drain();

    expect(m.view).toEqual(view(9)); // never clobbered by the later-resolving, earlier-issued fetches
  });

  it('scrubbing before the cached events window triggers exactly one bounded backfill', async () => {
    fetchViewMock.mockReset();
    fetchEventsMock.mockReset();
    fetchViewMock.mockImplementation((_t: string, _k: number, seq: number) => Promise.resolve(view(seq)));
    fetchEventsMock.mockResolvedValue([ev(0, 'turn'), ev(10, 'tap'), ev(40, 'turn')]);
    const m = new MatchState('t1');
    m.apply({ v: 1, t: 'snapshot', table: 't1', match: 1, seq: 0, body: { view: view(1), turn_starts: [0, 40], head: 50 } });
    m.apply({ v: 1, t: 'event', table: 't1', match: 1, seq: 51, body: ev(51) });
    m.dispatch({ type: 'pause' }); // cursor 51, events window starts at 51: no backfill yet
    await settle(() => m.view?.turn === 51);
    expect(fetchEventsMock).not.toHaveBeenCalled();

    m.dispatch({ type: 'scrub', seq: 10 }); // well before the known window
    await settle(() => m.view?.turn === 10);

    expect(fetchEventsMock).toHaveBeenCalledTimes(1);
    expect(fetchEventsMock).toHaveBeenCalledWith('t1', 1, 0);
    expect(m.dvr.events.map((e) => e.event.seq)).toEqual([0, 10, 40, 51]);
    expect(m.view).toEqual(view(10));
  });

  it('loadFinished renders a finished match paused at its last seq, with no stream', async () => {
    fetchViewMock.mockReset();
    fetchEventsMock.mockReset();
    fetchMatchesMock.mockReset();
    const info: MatchInfo = { table: 't1', match: 3, seed: 1, seats, state: 'finished', result: 'win', winner: 1, events: 6, turns: 2 };
    fetchMatchesMock.mockResolvedValue([info]);
    const all = [ev(0, 'turn'), ev(1), ev(2), ev(3, 'turn'), ev(4), ev(5)];
    fetchEventsMock.mockResolvedValue(all);
    fetchViewMock.mockImplementation((_t: string, _k: number, seq: number) => Promise.resolve(view(seq)));

    const m = new MatchState('t1');
    await m.loadFinished(3);
    await settle(() => m.view !== null);

    expect(fetchMatchesMock).toHaveBeenCalledWith('t1');
    expect(fetchEventsMock).toHaveBeenCalledWith('t1', 3, 0);
    expect(m.match).toBe(3);
    expect(m.seats).toEqual(seats);
    expect(m.dvr.head).toBe(5);
    expect(m.dvr.cursor).toBe(5);
    expect(m.dvr.live).toBe(false);
    expect(m.dvr.turnStarts).toEqual([0, 3]);
    expect(m.dvr.events.map((e) => e.event.seq)).toEqual([0, 1, 2, 3, 4, 5]);
    expect(m.view).toEqual(view(5));
    expect(m.loadError).toBeNull();
  });

  it('loadFinished sets loadError (and never rejects) when the table has no such match', async () => {
    fetchMatchesMock.mockReset();
    fetchMatchesMock.mockResolvedValue([]);
    const m = new MatchState('t9');
    // A fire-and-forget `void m.loadFinished(k)` (as Table.svelte does for a
    // stale /t/{t}/m/{k} link) must never produce an unhandled rejection:
    // the promise resolves either way, and the failure is reported through
    // loadError for the caller to render instead.
    await expect(m.loadFinished(1)).resolves.toBeUndefined();
    expect(m.loadError).toBe('no match 1');
    expect(m.match).toBeNull();
  });

  it('loadFinished clears a previous loadError on a subsequent success', async () => {
    fetchMatchesMock.mockReset();
    fetchEventsMock.mockReset();
    fetchViewMock.mockReset();
    fetchMatchesMock.mockResolvedValueOnce([]).mockResolvedValue([
      { table: 't9', match: 2, seed: 1, seats, state: 'finished', winner: 0, events: 2, turns: 1 } satisfies MatchInfo,
    ]);
    fetchEventsMock.mockResolvedValue([ev(0, 'turn'), ev(1)]);
    fetchViewMock.mockImplementation((_t: string, _k: number, seq: number) => Promise.resolve(view(seq)));
    const m = new MatchState('t9');

    await m.loadFinished(2); // no such match yet
    expect(m.loadError).toBe('no match 2');

    await m.loadFinished(2); // now it exists
    await settle(() => m.view !== null);
    expect(m.loadError).toBeNull();
    expect(m.match).toBe(2);
  });
});

describe('MatchState — stale paused-cursor fetches never clobber a fresher view (Task 21 fix round)', () => {
  it('returning to live discards a slower in-flight paused-cursor fetch', async () => {
    fetchViewMock.mockReset();
    fetchEventsMock.mockReset();
    const slow = deferred<View>();
    fetchViewMock
      .mockImplementationOnce(() => slow.promise) // the paused cursor's fetch, at seq 10
      .mockImplementation((_t: string, _k: number, seq: number) => Promise.resolve(view(seq))); // refreshLive's fetch onward
    const m = new MatchState('t1');
    m.apply({ v: 1, t: 'snapshot', table: 't1', match: 1, seq: 0, body: { view: view(1), turn_starts: [0], head: 5 } });
    for (const seq of [6, 7, 8, 9, 10]) m.apply({ v: 1, t: 'event', table: 't1', match: 1, seq, body: ev(seq) });

    m.dispatch({ type: 'pause' }); // fires the slow fetch for seq 10; it will not resolve yet
    await drain();
    expect(fetchViewMock).toHaveBeenCalledTimes(1);

    m.dispatch({ type: 'live' }); // returns to live; refreshLive fetches head (10) via a fresh, fast call
    await settle(() => m.view?.turn === 10);
    expect(fetchViewMock).toHaveBeenCalledTimes(2);

    slow.resolve(view(999)); // the stale paused-cursor fetch finally resolves
    await drain();
    expect(m.view).toEqual(view(10)); // unchanged: the live head view, not the stale paused one
  });

  it('a match transition discards a slower in-flight paused-cursor fetch from the old match', async () => {
    fetchViewMock.mockReset();
    fetchEventsMock.mockReset();
    const slow = deferred<View>();
    fetchViewMock.mockImplementationOnce(() => slow.promise); // match 1's paused cursor fetch
    const m = new MatchState('t1');
    m.apply({ v: 1, t: 'match_start', table: 't1', match: 1, seq: 0, body: matchStart() });
    m.apply({ v: 1, t: 'snapshot', table: 't1', match: 1, seq: 0, body: { view: view(1), turn_starts: [0], head: 5 } });
    for (const seq of [6, 7]) m.apply({ v: 1, t: 'event', table: 't1', match: 1, seq, body: ev(seq) });
    m.dispatch({ type: 'pause' }); // fires the slow fetch for seq 7; leaves it in flight
    await drain();
    expect(fetchViewMock).toHaveBeenCalledTimes(1);

    // match 1 ends, match 2 starts and snapshots in live — the new match's
    // view comes straight from the frame, no fetch involved.
    m.apply({ v: 1, t: 'match_start', table: 't1', match: 2, seq: 0, body: matchStart() });
    expect(m.view).toBeNull(); // match_start clears the previous match's board
    m.apply({ v: 1, t: 'snapshot', table: 't1', match: 2, seq: 0, body: { view: view(42), turn_starts: [0], head: 0 } });
    expect(m.view).toEqual(view(42));

    slow.resolve(view(999)); // match 1's stale fetch finally resolves
    await drain();
    expect(m.view).toEqual(view(42)); // still match 2's view, untouched by match 1's stale fetch
  });
});
