import { describe, expect, it, vi } from 'vitest';
import { render } from 'svelte/server';
import type { EventBody, PlayerView, SeatInfo, View } from '../protocol';
import { initSeatContext } from '../lib/seat';

// Table.svelte's MatchState opens nothing at import, but session.svelte's
// module-level Session opens a browser EventSource and match.svelte wires to
// it — mock session, and MatchState with a fixture-carrying fake so the SSR
// render is deterministic (nothing mounts, so no onMount fetches run; the
// fake's view/seats/dvr are exactly what the template renders).
const { fakeMatch } = vi.hoisted(() => {
  const shared = {
    view: null as View | null,
    seats: [] as SeatInfo[],
    events: [] as EventBody[],
    ctorArgs: [] as [string, unknown][],
    lastSeat: undefined as unknown,
  };
  class FakeMatch {
    constructor(table: string, seat?: unknown) {
      shared.ctorArgs.push([table, seat]);
      shared.lastSeat = seat;
    }
    match: number | null = 1;
    view = shared.view;
    seats = shared.seats;
    dvr = { match: 't1/1', head: 0, cursor: 0, live: true, events: shared.events, turnStarts: [0], gap: false };
    decision = null;
    halted: string | null = null;
    loadError: string | null = null;
    dispatch() {}
    apply() {}
    loadFinished() {}
  }
  return { fakeMatch: { shared, MatchState: FakeMatch } };
});
vi.mock('../lib/match.svelte', () => ({ MatchState: fakeMatch.MatchState }));
vi.mock('../lib/session.svelte', () => ({
  session: { stream: { onFrame: () => () => {} }, focus: async () => {}, unfocus: async () => {} },
}));

// The component's default export is the Svelte component; vi.mock is
// hoisted above this static import, so the mocks are installed first.
import Table from './Table.svelte';
const player = (seat: number): PlayerView => ({
  seat, name: `P${seat}`, life: 20, lost: false, library_size: 30, hand_size: 7, graveyard_size: 0,
  hand: [], battlefield: [], graveyard: [], exile: [], pool: {},
});
const seats: SeatInfo[] = [{ name: 'Ari', deck: 'mono-red', colour: '#e5484d' }, { name: 'Bo', deck: 'mono-green', colour: '#22c55e' }];
const view = (overrides: Partial<View> = {}): View => ({
  viewer: 0, visibility: 'seat', turn: 3, step: 'main', phase: 'main1', active: 0, priority: 0,
  over: false, draw: false, winner: null, players: [player(0), player(1)], stack: [], pending: [], ...overrides,
});

describe('Table.svelte seat gating (R-E4-4 / R-E4-5)', () => {
  it('test 5 — no seat in the URL renders no panel, and the spectator page renders as it always did', () => {
    initSeatContext('');
    fakeMatch.shared.view = view();
    fakeMatch.shared.seats = seats;
    fakeMatch.shared.ctorArgs = [];

    const { html } = render(Table, { props: { table: 't1' } });

    expect(html).not.toContain('data-seat-panel'); // no seat -> no panel
    expect(html).toContain('data-cursor'); // the DVR bar still renders for the spectator
    // the seat identity never reached the MatchState: constructed with no
    // seat context, so no seat-scoped fetch can be built from it
    expect(fakeMatch.shared.ctorArgs).toEqual([['t1', undefined]]);
  });

  it('test 7 — seated, the panel renders, and the token never reaches the DOM or the transcript', () => {
    initSeatContext('?seat=0&token=TOPSECRETVALUEnEVERseen');
    fakeMatch.shared.view = view({
      decision: {
        seq: 9, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1,
        options: [
          { index: 0, kind: 'cast', label: 'Cast Lighting Bolt', obj: undefined, player: 0 },
          { index: 1, kind: 'pass', label: 'Pass priority', obj: undefined, player: 0 },
          { index: 2, kind: 'concede', label: 'Concede', obj: undefined, player: 0 },
        ],
      },
    });
    fakeMatch.shared.seats = seats;
    fakeMatch.shared.events = [{ event: { seq: 1, kind: 'draw', player: 0 }, line: 'Ari draws a card' }];

    const { html } = render(Table, { props: { table: 't1' } });

    expect(html).toContain('data-seat-panel'); // the seat surface is mounted
    expect(html).toContain('Ari draws a card'); // the transcript renders
    expect(html).not.toContain('TOPSECRETVALUEnEVERseen'); // R-E4-5: the token is never rendered
    expect(fakeMatch.shared.lastSeat).toEqual({ seat: 0, token: 'TOPSECRETVALUEnEVERseen' });

    // restore the no-seat baseline for any later test in this file
    initSeatContext('');
  });
});
