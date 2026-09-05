import { describe, expect, it, vi } from 'vitest';
import type { Frame, Hello } from '../protocol';

// session.svelte.ts opens a real browser EventSource by default; stub the
// stream and api modules so this stays hermetic (no network, no EventSource).
const { fakeStream, subscribeMock, unsubscribeMock, emit } = vi.hoisted(() => {
  const handlers = new Set<(f: Frame) => void>();
  let sessionId: string | null = null;
  const subscribeMock = vi.fn().mockResolvedValue(undefined);
  const unsubscribeMock = vi.fn().mockResolvedValue(undefined);
  const fakeStream = {
    get session() { return sessionId; },
    onFrame(h: (f: Frame) => void) { handlers.add(h); return () => handlers.delete(h); },
    close: vi.fn(),
  };
  const emit = (f: Frame) => {
    if (f.t === 'hello') sessionId = (f.body as Hello).session;
    for (const h of handlers) h(f);
  };
  return { fakeStream, subscribeMock, unsubscribeMock, emit };
});
vi.mock('./stream', () => ({ openStream: () => fakeStream }));
vi.mock('./api', () => ({ subscribe: subscribeMock, unsubscribe: unsubscribeMock }));

const { session } = await import('./session.svelte');

const hello = (s: string): Frame => ({ v: 1, t: 'hello', seq: 0, body: { session: s, tables: [] } });
const flush = () => new Promise((r) => setTimeout(r, 0));

describe('session', () => {
  it('subscribes on hello and re-subscribes overview + focused tables across a reconnect', async () => {
    expect(session.id).toBeNull();

    session.ensureOverview(); // no session yet: just remembers the intent
    expect(subscribeMock).not.toHaveBeenCalled();

    emit(hello('s1'));
    expect(session.id).toBe('s1');
    expect(subscribeMock).toHaveBeenCalledWith('s1', '*', 'overview');
    await flush(); // let the hello's own resubscribe() settle before isolating focus()'s idempotency below

    subscribeMock.mockClear();
    await session.focus('t1');
    expect(subscribeMock).toHaveBeenCalledWith('s1', 't1', 'focus');
    expect(subscribeMock).toHaveBeenCalledTimes(1);

    // idempotent: a re-firing effect (e.g. a route re-render) must not
    // re-issue the subscribe — the host pushes a fresh snapshot per subscribe
    await session.focus('t1');
    expect(subscribeMock).toHaveBeenCalledTimes(1);

    subscribeMock.mockClear();
    emit(hello('s2')); // server-side restart: fresh session id
    expect(session.id).toBe('s2');
    await flush();
    expect(subscribeMock).toHaveBeenCalledWith('s2', '*', 'overview');
    expect(subscribeMock).toHaveBeenCalledWith('s2', 't1', 'focus');

    await session.unfocus('t1');
    expect(unsubscribeMock).toHaveBeenCalledWith('s2', 't1');

    subscribeMock.mockClear();
    emit(hello('s3'));
    await flush();
    expect(subscribeMock).toHaveBeenCalledWith('s3', '*', 'overview');
    expect(subscribeMock).not.toHaveBeenCalledWith('s3', 't1', 'focus');
  });
});
