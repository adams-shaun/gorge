import { describe, expect, it, vi } from 'vitest';
import type { Frame, TableInfo } from '../protocol';
import type { TableState } from './tables.svelte';

// tables.svelte.ts wires its constructor to session.stream.onFrame, and the
// real session opens a browser EventSource — stub session so this stays a
// pure, hermetic test of apply() with hand-built frames.
const { fakeSession } = vi.hoisted(() => {
  const fakeSession = { stream: { onFrame: (): (() => void) => () => {} } };
  return { fakeSession };
});
vi.mock('./session.svelte', () => ({ session: fakeSession }));

const fetchTablesMock = vi.fn();
vi.mock('./api', () => ({ fetchTables: fetchTablesMock }));

const { tables } = await import('./tables.svelte');

const info = (id: string, overrides: Partial<TableInfo> = {}): TableInfo =>
  ({ id, name: id, seats: 2, spectator: '', state: 'idle', match: 0, perpetual: false, format: 'constructed', bot_policy: 'bot', mulligans: 6, auto_mana: false, ...overrides });

function find(id: string): TableState {
  const t = tables.list.find((x) => x.info.id === id);
  if (!t) throw new Error(`no table ${id} in the store`);
  return t;
}

describe('tables: series table state', () => {
  it('a table_state frame sets the state, mapping between_games to cooldown', () => {
    tables.apply({ v: 1, t: 'hello', seq: 0, body: { session: 's1', tables: [info('s1', { series: true })] } });
    expect(find('s1').info.state).toBe('idle'); // precondition: a fresh table is idle

    tables.apply({ v: 1, t: 'table_state', seq: 1, table: 's1', body: { state: 'between_games' } });
    expect(find('s1').info.state).toBe('cooldown');

    tables.apply({ v: 1, t: 'table_state', seq: 2, table: 's1', body: { state: 'live' } });
    expect(find('s1').info.state).toBe('live');

    tables.apply({ v: 1, t: 'table_state', seq: 3, table: 's1', body: { state: 'idle' } });
    expect(find('s1').info.state).toBe('idle');
  });

  it("a series table's match_end is between games, not idle; a plain table's is idle", () => {
    tables.apply({ v: 1, t: 'hello', seq: 0, body: { session: 's1', tables: [info('ser', { series: true }), info('plain')] } });
    expect(find('ser').info.series).toBe(true); // precondition: the store kept the series flag
    expect(find('plain').info.series).toBeFalsy();

    tables.apply({ v: 1, t: 'match_end', seq: 1, table: 'ser', body: { result: 'win', winner: 0, head: 'h' } });
    expect(find('ser').info.state).toBe('cooldown');

    tables.apply({ v: 1, t: 'match_end', seq: 2, table: 'plain', body: { result: 'win', winner: 0, head: 'h' } });
    expect(find('plain').info.state).toBe('idle');
  });

  it('a series follows the server across games: finished board, between games, live again, then idle', () => {
    // The frame order host/registry.go emits for a Bo3 table: table_state live
    // ahead of each match_start; after a match_end the loop head sets
    // between_games while the series hook plans the next game, and idle once
    // the hook ends the series.
    tables.apply({ v: 1, t: 'hello', seq: 0, body: { session: 's1', tables: [info('bo3', { series: true, state: 'live', match: 1 })] } });
    const states: string[] = [];
    const step = (f: Pick<Frame, 't' | 'match' | 'body'>) => {
      tables.apply({ v: 1, seq: states.length + 1, table: 'bo3', ...f });
      states.push(find('bo3').info.state);
    };
    step({ t: 'match_end', match: 1, body: { result: 'win', winner: 1, head: 'h1' } });
    step({ t: 'table_state', match: 1, body: { state: 'between_games' } });
    step({ t: 'table_state', match: 2, body: { state: 'live' } });
    step({ t: 'match_start', match: 2, body: { seats: [] } });
    step({ t: 'match_end', match: 2, body: { result: 'win', winner: 1, head: 'h2' } });
    step({ t: 'table_state', match: 2, body: { state: 'between_games' } });
    step({ t: 'table_state', match: 2, body: { state: 'idle' } });
    expect(states).toEqual(['cooldown', 'cooldown', 'live', 'live', 'cooldown', 'cooldown', 'idle']);
    expect(find('bo3').match).toBe(2); // the store followed the second game
  });

  it('a series table that is between games on (re)connect is not idle', () => {
    // TableInfo.State reports the server's internal cooldown between games.
    tables.apply({ v: 1, t: 'hello', seq: 0, body: { session: 's1', tables: [info('bo3', { series: true, state: 'cooldown', match: 1 })] } });
    expect(find('bo3').info.state).toBe('cooldown');
  });

  it('a table_state frame for an unknown table is ignored', () => {
    tables.apply({ v: 1, t: 'hello', seq: 0, body: { session: 's1', tables: [info('known')] } });
    tables.apply({ v: 1, t: 'table_state', seq: 1, table: 'ghost', body: { state: 'live' } });
    expect(find('known').info.state).toBe('idle');
  });

  it('a table_state frame with an unrecognised or missing state is ignored (fail closed)', () => {
    tables.apply({ v: 1, t: 'hello', seq: 0, body: { session: 's1', tables: [info('known')] } });
    expect(find('known').info.state).toBe('idle'); // precondition: the table starts idle
    // The server emits only between_games|live|idle; anything else — a future
    // value this client does not know, or a missing state — must not become
    // the store's state and feed the idle/matches-page derivation.
    tables.apply({ v: 1, t: 'table_state', seq: 1, table: 'known', body: { state: 'rebooting' } });
    expect(find('known').info.state).toBe('idle');
    tables.apply({ v: 1, t: 'table_state', seq: 2, table: 'known', body: {} });
    expect(find('known').info.state).toBe('idle');
    // ...while a known value still lands.
    tables.apply({ v: 1, t: 'table_state', seq: 3, table: 'known', body: { state: 'between_games' } });
    expect(find('known').info.state).toBe('cooldown');
  });
});
