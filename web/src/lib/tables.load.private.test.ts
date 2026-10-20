import { describe, expect, it, vi } from 'vitest';
import type { TableInfo, Widget } from '../protocol';

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

describe('tables.load private tables', () => {
  it('keeps a known private table and takes the lobby response for public tables', async () => {
    const priv = info('priv', { name: 'private game', state: 'live', match: 4, seat_names: ['Ari', 'Bo'] });
    const pub = info('pub', { name: 'old lobby name', state: 'idle' });
    tables.apply({ v: 1, t: 'hello', seq: 0, body: { session: 's1', tables: [pub, priv] } });
    expect(tables.list.map((t) => t.info.id)).toEqual(['pub', 'priv']);

    const widget: Widget = { turn: 3, step: 'main', phase: 'main1', active: 0, priority: 0, life: [20, 18], lost: [false, false], stack_depth: 1, last: 'x drew a card', state: 'live' };
    tables.apply({ v: 1, t: 'widget', seq: 1, table: 'priv', match: 4, body: widget });

    // The lobby endpoint omits the private on-demand table entirely.
    fetchTablesMock.mockResolvedValue([info('pub', { name: 'new lobby name', state: 'live', match: 9 })]);
    await tables.load();

    expect(tables.list.map((t) => t.info.id)).toEqual(['pub', 'priv']);
    expect(tables.list[0].info.name).toBe('new lobby name');
    expect(tables.list[0].info.state).toBe('live');
    expect(tables.list[0].match).toBe(9);

    const kept = tables.list.find((t) => t.info.id === 'priv');
    expect(kept).toBeDefined();
    expect(kept?.info.name).toBe('private game');
    expect(kept?.info.state).toBe('live');
    expect(kept?.match).toBe(4);
    expect(kept?.seats.map((s) => s.name)).toEqual(['Ari', 'Bo']);
    expect(kept?.widget).toEqual(widget);
  });
});
