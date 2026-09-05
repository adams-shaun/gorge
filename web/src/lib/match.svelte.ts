import type { DecisionBody, Frame, MatchStart, SeatInfo, Snapshot, View, EventBody, TableHaltedBody } from '../protocol';
import { dvrReducer, initialDvr, type DvrAction, type DvrState } from './dvr';
import { fetchView } from './api';

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

  constructor(readonly table: string) {}

  apply(f: Frame) {
    if (f.table !== this.table) return;
    switch (f.t) {
      case 'match_start':
        this.match = f.match ?? null;
        this.seats = (f.body as MatchStart).seats;
        this.view = null; // the previous match's board; wait for this one's snapshot before showing anything
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
}
