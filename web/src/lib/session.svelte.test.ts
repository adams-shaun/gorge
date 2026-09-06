import { describe, expect, it, vi } from 'vitest';
import type { Frame, Hello } from '../protocol';

// session.svelte.ts opens a real browser EventSource by default; stub the
// stream and api modules so this stays hermetic (no network, no EventSource).
// The openStream mock records the URL it is given, which is what the
// base-path wiring tests below assert on (streaming rides the same base as
// every other request path).
const { fakeStream, subscribeMock, unsubscribeMock, emit, streamUrls } = vi.hoisted(() => {
  const handlers = new Set<(f: Frame) => void>();
  let sessionId: string | null = null;
  const subscribeMock = vi.fn().mockResolvedValue(undefined);
  const unsubscribeMock = vi.fn().mockResolvedValue(undefined);
  const streamUrls: string[] = [];
  const fakeStream = {
    get session() { return sessionId; },
    onFrame(h: (f: Frame) => void) { handlers.add(h); return () => handlers.delete(h); },
    close: vi.fn(),
  };
  const emit = (f: Frame) => {
    if (f.t === 'hello') sessionId = (f.body as Hello).session;
    for (const h of handlers) h(f);
  };
  return { fakeStream, subscribeMock, unsubscribeMock, emit, streamUrls };
});
vi.mock('./stream', () => ({ openStream: (url: string) => { streamUrls.push(url); return fakeStream; } }));
vi.mock('./api', () => ({ subscribe: subscribeMock, unsubscribe: unsubscribeMock }));

const { session } = await import('./session.svelte');

const hello = (s: string): Frame => ({ v: 1, t: 'hello', seq: 0, body: { session: s, tables: [] } });
const flush = () => new Promise((r) => setTimeout(r, 0));

describe('session', () => {
  it('subscribes on hello and re-subscribes overview + focused tables across a reconnect', async () => {
    expect(session.id).toBeNull();
    // With an empty base (the default) the session opens the stream at the
    // root, byte-identical to the pre-base client.
    expect(streamUrls[0]).toBe('/api/stream');

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

  it('opens the stream under the base path when the served page sets one', async () => {
    // The session is a module singleton constructed at import time, so a
    // base set after the first import cannot reach the instance the other
    // tests hold; re-import in a fresh registry with the base already set
    // proves the constructor really does prefix the stream URL (./basepath
    // is real in these tests — only ./stream and ./api are mocked).
    vi.resetModules();
    const bp = await import('./basepath');
    bp.setBasePathForTests('/gorge');
    const mod = await import('./session.svelte');
    expect(mod.session).toBeDefined();
    expect(streamUrls.at(-1)).toBe('/gorge/api/stream');
    bp.setBasePathForTests('');
  });
});
