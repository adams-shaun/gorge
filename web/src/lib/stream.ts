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

// Every frame kind the session stream may carry, in one place. A new kind in
// the generated FrameType union must be added here: the exhaustiveness guard
// below fails the build otherwise, so a kind can never be silently dropped
// the way table_state was by the old hand-maintained list. The four
// draft-event frames (event_state, draft_pack, pod_progress, series_update)
// ride the same session stream once POST /api/events/{id}/subscribe binds the
// session to an event.
const FRAME_TYPES = [
  'hello', 'widget', 'match_start', 'snapshot', 'event', 'decision', 'match_end', 'table_halted', 'table_state', 'overflow', 'error', 'rewind',
  'event_state', 'draft_pack', 'pod_progress', 'series_update',
] as const;
type KnownFrameType = (typeof FRAME_TYPES)[number];
// Compile-time guard: if protocol.ts declares a frame kind this list omits,
// the Exclude stops being never and this initialiser fails to type-check.
const frameTypesCoverEveryKind: [Exclude<FrameType, KnownFrameType>] extends [never] ? true : false = true;
void frameTypesCoverEveryKind;

/** parseFrame decodes one SSE data line; anything malformed or from another protocol version is dropped. */
export function parseFrame(data: string): Frame | null {
  try {
    const f = JSON.parse(data) as Frame;
    if (f.v !== 1 || !(FRAME_TYPES as readonly string[]).includes(f.t)) return null;
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
