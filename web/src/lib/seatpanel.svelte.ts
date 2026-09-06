import type { Decision, Intent, Option, View } from '../protocol';
import { fetchPending, postIntent, ApiError } from './api';
import type { SeatCtx } from './seat';
import { decide, type StopReason, type Stops, type TurnSide } from './autopilot';
import { defaultStops, loadStops, saveStops, toggleStop } from './stops';

/**
 * SeatPanelState is everything a human seat answers with. It holds the
 * pending decision (adopted from view.decision, refreshed from /pending),
 * the user's picked options, the concede-confirmation and posted states,
 * and posts the intent. It is deliberately rules-ignorant (R-E4-2): the
 * options are the server's verbatim, the only selection constraints are the
 * decision's own min/max, and no option is ever chosen by position — the
 * primary button resolves its option by kind, never by index (R-E4-1), and
 * the concede option is the LAST one on the wire precisely so that a client
 * which defaulted to the last option would concede on the very first
 * priority window. This module never does that: nothing selects, preselects
 * or auto-submits an option it was not explicitly handed by a user click.
 */

/** primaryOf resolves the "primary" option by kind — pass/resolve — never by position (R-E4-1). */
export function primaryOf(d: Decision): Option | null {
  for (const o of d.options) {
    if (o.kind === 'pass' || o.kind === 'resolve') return o;
  }
  return null;
}

export function isConcede(o: Option): boolean {
  return o.kind === 'concede';
}

/**
 * Tone is how loudly the panel presents its state, and it is resolved from
 * option KINDS alone — never a label, never a position (R-E4-1).
 *
 * `offered` is a window this seat may decline: the decision carries a `pass`
 * option, so doing nothing is a legal answer and the game moves on without
 * you. `initiative` is a decision the game is blocked on — a target, a
 * mulligan, a block assignment, a mode — where there is no pass and nothing
 * happens anywhere at the table until this seat answers. Those two deserve
 * different colours because they demand different things of the player, and
 * painting them alike is what made the old panel unreadable across a room.
 */
export type Tone = 'initiative' | 'offered' | 'idle';

export function toneOf(d: Decision | null): Tone {
  if (d === null) return 'idle';
  return d.options.some((o) => o.kind === 'pass') ? 'offered' : 'initiative';
}

/**
 * MulliganPhase names which half of the London round a `mulligan` decision is
 * in, so the seat panel can lay it out. The two halves are told apart by their
 * option KINDS — `keep`/`mulligan` in the first, `bottom` in the second — and
 * never by option count, position or label text (FL-101).
 *
 * A mulligan decision carrying any other kind returns null and falls back to
 * the generic option list, so an option this layout does not understand is
 * still reachable rather than silently dropped.
 */
export type MulliganPhase =
  | { phase: 'keep'; choices: Option[] }
  | { phase: 'bottom'; cards: Option[] }
  | null;

export function mulliganPhase(d: Decision | null): MulliganPhase {
  if (d === null || d.kind !== 'mulligan' || d.options.length === 0) return null;
  if (d.options.every((o) => o.kind === 'keep' || o.kind === 'mulligan')) {
    return { phase: 'keep', choices: d.options };
  }
  if (d.options.every((o) => o.kind === 'bottom')) {
    return { phase: 'bottom', cards: d.options };
  }
  return null;
}

/**
 * AUTO_PASS_CAP bounds how many priority windows auto may pass in an
 * unbroken run before it switches itself off. A runaway autopasser is not a
 * cosmetic bug: it hammers the server and it passes the game away in
 * silence. Forty is roughly two turn cycles of an uneventful four-seat
 * game — long enough that a normal quiet stretch never trips it, short
 * enough that a stuck loop is caught in seconds.
 */
export const AUTO_PASS_CAP = 40;

/**
 * AutoOffReason is why auto is no longer running, as distinct from
 * StopReason (why auto declined THIS window but stays armed). The two are
 * separate vocabularies because they need separate words on screen: one is
 * "waiting for you here", the other is "auto is off now".
 */
export type AutoOffReason = 'loop' | 'cap' | 'human' | 'escape';

/**
 * AutoNote is the one line the panel shows about what auto is doing. It is
 * an enum-shaped value, never a string to print: autoNoteText turns it into
 * words, so no StopReason identifier can reach the screen.
 */
