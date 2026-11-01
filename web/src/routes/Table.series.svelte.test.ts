import { describe, expect, it, vi } from 'vitest';
import { render } from 'svelte/server';
import type { PlayerView, TableInfo, View } from '../protocol';
import { initSeatContext } from '../lib/seat';

// A series (best-of-three) table between two games keeps the finished
// board on the table page because the TABLE STORE knows the table is between
// games (TableInfo.series at match_end, then the server's table_state frame),
// not because any event overlay reports the series in play. The join link
// here carries no &event=, so TableEventOverlay has no event client and
// cannot influence the page at all. The store is the real one, fed
// hand-built frames; MatchState and session are mocked as in
// Table.svelte.test.ts so the SSR render is deterministic.
const { fake } = vi.hoisted(() => {
  const shared = { view: null as View | null };
  class FakeMatch {
    match: number | null = 1;
    view: View | null = shared.view;
    seats = [];
    dvr = { match: 't1/1', head: 0, cursor: 0, live: true, events: [], turnStarts: [0], gap: false };
    decision = null;
    halted: string | null = null;
    loadError: string | null = null;
    dispatch() {}
    apply() {}
    loadFinished() {}
  }
  return { fake: { shared, FakeMatch } };
});
vi.mock('../lib/match.svelte', () => ({ MatchState: fake.FakeMatch }));
vi.mock('../lib/session.svelte', () => ({
  session: { stream: { onFrame: () => () => {} }, focus: async () => {}, unfocus: async () => {}, watchEvent: async () => {}, unwatchEvent: () => {} },
}));
vi.mock('../lib/images', () => ({ images: { url: () => new Promise<string | null>(() => {}), offline: () => false } }));

import Table from './Table.svelte';
import { tables } from '../lib/tables.svelte';

const player = (seat: number): PlayerView => ({
  seat, name: `P${seat}`, life: 20, lost: false, spells_cast_this_turn: 0, life_gained_this_turn: 0, life_lost_this_turn: 0, tokens_created_this_turn: 0, library_size: 30, hand_size: 7, graveyard_size: 0,
  completed_dungeons: 0,
  hand: [], battlefield: [], graveyard: [], exile: [], pool: {}, command: [], commanders: [], commander_casts: [],
});
const over: View = {
  viewer: 0, visibility: 'seat', turn: 6, round: 6, step: 'end', phase: 'end', active: 1, priority: 1,
  over: true, draw: false, winner: 1, players: [player(0), player(1)], stack: [], pending: [],
};
const info = (id: string, overrides: Partial<TableInfo> = {}): TableInfo =>
  ({ id, name: id, seats: 2, spectator: '', state: 'live', match: 1, perpetual: false, format: 'limited', bot_policy: 'bot', mulligans: 6, auto_mana: false, ...overrides });

function page(table: string): string {
  initSeatContext('?seat=0&token=GAMEtok');
  fake.shared.view = over;
  return render(Table, { props: { table } }).html;
}

const BOARD = /<section class="board[ "]/;
function onBoard(html: string): boolean {
  return BOARD.test(html) && !html.includes('matches-page');
}

describe('Table.svelte — a series table between games', () => {
  it('keeps the finished board after match_end and between_games, and drops to the match list only when the series is over', () => {
    tables.apply({ v: 1, t: 'hello', seq: 0, body: { session: 's1', tables: [info('bo3', { series: true })] } });
    tables.apply({ v: 1, t: 'match_end', seq: 1, table: 'bo3', match: 1, body: { result: 'win', winner: 1, head: 'h1' } });
    expect(onBoard(page('bo3'))).toBe(true);

    tables.apply({ v: 1, t: 'table_state', seq: 2, table: 'bo3', match: 1, body: { state: 'between_games' } });
    expect(onBoard(page('bo3'))).toBe(true);

    tables.apply({ v: 1, t: 'table_state', seq: 3, table: 'bo3', match: 2, body: { state: 'live' } });
    expect(onBoard(page('bo3'))).toBe(true);

    tables.apply({ v: 1, t: 'table_state', seq: 4, table: 'bo3', match: 2, body: { state: 'idle' } });
    const html = page('bo3');
    expect(html).toContain('matches-page');
    expect(html).not.toMatch(BOARD);
  });

  it('an ordinary single-match table still drops to the match list at match_end', () => {
    tables.apply({ v: 1, t: 'hello', seq: 0, body: { session: 's1', tables: [info('plain')] } });
    tables.apply({ v: 1, t: 'match_end', seq: 1, table: 'plain', match: 1, body: { result: 'win', winner: 1, head: 'h1' } });
    expect(page('plain')).toContain('matches-page');
  });
});
