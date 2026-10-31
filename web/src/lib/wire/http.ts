import type { EventSourceLike } from '../stream';

/**
 * http.ts is what the opt-in wires' transports share: a bounded fetch, the
 * refused-request error, and one EventSource on a `data:`-only stream. The
 * browser reconnects an EventSource by itself. EventSource cannot set headers,
 * so a stream's token rides as `?token=`.
 */

const TIMEOUT = 10_000;

/** WireError is a refused request: a protocol error from a route, or an HTTP-layer refusal (claim, route). */
export class WireError extends Error {
  constructor(public status: number, public code: string, message: string, public promptId?: number) {
    super(message);
  }
}

export async function bounded(url: string, init: RequestInit = {}): Promise<Response> {
  const control = new AbortController();
  const timer = setTimeout(() => control.abort(), TIMEOUT);
  try {
    return await fetch(url, { ...init, signal: control.signal });
  } catch (e) {
    if (control.signal.aborted) throw new WireError(0, 'timeout', `request timed out after ${TIMEOUT}ms`);
    throw new WireError(0, 'network', e instanceof Error ? e.message : String(e));
  } finally {
    clearTimeout(timer);
  }
}

export type ESCtor = new (url: string) => EventSourceLike;

export interface WireStream {
  close(): void;
}

/** openStream owns one EventSource; its frames carry no SSE event name, so they arrive as `message`. */
export function openStream(url: string, onData: (data: string) => void, onError: () => void, es: ESCtor = EventSource as unknown as ESCtor): WireStream {
  const source = new es(url);
  source.addEventListener('message', (e: MessageEvent) => onData(String(e.data)));
  source.addEventListener('error', () => onError());
  return { close: () => source.close() };
}