export type AutoNote =
  | { kind: 'off' }
  | { kind: 'armed' }
  | { kind: 'passing'; count: number }
  | { kind: 'waiting'; reason: StopReason }
  | { kind: 'stopped'; reason: AutoOffReason };

const WAITING_TEXT: Record<StopReason, string> = {
  'disabled': 'Auto is off.',
  'not-priority': 'Auto is waiting: this decision needs you, not a pass.',
  'unexpected-shape': 'Auto is waiting: it does not recognise this window.',
  'stop-set': 'Auto stopped here: you set a stop on this step.',
  'has-action-and-stack': 'Auto stopped here: something is on the stack and you can respond.',
};

const OFF_TEXT: Record<AutoOffReason, string> = {
  'loop': 'Auto switched itself off: the same decision came back after it answered.',
  'cap': `Auto switched itself off after ${AUTO_PASS_CAP} passes in a row.`,
  'human': 'Auto switched off: you took the decision yourself.',
  'escape': 'Auto switched off: you pressed Escape.',
};

/** autoNoteText renders an AutoNote as plain words. No enum identifier ever reaches the screen. */
export function autoNoteText(note: AutoNote): string {
  switch (note.kind) {
    case 'off':
      return 'Auto is off. You answer every window.';
    case 'armed':
      return 'Auto is on. It passes windows where you have nothing to do, and stops at your stops.';
    case 'passing':
      return note.count === 1
        ? 'Auto passed 1 priority window.'
        : `Auto passed ${note.count} priority windows.`;
    case 'waiting':
      return WAITING_TEXT[note.reason];
    case 'stopped':
      return OFF_TEXT[note.reason];
  }
}

/** safeStorage is localStorage where it exists and is reachable; null under SSR and in a browser that refuses site data. Same guard as images.ts. */
function safeStorage(): Storage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage;
  } catch {
    return null;
  }
}

export class SeatPanelState {
  readonly table: string;
  readonly ctx: SeatCtx;
  private readonly storage: Storage | null;

  constructor(table: string, readonly match: number, ctx: SeatCtx, storage: Storage | null = safeStorage()) {
    this.table = table;
    this.ctx = ctx;
    this.storage = storage;
  }

  /** pending is the decision this seat must answer right now, or null when the game is waiting on someone else. */
  pending = $state<Decision | null>(null);
  /** picked holds the chosen option indices, in click order: for a permutation decision the ORDER is the answer (Chosen returns options in client order), so picks must never be reordered. */
  picked = $state<number[]>([]);
  /** postedSeq is the seq of the last intent the server accepted; while the pending decision has this seq the answer is in and the options stay hidden. */
  postedSeq = $state<number | null>(null);
  /** confirming arms the concede option's required second confirmation (R-E4-1). */
  confirming = $state(false);
  /** error surfaces a rejected intent — never swallowed (a stale seq must be seen and recovered from, not silently dropped). */
  error = $state<string | null>(null);
  busy = $state(false);

  // ---- autopilot ------------------------------------------------------
  //
  // decide() (lib/autopilot) is pure and already tested; what lives here is
  // the LOOP around it, which is the dangerous half. A mis-firing
  // autopasser loses a game in silence, so every field below exists to make
  // it stop rather than to make it go.

  /** auto is the player's opt-in. It starts OFF on every load and is never persisted: a seat that comes back to a page must choose to hand the game over again. */
  auto = $state(false);
  /** stops is this seat's per-step stop set, loaded from storage on mount and saved on every toggle. */
  stops = $state<Stops>(defaultStops());
  /** autoPassed counts every window auto has answered this session, so the pass is visible after the fact. */
  autoPassed = $state(0);
  /** autoRun is the current unbroken run of auto-passes; the cap is on this, not on the session total. */
  autoRun = $state(0);
  /** note is what the panel says about auto, as a value — autoNoteText turns it into words. */
  note = $state<AutoNote>({ kind: 'off' });
  /**
   * autoActedSeq is the seq auto last posted for. If a decision with that
   * seq is put in front of auto again, the answer did not take and auto
   * would post it forever: that is the loop guard, and it disables auto.
   */
  private autoActedSeq: number | null = null;

  /** mountStops loads this seat's saved stops. Called from the component on mount, where storage exists. */
  mountStops() {
    this.stops = loadStops(this.storage, this.table, this.ctx.seat);
  }

