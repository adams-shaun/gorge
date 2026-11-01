import type { Decision, DecisionBody, EventBody, Intent, SeatInfo, View } from '../../protocol';
import { dvrReducer, initialDvr, type DvrAction, type DvrState } from '../dvr';
import type { SeatCtx, SeatTransport } from '../seat';
import { ModelBus } from '../clientmodel/gorge';
import type { ClientModelSource, ModelListener, PendingDecision, Transition } from '../clientmodel/types';
import type { WireProtocol } from './pref';
import type { WireStream } from './http';

const RING = 500;

/** A start that fails leaves nothing open. quiet is a table the wire doesn't cover: no failure worth showing. */
export type StartResult = { ok: true } | { ok: false; reason: string; quiet?: boolean };

interface RingEntry { seq: number; view: View }

/**
 * SnapshotMatch is the table route's game source for a wire that delivers
 * whole views rather than gorge's event stream. It carries the surface the
 * route reads from MatchState (match, view, seats, dvr, dispatch, halted, …)
 * and implements ClientModelSource, so the board, motion, log and seat panel
 * are unchanged. The DVR is a client-side ring of snapshots, each at a
 * synthetic seq that follows its log lines.
 *
 * It is also the seat's SeatTransport: the panel's intents, pending reads and
 * undo requests go over the wire, never the native intent route.
 */
export abstract class SnapshotMatch implements ClientModelSource, SeatTransport {
  abstract readonly protocol: WireProtocol;
  match = $state<number | null>(null);
  view = $state<View | null>(null);
  renderedSeq = $state<number | null>(null);
  seats = $state<SeatInfo[]>([]);
  // Raw, not deep: every write is a whole-object reassignment from
  // dvrReducer (see MatchState.dvr in ../match.svelte.ts for the freeze a
  // deep proxy of the growing event log caused).
  dvr = $state.raw<DvrState>(initialDvr);
  /** The spectator decision line; a wire source is always seated, so never set. */
  decision = $state<DecisionBody | null>(null);
  halted = $state<string | null>(null);
  loadError = $state<string | null>(null);
  /** The last protocol error or degradation worth showing, until dismissed or superseded. */
  notice = $state<string | null>(null);
  /** Called when an undo landed. */
  onRewind: (() => void) | null = null;

  protected model = new ModelBus();
  protected stream: WireStream | null = null;
  protected live: View | null = null;
  protected seq = 0;
  protected closed = false;
  private ring: RingEntry[] = [];

  constructor(readonly table: string, readonly seat: SeatCtx) {}

  abstract start(): Promise<StartResult>;
  abstract postIntent(t: string, k: number, intent: Intent): Promise<void>;
  abstract fetchPending(t: string, k: number): Promise<Decision>;
  abstract postUndo(t: string, k: number): Promise<void>;

  /** The seat context the panel answers through: this source is its transport. */
  get ctx(): SeatCtx {
    return { seat: this.seat.seat, token: this.seat.token, transport: this };
  }

  get clientDecision(): PendingDecision | null {
    return this.view?.decision ?? null;
  }

  onModel(fn: ModelListener): () => void {
    return this.model.on(fn);
  }

  close() {
    this.closed = true;
    this.stream?.close();
    this.stream = null;
  }

  protected seatName(s: number): string {
    return this.seats[s]?.name || this.live?.players.find((p) => p.seat === s)?.name || `Seat ${s}`;
  }

  /** The event kind a log line counts as, which decides whether the log hides it by default. */
  protected lineKind(_line: string): string {
    return this.protocol;
  }

  protected log(line: string, v: View) {
    this.seq++;
    const body: EventBody = { event: { seq: this.seq, kind: this.lineKind(line), player: v.active }, line };
    this.dvr = dvrReducer(this.dvr, { type: 'event', body });
  }

  protected pushRing(v: View) {
    const last = this.ring[this.ring.length - 1];
    if (last && last.seq === this.seq) last.view = v;
    else this.ring.push({ seq: this.seq, view: v });
    if (this.ring.length > RING) this.ring.splice(0, this.ring.length - RING);
    if (this.dvr.head < this.seq) this.dvr = dvrReducer(this.dvr, { type: 'head', seq: this.seq });
  }

  /** advance records one forward step: its log lines, a turn mark, the snapshot, and the paint. */
  protected advance(lines: string[], prev: View, next: View, transitions: Transition[]) {
    for (const line of lines) this.log(line, next);
    if (next.turn !== prev.turn) this.dvr = { ...this.dvr, turnStarts: [...this.dvr.turnStarts, this.seq - lines.length + 1] };
    this.pushRing(next);
    this.show(next, 'step', prev, transitions);
  }

  /** Replaces the newest ring snapshot, for a change that isn't a new position (a decision laid over the board). */
  protected replaceRingHead(v: View) {
    if (this.ring.length > 0) this.ring[this.ring.length - 1] = { ...this.ring[this.ring.length - 1], view: v };
  }

  /** show paints a live view: a forward step animates, anything else resets motion. */
  protected show(next: View, how: 'step' | 'reset', prev?: View, transitions: Transition[] = []) {
    if (!this.dvr.live) return;
    if (how === 'step' && prev && this.view !== null) this.model.emit({ type: 'step', prev: this.view, next, transitions });
    else this.model.emit({ type: 'reset' });
    this.assign(next);
  }

  protected assign(v: View) {
    this.view = v;
    this.renderedSeq = this.seq;
  }

  /** dispatch drives the DVR over the snapshot ring. */
  dispatch(a: DvrAction) {
    this.dvr = dvrReducer(this.dvr, a);
    if (this.dvr.live) {
      if (this.live && this.view !== this.live) {
        this.model.emit({ type: 'reset' });
        this.assign(this.live);
      }
      return;
    }
    const at = this.snapshotAt(this.dvr.cursor);
    if (at && at !== this.view) {
      this.model.emit({ type: 'reset' });
      this.view = { ...at, decision: null };
      this.renderedSeq = this.dvr.cursor;
    }
  }

  /** snapshotAt is the newest ring snapshot at or before seq. */
  snapshotAt(seq: number): View | null {
    let found: View | null = null;
    for (const e of this.ring) if (e.seq <= seq) found = e.view;
    return found ?? this.ring[0]?.view ?? null;
  }
}
