import { openStream, type Stream } from './stream';
import { subscribe, unsubscribe } from './api';
import { withBase } from './basepath';
import type { Frame } from '../protocol';

/**
 * One stream per page. Every hello (first connect or a server-side restart
 * of the session) re-issues the subscriptions this page holds, so a
 * reconnect is invisible to the routes.
 */
const RESUBSCRIBE_TRIES = 4;
const RESUBSCRIBE_BACKOFF_MS = 500;

/** delay is the resubscribe backoff's sleep, exported-by-injection for the test seam below. */
let delay = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

/** setResubscribeDelay replaces the backoff sleep so a test need not wait real time. It returns the previous one. */
export function setResubscribeDelay(fn: (ms: number) => Promise<void>): (ms: number) => Promise<void> {
  const previous = delay;
  delay = fn;
  return previous;
}

class Session {
  readonly stream: Stream;
  private wantOverview = false;
  private focused = new Set<string>();
  #id = $state<string | null>(null);
  get id(): string | null { return this.#id; }

  constructor() {
    // The SSE stream is just another same-origin request path and rides the
    // same base as everyone else.
    this.stream = openStream(withBase('/api/stream'));
    this.stream.onFrame((f: Frame) => {
      if (f.t !== 'hello') return;
      this.#id = this.stream.session;
      void this.resubscribe();
    });
  }
  /**
   * resubscribe re-issues this page's subscriptions against the current
   * session id. It RETRIES, because a swallowed failure here is invisible and
   * permanent: the SSE stream stays open and healthy-looking while the server
   * sends this session nothing for the table, and `focus()` is idempotent on
   * `this.focused`, so no later effect re-fire can repair it. The page then
   * holds its last painted board until the player reloads. The most likely
   * failure is exactly the one that caused the reconnect — a starved or timed
   * out request (see STATE_TIMEOUT in ./api) — so one retry pass is usually
   * enough, and the backoff bounds the noise when the server is genuinely
   * unreachable.
   */
  private async resubscribe(attempt = 0) {
    if (!this.#id) return;
    const id = this.#id;
    const failed: Array<[string, 'overview' | 'focus']> = [];
    if (this.wantOverview && !(await this.trySubscribe(id, '*', 'overview'))) failed.push(['*', 'overview']);
    for (const t of this.focused) {
      if (!(await this.trySubscribe(id, t, 'focus'))) failed.push([t, 'focus']);
    }
    if (failed.length === 0 || attempt >= RESUBSCRIBE_TRIES - 1) return;
    // A hello for a NEWER session has already re-entered this method; that
    // run owns the subscriptions now and this one must not fight it.
    await delay(RESUBSCRIBE_BACKOFF_MS * 2 ** attempt);
    if (this.#id !== id) return;
    await this.resubscribe(attempt + 1);
  }

  private async trySubscribe(id: string, table: string, mode: 'overview' | 'focus'): Promise<boolean> {
    try {
      await subscribe(id, table, mode);
      return true;
    } catch {
      return false;
    }
  }
  ensureOverview() {
    if (this.wantOverview) return;
    this.wantOverview = true;
    if (this.#id) void subscribe(this.#id, '*', 'overview').catch(() => {});
  }
  async focus(table: string) {
    if (this.focused.has(table)) return; // idempotent: a re-firing effect must not re-subscribe (the host pushes a snapshot per subscribe)
    this.focused.add(table);
    if (this.#id) await subscribe(this.#id, table, 'focus');
  }
  async unfocus(table: string) {
    this.focused.delete(table);
    if (this.#id) await unsubscribe(this.#id, table).catch(() => {});
  }
}

export const session = new Session();