  /**
   * toggleStop flips one step's stop on one turn side and persists it. The
   * two sides are separate sets on purpose: stopping in your own combat and
   * stopping in an opponent's are different intentions, and a stop set on
   * one side must never mark the other.
   */
  toggleStop(step: string, side: TurnSide) {
    const next = toggleStop(this.stops, side, step);
    if (next === this.stops) return; // a step that cannot take a stop
    this.stops = next;
    saveStops(this.storage, this.table, this.ctx.seat, next);
  }

  /** setAuto is the Auto/Manual control. Turning it on clears the previous run so an old count never trips the cap. */
  setAuto(on: boolean) {
    this.auto = on;
    this.autoRun = 0;
    this.autoActedSeq = null;
    this.note = on ? { kind: 'armed' } : { kind: 'off' };
  }

  /** suspendAuto switches auto off with a stated reason. A human always wins: any answer this seat gives by hand takes the wheel back. */
  suspendAuto(reason: AutoOffReason) {
    if (!this.auto) return;
    this.auto = false;
    this.autoRun = 0;
    this.autoActedSeq = null;
    this.note = { kind: 'stopped', reason };
  }

  /** onKeydown is the panel's key handler: Escape, and only Escape, suspends auto. */
  onKeydown(key: string) {
    if (key === 'Escape') this.suspendAuto('escape');
  }

  /**
   * considerAuto is the whole autopilot loop, run once per decision/view
   * change. Order matters and every early return is a refusal to act:
   * nothing pending, in flight, already answered, seen before (loop), the
   * run cap, then and only then decide().
   */
  considerAuto(view: View) {
    if (!this.auto) return;
    const d = this.pending;
    if (d === null || this.busy || d.seq === this.postedSeq) return;

    // Loop guard: auto already answered this seq and here it is again. The
    // answer did not take, so posting it a second time is the start of an
    // unbounded retry against the server.
    if (this.autoActedSeq !== null && d.seq === this.autoActedSeq) {
      this.suspendAuto('loop');
      return;
    }

    const verdict = decide({ decision: d, view, seat: this.ctx.seat, stops: this.stops, enabled: true });
    if (verdict.act === 'stop') {
      // A stop is a hand-back, not a failure: auto stays armed and the run
      // resets, because the player is about to look at this window.
      this.autoRun = 0;
      this.note = { kind: 'waiting', reason: verdict.reason };
      return;
    }

    if (this.autoRun >= AUTO_PASS_CAP) {
      this.suspendAuto('cap');
      return;
    }

    this.autoActedSeq = d.seq;
    this.autoRun += 1;
    this.autoPassed += 1;
    this.note = { kind: 'passing', count: this.autoPassed };
    void this.post([verdict.index]);
  }

  /** begin resets the seat across a match boundary. (The component keys the panel by match, so a new match is a fresh instance — this is belt and braces.) */
  begin() {
    this.pending = null;
    this.picked = [];
    this.postedSeq = null;
    this.confirming = false;
    this.error = null;
    // A new match is a new opt-in: auto never carries across a match
    // boundary on its own.
    this.auto = false;
    this.autoRun = 0;
    this.autoPassed = 0;
    this.autoActedSeq = null;
    this.note = { kind: 'off' };
  }

  /**
   * adoptView synchronises with the parent's view: the seat-scoped view at
   * head carries the decision asked of this seat (and only this seat's).
   * A null decision means the wait is on someone else, or the game is
   * resolving. A decision whose seq we already answered is ignored — a
   * stale view must not re-open answered options.
   */
  adoptView(d: Decision | null) {
    this.adopt(d);
  }

  private adopt(d: Decision | null) {
    if (d === null) {
      this.pending = null;
      return;
    }
    if (this.postedSeq !== null && d.seq === this.postedSeq) return;
    if (this.pending?.seq === d.seq) return;
    this.pending = d;
    this.postedSeq = null;
    this.picked = [];
    this.confirming = false;
  }

  primary(): Option | null {
    return this.pending !== null && this.pending.seq !== this.postedSeq ? primaryOf(this.pending) : null;
  }

  get showSubmit(): boolean {
    const d = this.pending;
    return d !== null && d.seq !== this.postedSeq && d.max > 1;
  }

