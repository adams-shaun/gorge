import type { DecisionBody, Frame, MatchStart, SeatInfo, Snapshot, View, EventBody, TableHaltedBody } from '../protocol';
import { dvrReducer, initialDvr, type DvrAction, type DvrState } from './dvr';
import { fetchEvents, fetchMatches, fetchView } from './api';
import { ViewCache } from './viewcache';
import { turnStartsFrom } from './turns';

/** MatchState is everything the focused view renders for one table. */
export class MatchState {
  match = $state<number | null>(null);
  view = $state<View | null>(null);
  seats = $state<SeatInfo[]>([]);
  dvr = $state<DvrState>(initialDvr);
  decision = $state<DecisionBody | null>(null);
  halted = $state<string | null>(null);
  private inflight = false;
  private again = false;
  private cache = new ViewCache((seq) => fetchView(this.table, this.match!, seq));
  private seeking = 0;

  constructor(readonly table: string) {}

  apply(f: Frame) {
    if (f.table !== this.table) return;
    switch (f.t) {
      case 'match_start':
        this.match = f.match ?? null;
        this.seats = (f.body as MatchStart).seats;
        this.decision = null;
        this.halted = null;
        break;
      case 'snapshot': {
        const s = f.body as Snapshot;
        this.match = f.match ?? this.match;
        this.dispatch({ type: 'snapshot', match: `${this.table}/${this.match}`, head: s.head, turnStarts: s.turn_starts });
        if (this.dvr.live) this.view = s.view;
        break;
      }
      case 'event':
        this.dispatch({ type: 'event', body: f.body as EventBody });
        break;
      case 'decision':
        this.decision = f.body as DecisionBody;
        if (this.dvr.live) void this.refreshLive();
        break;
      case 'match_end':
        this.decision = null;
        if (this.dvr.live) void this.refreshLive();
        break;
      case 'table_halted':
        this.halted = (f.body as TableHaltedBody).reason;
        break;
    }
  }

  dispatch(a: DvrAction) {
    this.dvr = dvrReducer(this.dvr, a);
    if (a.type === 'snapshot' || a.type === 'reset') this.cache.clear();
    if (!this.dvr.live && a.type !== 'event') void this.showCursor();
    if (a.type === 'live') void this.refreshLive();
  }

  /** refreshLive is PL-16: one GET per burst, coalesced. */
  async refreshLive() {
    if (this.match === null) return;
    if (this.inflight) { this.again = true; return; }
    this.inflight = true;
    try {
      const v = await fetchView(this.table, this.match, this.dvr.head);
      if (this.dvr.live) this.view = v;
    } catch { /* a 409 while the head moved: the next burst refetches */ }
    finally {
      this.inflight = false;
      if (this.again) { this.again = false; void this.refreshLive(); }
    }
  }

  /** showCursor renders the view at the cursor (paused) and backfills the transcript when the cursor precedes the known events. */
  async showCursor() {
    if (this.match === null || this.dvr.live) return;
    const seq = this.dvr.cursor;
    const token = ++this.seeking;
    const first = this.dvr.events[0]?.event.seq ?? this.dvr.head + 1;
    if (seq < first) {
      const since = Math.max(0, seq - 200);
      const older = await fetchEvents(this.table, this.match, since).catch(() => []);
      this.dispatch({ type: 'backfill', events: older.filter((e) => e.event.seq < first) });
    }
    const v = await this.cache.get(seq).catch(() => null);
    if (v && token === this.seeking) this.view = v;
  }

  /** loadFinished renders a match that is not live: no subscription, everything from the JSON GETs. */
  async loadFinished(k: number) {
    const infos = await fetchMatches(this.table);
    const info = infos.find((m) => m.match === k);
    if (!info || info.events === 0) throw new Error(`no match ${k}`);
    this.match = k;
    this.seats = info.seats;
    const all = await fetchEvents(this.table, k, 0);
    this.dispatch({ type: 'snapshot', match: `${this.table}/${k}`, head: info.events - 1, turnStarts: turnStartsFrom(all) });
    this.dispatch({ type: 'backfill', events: all });
    this.dispatch({ type: 'pause' });
    this.dispatch({ type: 'scrub', seq: info.events - 1 });
  }
}
