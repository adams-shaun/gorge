import type { View } from '../protocol';

/** ViewCache memoises view-at-seq fetches with a small LRU, so stepping back and forth never refetches. */
export class ViewCache {
  private done = new Map<number, View>();
  private pending = new Map<number, Promise<View>>();
  constructor(private load: (seq: number) => Promise<View>, private cap = 64) {}

  has(seq: number) { return this.done.has(seq); }
  clear() { this.done.clear(); this.pending.clear(); }

  get(seq: number): Promise<View> {
    const hit = this.done.get(seq);
    if (hit) { this.done.delete(seq); this.done.set(seq, hit); return Promise.resolve(hit); }
    const inflight = this.pending.get(seq);
    if (inflight) return inflight;
    const p = this.load(seq).then((v) => {
      this.done.set(seq, v);
      if (this.done.size > this.cap) this.done.delete(this.done.keys().next().value as number);
      return v;
    }).finally(() => this.pending.delete(seq));
    this.pending.set(seq, p);
    return p;
  }
}
