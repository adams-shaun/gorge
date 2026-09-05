import type { Frame, FrameType, Hello } from '../protocol';

/** The subset of EventSource the stream uses; tests pass a fake. */
export interface EventSourceLike {
  addEventListener(type: string, fn: (e: MessageEvent) => void): void;
  close(): void;
}
type ESCtor = new (url: string) => EventSourceLike;

export type FrameHandler = (f: Frame) => void;

export interface Stream {
  readonly session: string | null;
  onFrame(h: FrameHandler): () => void;
  close(): void;
}

const FRAME_TYPES: FrameType[] = ['hello', 'widget', 'match_start', 'snapshot', 'event', 'decision', 'match_end', 'table_halted', 'overflow', 'error'];

/** parseFrame decodes one SSE data line; anything malformed or from another protocol version is dropped. */
export function parseFrame(data: string): Frame | null {
  try {
    const f = JSON.parse(data) as Frame;
    if (f.v !== 1 || !FRAME_TYPES.includes(f.t as FrameType)) return null;
    return f;
  } catch {
    return null;
  }
}

/**
 * openStream owns one EventSource. The browser reconnects and resends
 * Last-Event-ID by itself; every hello (first connect, or a resume the
 * server could not serve from its ring) carries a new session id, which
 * handlers see as a hello frame and answer by re-subscribing.
 */
export function openStream(url: string, es: ESCtor = EventSource as unknown as ESCtor): Stream {
  const source = new es(url);
  const handlers = new Set<FrameHandler>();
  let session: string | null = null;
  for (const t of FRAME_TYPES) {
    source.addEventListener(t, (e: MessageEvent) => {
      const f = parseFrame(String(e.data));
      if (!f) return;
      if (f.t === 'hello') session = (f.body as Hello).session;
      for (const h of handlers) h(f);
    });
  }
  return {
    get session() { return session; },
    onFrame(h) { handlers.add(h); return () => handlers.delete(h); },
    close() { source.close(); },
  };
}
