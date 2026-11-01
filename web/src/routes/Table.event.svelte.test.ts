import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'svelte/server';
import type { EventSeries, EventState, PlayerView, SeatInfo, View } from '../protocol';
import { initSeatContext } from '../lib/seat';
import { eventState } from '../test/eventfixtures';

// A series game's join link carries &event=<id>; the table page then
// watches that event with the event seat the event page REMEMBERED (never
// the URL's game token) and overlays the between-games panel while the
// viewer's series on this table is between games and the server waits on
// the viewer's deck. Same SSR harness as Table.svelte.test.ts: MatchState
// and session are mocked; EventClient is a fake whose state the test sets
// (its load() would be the network, and onMount never runs under SSR).
const { fake } = vi.hoisted(() => {
  const shared = {
    state: null as EventState | null,
    ctor: [] as [string, unknown][],
    view: null as View | null,
    seats: [] as SeatInfo[],
  };
  class FakeMatch {
    match: number | null = 1;
    view: View | null = null;
    seats: SeatInfo[] = [];
    dvr = { match: 't1/1', head: 0, cursor: 0, live: true, events: [], turnStarts: [0], gap: false };
    decision = null;
    halted: string | null = null;
    loadError: string | null = null;
    constructor() {
      this.view = shared.view;
      this.seats = shared.seats;
    }
    dispatch() {}
    apply() {}
    loadFinished() {}
  }
  class FakeEventClient {
    id: string;
    seat: { seat: number; token: string } | null;
    state: EventState | null;
    constructor(id: string, seat: { seat: number; token: string } | null) {
      shared.ctor.push([id, seat]);
      this.id = id;
      this.seat = seat;
      this.state = shared.state;
    }
    get you() {
      return this.state?.you ?? null;
    }
    get token() {
      return this.seat?.token ?? null;
    }
    async load() {}
    handleFrame() {
      return false;
    }
  }
  return { fake: { shared, FakeMatch, FakeEventClient } };
});
vi.mock('../lib/match.svelte', () => ({ MatchState: fake.FakeMatch }));
vi.mock('../lib/session.svelte', () => ({
  session: { stream: { onFrame: () => () => {} }, focus: async () => {}, unfocus: async () => {}, watchEvent: async () => {}, unwatchEvent: () => {} },
}));
vi.mock('../lib/draftevent.svelte', async (orig) => ({ ...(await orig<typeof import('../lib/draftevent.svelte')>()), EventClient: fake.FakeEventClient }));
vi.mock('../lib/images', () => ({ images: { url: () => new Promise<string | null>(() => {}), offline: () => false } }));

import Table from './Table.svelte';

const player = (seat: number): PlayerView => ({
  seat, name: `P${seat}`, life: 20, lost: false, spells_cast_this_turn: 0, life_gained_this_turn: 0, life_lost_this_turn: 0, tokens_created_this_turn: 0, library_size: 30, hand_size: 7, graveyard_size: 0,
  completed_dungeons: 0,
  hand: [], battlefield: [], graveyard: [], exile: [], pool: {}, command: [], commanders: [], commander_casts: [],
});
const over: View = {
  viewer: 0, visibility: 'seat', turn: 6, round: 6, step: 'end', phase: 'end', active: 1, priority: 1,
  over: true, draw: false, winner: 1, players: [player(0), player(1)], stack: [], pending: [],
};
const TABLE = 'e1-r1-s03-g1-a1';
const series = (o: Partial<EventSeries>): EventSeries => ({
  round: 1, a: 0, b: 3, best_of: 3, state: 'between', table: TABLE, game: 2, games: [3], chooser: 0, waiting: [0, 3], score: [0, 1], winner: null, ...o,
});

function memStorage(init: Record<string, string> = {}): Storage {
  const m = new Map(Object.entries(init));
  return {
    get length() { return m.size; },
    clear: () => m.clear(),
    getItem: (k) => m.get(k) ?? null,
    key: (i) => [...m.keys()][i] ?? null,
    removeItem: (k) => void m.delete(k),
    setItem: (k, v) => void m.set(k, String(v)),
  };
}

function page(row: EventSeries | null, opts: { stored?: boolean; event?: boolean } = {}) {
  const { stored = true, event = true } = opts;
  const search = `?seat=0&token=GAMEtokenNEVERanEVENTseat${event ? '&event=e1' : ''}`;
  initSeatContext(search);
  vi.stubGlobal('location', { search, origin: 'http://127.0.0.1', pathname: `/t/${TABLE}` });
  const storage = memStorage(stored ? { 'gorge.event.seat.e1': JSON.stringify({ seat: 0, token: 'EVENTtok0' }) } : {});
  vi.stubGlobal('localStorage', storage);
  fake.shared.view = over;
  fake.shared.seats = [{ name: 'Ada', deck: 'd', colour: '#e5484d' }, { name: 'Bot 4', deck: 'd', colour: '#22c55e' }];
  fake.shared.state = eventState({ phase: 'ROUND1', round: 1, best_of: 3, series: row ? [row] : [] });
  fake.shared.ctor = [];
  const { html } = render(Table, { props: { table: TABLE } });
  return { html, storage };
}

afterEach(() => vi.unstubAllGlobals());

describe('Table.svelte — the between-games overlay', () => {
  it('with ?event= and a remembered event seat, a between row waiting on the viewer overlays the panel', () => {
    const { html, storage } = page(series({}));
    expect(html).toContain('data-testid="table-between"');
    expect(html).toContain('data-testid="between-games"');
    expect(html).toContain('You will choose play or draw');
    // The event seat is the REMEMBERED one; the URL's game token is never
    // taken (or stored) as an event seat, and never rendered.
    expect(fake.shared.ctor).toEqual([['e1', { seat: 0, token: 'EVENTtok0' }]]);
    expect(storage.getItem('gorge.event.seat.e1')).toBe(JSON.stringify({ seat: 0, token: 'EVENTtok0' }));
    expect(html).not.toContain('GAMEtoken');
    expect(html).not.toContain('EVENTtok0');
  });

  it('hides once the next game is live', () => {
    const { html } = page(series({ state: 'live', game: 2, waiting: undefined }));
    expect(html).not.toContain('data-testid="table-between"');
    expect(html).not.toContain('data-testid="between-games"');
  });

  it('after the viewer\'s deck is in, a status line replaces the panel', () => {
    const { html } = page(series({ waiting: [3] }));
    expect(html).not.toContain('data-testid="between-games"');
    expect(html).toMatch(/data-testid="table-between-waiting"[^>]*>\s*Deck locked for game 2\. Waiting for Seat 4 · Bot 4 \(bot\)…/);
  });

  it('no remembered seat, or no event in the link: no event client and no overlay', () => {
    for (const o of [{ stored: false }, { event: false }]) {
      const { html } = page(series({}), o);
      expect(fake.shared.ctor).toEqual([]);
      expect(html).not.toContain('data-testid="table-between"');
    }
  });

  it('a series on another table (after a crash or restart moved it) is not this page\'s', () => {
    const { html } = page(series({ table: 'e1-r1-s03-g1-a2' }));
    expect(html).not.toContain('data-testid="table-between"');
  });
});
