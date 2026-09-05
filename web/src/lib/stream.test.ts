import { describe, expect, it } from 'vitest';
import { openStream, parseFrame, type EventSourceLike } from './stream';

class FakeES implements EventSourceLike {
  static last: FakeES | null = null;
  listeners: Record<string, ((e: MessageEvent) => void)[]> = {};
  closed = false;
  constructor(public url: string) { FakeES.last = this; }
  addEventListener(type: string, fn: (e: MessageEvent) => void) { (this.listeners[type] ??= []).push(fn); }
  close() { this.closed = true; }
  emit(type: string, data: string, lastEventId = '') {
    for (const fn of this.listeners[type] ?? []) fn(new MessageEvent(type, { data, lastEventId }));
  }
}

const hello = (session: string) => JSON.stringify({ v: 1, t: 'hello', seq: 0, body: { session, tables: [] } });

describe('stream', () => {
  it('parses frames and rejects garbage', () => {
    expect(parseFrame(hello('s1'))?.t).toBe('hello');
    expect(parseFrame('{')).toBeNull();
    expect(parseFrame(JSON.stringify({ v: 2, t: 'hello', seq: 0, body: {} }))).toBeNull(); // wrong version
  });
  it('tracks the session across hellos and dispatches every frame type', () => {
    const s = openStream('/api/stream', FakeES as unknown as typeof EventSource);
    const seen: string[] = [];
    s.onFrame((f) => seen.push(f.t));
    const es = FakeES.last!;
    expect(es.url).toBe('/api/stream');
    es.emit('hello', hello('s1'));
    expect(s.session).toBe('s1');
    es.emit('widget', JSON.stringify({ v: 1, t: 'widget', seq: 5, table: 't1', match: 1, body: {} }));
    es.emit('hello', hello('s2'));
    expect(s.session).toBe('s2');
    expect(seen).toEqual(['hello', 'widget', 'hello']);
    s.close();
    expect(es.closed).toBe(true);
  });
});
