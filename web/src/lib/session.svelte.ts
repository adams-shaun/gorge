import { openStream, type Stream } from './stream';
import { subscribe, unsubscribe } from './api';
import type { Frame } from '../protocol';

/**
 * One stream per page. Every hello (first connect or a server-side restart
 * of the session) re-issues the subscriptions this page holds, so a
 * reconnect is invisible to the routes.
 */
class Session {
  readonly stream: Stream;
  private wantOverview = false;
  private focused = new Set<string>();
  id = $state<string | null>(null);

  constructor() {
    this.stream = openStream('/api/stream');
    this.stream.onFrame((f: Frame) => {
      if (f.t !== 'hello') return;
      this.id = this.stream.session;
      void this.resubscribe();
    });
  }
  private async resubscribe() {
    if (!this.id) return;
    if (this.wantOverview) await subscribe(this.id, '*', 'overview').catch(() => {});
    for (const t of this.focused) await subscribe(this.id, t, 'focus').catch(() => {});
  }
  ensureOverview() {
    if (this.wantOverview) return;
    this.wantOverview = true;
    if (this.id) void subscribe(this.id, '*', 'overview').catch(() => {});
  }
  async focus(table: string) {
    this.focused.add(table);
    if (this.id) await subscribe(this.id, table, 'focus');
  }
  async unfocus(table: string) {
    this.focused.delete(table);
    if (this.id) await unsubscribe(this.id, table).catch(() => {});
  }
}

export const session = new Session();
