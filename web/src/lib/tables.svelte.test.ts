import { describe, expect, it, vi } from 'vitest';
import type { Frame, SeatInfo, TableInfo, Widget } from '../protocol';

// tables.svelte.ts wires its constructor to session.stream.onFrame, and the
// real session opens a browser EventSource — stub session so this stays a
// pure, hermetic test of apply()/load() with hand-built frames.
const { fakeSession } = vi.hoisted(() => {
  const fakeSession = { stream: { onFrame: (): (() => void) => () => {} } };
  return { fakeSession };
});
vi.mock('./session.svelte', () => ({ session: fakeSession }));

const fetchTablesMock = vi.fn();
vi.mock('./api', () => ({ fetchTables: fetchTablesMock }));

const { tables } = await import('./tables.svelte');

const info = (id: string, overrides: Partial<TableInfo> = {}): TableInfo =>
  ({ id, name: id, seats: 2, spectator: '', state: 'idle', match: 0, perpetual: false, ...overrides });

describe('tables', () => {
  it('loads in host order and updates via apply(frame)', async () => {
    fetchTablesMock.mockResolvedValue([info('t2'), info('t1')]);
    await tables.load();
    expect(tables.list.map((t) => t.info.id)).toEqual(['t2', 't1']); // host order, not object-key order

    const helloFrame: Frame = { v: 1, t: 'hello', seq: 0, body: { session: 's1', tables: [info('t1'), info('t2')] } };
    tables.apply(helloFrame);
    expect(tables.list.map((t) => t.info.id)).toEqual(['t1', 't2']);

    const widget: Widget = { turn: 3, step: 'main', phase: 'main1', active: 0, priority: 0, life: [20, 18], lost: [false, false], stack_depth: 1, last: 'x drew a card', state: 'live' };
    tables.apply({ v: 1, t: 'widget', seq: 1, table: 't1', match: 5, body: widget });
    expect(tables.list[0].widget).toEqual(widget);
    expect(tables.list[0].match).toBe(5);

    // a reconnect hello must not lose the widget/seats already known for t1
    tables.apply(helloFrame);
    expect(tables.list[0].widget).toEqual(widget);

    const seats: SeatInfo[] = [{ name: 'Ari', deck: 'mono-red', colour: '#e5484d' }, { name: 'Bo', deck: 'mono-green', colour: '#22c55e' }];
    tables.apply({ v: 1, t: 'match_start', seq: 2, table: 't1', match: 6, body: { seats, seed: 1, spectator: '' } });
    expect(tables.list[0].seats).toEqual(seats);
    expect(tables.list[0].info.state).toBe('live');
    expect(tables.list[0].match).toBe(6);

    tables.apply({ v: 1, t: 'table_halted', seq: 3, table: 't1', body: { reason: 'panic' } });
    expect(tables.list[0].info.state).toBe('halted');

    tables.apply({ v: 1, t: 'match_end', seq: 4, table: 't2', body: { result: 'win', winner: 0, head: 'h' } });
    expect(tables.list[1].info.state).toBe('idle'); // t2 is not perpetual

    // frames for unknown tables are ignored, not crashing
    tables.apply({ v: 1, t: 'widget', seq: 5, table: 'ghost', body: widget });
    expect(tables.list.find((t) => t.info.id === 'ghost')).toBeUndefined();
  });

  it('keeps a perpetual table in cooldown, not idle, on match_end', () => {
    tables.apply({ v: 1, t: 'hello', seq: 0, body: { session: 's1', tables: [info('p1', { perpetual: true })] } });
    tables.apply({ v: 1, t: 'match_end', seq: 1, table: 'p1', body: { result: 'win', winner: 0, head: 'h' } });
    expect(tables.list[0].info.state).toBe('cooldown');
  });
});