  /** canSubmit gates the submit button on the decision's OWN min/max — the only selection constraints the client may enforce (R-E4-2). */
  get canSubmit(): boolean {
    const d = this.pending;
    return d !== null && this.picked.length >= d.min && this.picked.length <= d.max;
  }

  /** click handles one option click. A single-required-option decision posts immediately (the click IS the answer); a multi-pick toggles into `picked` for submit. The concede option never posts on the first click (R-E4-1). */
  click(index: number) {
    const d = this.pending;
    if (d === null || d.seq === this.postedSeq || this.busy) return;
    const opt = d.options[index];
    if (opt === undefined) return;
    // A human always wins: touching an option takes the wheel back before
    // anything is posted, so auto cannot answer the next window either.
    this.suspendAuto('human');
    if (isConcede(opt)) {
      if (this.confirming) void this.post([index]);
      else this.confirming = true;
      return;
    }
    this.confirming = false;
    if (d.min === 1 && d.max === 1) {
      void this.post([index]);
      return;
    }
    const at = this.picked.indexOf(index);
    if (at >= 0) this.picked = this.picked.filter((i) => i !== index);
    else this.picked = [...this.picked, index];
  }

  /**
   * toggle adds or removes one option from `picked` and never posts. click()
   * posts straight away on a min==max==1 decision because there the click IS
   * the answer, which is right for a priority option and wrong for bottoming:
   * bottoming one card has exactly that shape and is irreversible, so those
   * cards toggle and the player commits with the submit button.
   */
  toggle(index: number) {
    const d = this.pending;
    if (d === null || d.seq === this.postedSeq || this.busy) return;
    if (d.options[index] === undefined) return;
    this.suspendAuto('human');
    this.confirming = false;
    const at = this.picked.indexOf(index);
    if (at >= 0) this.picked = this.picked.filter((i) => i !== index);
    else this.picked = [...this.picked, index];
  }

  /** primaryClick posts the primary-by-kind option directly. */
  primaryClick() {
    const d = this.pending;
    const p = d ? primaryOf(d) : null;
    if (d === null || p === null || d.seq === this.postedSeq || this.busy) return;
    this.suspendAuto('human');
    void this.post([p.index]);
  }

  /** confirmConcede posts the armed concede option — the second, explicit confirmation. */
  confirmConcede() {
    const d = this.pending;
    if (d === null || !this.confirming || this.busy) return;
    const idx = d.options.findIndex(isConcede);
    if (idx < 0) return;
    this.suspendAuto('human');
    void this.post([idx]);
  }

  /** submit posts the picked set — gated on min/max; a rejected answer is recovered from, never treated as impossible. */
  submit() {
    const d = this.pending;
    if (d === null || d.seq === this.postedSeq || this.busy) return;
    if (this.picked.length < d.min || this.picked.length > d.max) return;
    this.suspendAuto('human');
    void this.post([...this.picked]);
  }

  private async post(choices: number[]) {
    const d = this.pending;
    if (d === null || this.busy) return;
    this.busy = true;
    this.error = null;
    try {
      await postIntent(this.table, this.match, { seq: d.seq, player: d.player, choices } satisfies Intent, this.ctx);
      this.postedSeq = d.seq;
      // If a new decision was adopted while the intent was in flight (a
      // rapid successive ask), keep it; only drop the decision we answered.
      if (this.pending?.seq === d.seq) this.pending = null;
      this.picked = [];
      this.confirming = false;
    } catch (e) {
      // Surfaced, not swallowed: the intent was rejected (a stale seq, a
      // race, a refusal) and the game is exactly where it was — recover by
      // adopting the CURRENT decision rather than wedging on the stale one.
      this.picked = [];
      this.confirming = false;
      this.error = e instanceof Error ? e.message : String(e);
      void this.refreshPending();
    } finally {
      this.busy = false;
    }
  }

  /** refreshPending re-reads the current decision from /pending: the recovery path after a rejection, and the not-yet-viewed case at mount. A 409 conflict IS the normal "nothing pending" answer (the wait is on someone else), not an error. */
  async refreshPending() {
    try {
      const d = await fetchPending(this.table, this.match, this.ctx);
      this.adopt(d);
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        this.adopt(null);
        return;
      }
      if (this.error === null) this.error = e instanceof Error ? e.message : String(e);
    }
  }
}
