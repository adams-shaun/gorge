import type { Decision, Intent, Option, View } from '../protocol';
import { fetchPending, postIntent, ApiError } from './api';
import type { SeatCtx } from './seat';
import { decide, emptyPriorityWindow, type StopReason, type Stops, type TurnSide } from './autopilot';
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

/**
 * pickOption is the pure heart of the seat's selection logic: it applies one
 * click identified by the option's wire index to the current picked index set and returns the
 * resulting set. It is expressed ONLY in terms of the decision's own min/max
 * (applied elsewhere) and the option Group field, never in terms of what a
 * Group's members are (R-E4-2) — the panel never learns what a blocker is.
 *
 *  - If the clicked option is already picked, it is removed (toggle off).
 *  - Otherwise, if it carries a non-empty Group already represented in
 *    `picked`, that previously-picked group member is REPLACED by the new
 *    option: moving one blocker from attacker A to attacker B just works,
 *    and at most one option of a Group is ever held.
 *  - Otherwise it is appended.
 */
export function pickOption(d: Decision, index: number, picked: number[]): number[] {
  const opt = optionAt(d, index);
  if (opt === undefined) return [...picked];
  const at = picked.indexOf(index);
  if (at >= 0) return picked.filter((i) => i !== index);
  const g = opt.group;
  if (g) {
    const existing = picked.find((i) => optionAt(d, i)?.group === g);
    if (existing !== undefined) {
      // Replace: drop the old group member, keep the rest's click order,
      // and put the freshly picked option at the end.
      return picked.filter((i) => i !== existing).concat(index);
    }
  }
  return [...picked, index];
}

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

/** optionAt resolves a wire option by its own index, never by array position (R-E4-1). */
function optionAt(d: Decision, index: number): Option | undefined {
  return d.options.find((o) => o.index === index);
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
  | { kind: 'skip-off'; reason: AutoOffReason }
  | { kind: 'skipped'; count: number }
  | { kind: 'armed' }
  | { kind: 'passing'; count: number }
  | { kind: 'waiting'; reason: StopReason }
  | { kind: 'stopped'; reason: AutoOffReason }
  | { kind: 'fast-armed' }
  | { kind: 'fast-passing'; count: number }
  | { kind: 'fast-stopped'; reason: StopReason | AutoOffReason }
  | { kind: 'fast-cancelled' };

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
      return 'Auto is off. You answer every window that offers you something to do.';
    case 'skip-off':
      return `${OFF_TEXT[note.reason]} Empty windows are no longer skipped either.`;
    case 'skipped':
      return note.count === 1
        ? 'Passed 1 window where you had nothing to do.'
        : `Passed ${note.count} windows where you had nothing to do.`;
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
    case 'fast-armed':
      return 'Fast forward is running until the next pause point.';
    case 'fast-passing':
      return note.count === 1
        ? 'Fast forward passed 1 priority window.'
        : `Fast forward passed ${note.count} priority windows.`;
    case 'fast-stopped':
      return note.reason in WAITING_TEXT
        ? WAITING_TEXT[note.reason as StopReason].replace('Auto', 'Fast forward')
        : OFF_TEXT[note.reason as AutoOffReason].replace('Auto', 'Fast forward');
    case 'fast-cancelled':
      return 'Fast forward cancelled: you took the controls.';
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
  /**
   * skipEmpty is the empty-window floor, and it is ON by default -- the one
   * thing the panel does for the player without being asked. A priority
   * window whose only options are pass and concede asks nothing: there is no
   * action to take, so collecting a click there is pure friction between the
   * player and the next real decision. It is separate from `auto` because it
   * is a different promise: auto decides FOR you (which is why it is opt-in
   * and never persisted), whereas this only declines to interrupt you when
   * there was nothing to decide. Turning it off restores the old
   * stop-at-every-window behaviour.
   */
  skipEmpty = $state(true);

  /** autoPassed counts every window auto has answered this session, so the pass is visible after the fact. */
  autoPassed = $state(0);

  /** emptySkipped counts the no-action windows the floor passed, kept apart from autoPassed so the panel never credits auto with a pass it did not make. */
  emptySkipped = $state(0);
  /** autoRun is the current unbroken run of machine passes; both Auto and one-shot fast-forward share its hard cap. */
  autoRun = $state(0);
  /** fastForward is the one-shot run: unlike Auto it switches off on every decide() stop verdict. */
  fastForward = $state(false);
  /** fastPassed is the current one-shot's visible pass count. */
  fastPassed = $state(0);
  /** note is what the panel says about automatic action, as a value — autoNoteText turns it into words. */
  note = $state<AutoNote>({ kind: 'off' });
  /**
   * autoActedSeq is the seq auto last posted for. If a decision with that
   * seq is put in front of auto again, the answer did not take and auto
   * would post it forever: that is the loop guard, and it disables auto.
   */
  private autoActedSeq: number | null = null;
  /** The player's own set-stop window where the last fast-forward run handed control back. */
  private fastStoppedSeq: number | null = null;
  /** One restart may acknowledge exactly that stopped window; it is consumed before posting. */
  private fastAcknowledgedSeq: number | null = null;
  /**
   * autoStoppedSeq is the seq of the window where persistent Auto stopped for
   * the player's OWN set stop (decide()'s stop-set verdict). Answering exactly
   * that window by hand is what the stop exists to invite, so it does not take
   * the wheel: the token is consumed by that one answer and a fresh one is
   * minted at the next stop-set stop. Any other stop reason mints nothing, and
   * every path that switches Auto off clears the token, so it can never leak
   * to a differently-stopped window or outlive its own.
   */
  private autoStoppedSeq: number | null = null;

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
    this.cancelFastForward();
    const next = toggleStop(this.stops, side, step);
    if (next === this.stops) return; // a step that cannot take a stop
    this.stops = next;
    saveStops(this.storage, this.table, this.ctx.seat, next);
  }

  /** setAuto is the Auto/Manual control. Turning it on clears the previous run so an old count never trips the cap. */
  setAuto(on: boolean) {
    this.cancelFastForward(false);
    this.auto = on;
    this.autoRun = 0;
    this.autoActedSeq = null;
    // A fresh arm is a fresh run: no stop's keep-armed exception carries
    // across it.
    this.autoStoppedSeq = null;
    this.note = on ? { kind: 'armed' } : { kind: 'off' };
  }

  /** setSkipEmpty is the empty-window floor's control. Turning it back on clears the run so an old count never trips the cap. */
  setSkipEmpty(on: boolean) {
    this.cancelFastForward();
    this.skipEmpty = on;
    this.autoRun = 0;
    this.autoActedSeq = null;
    if (!this.auto) this.note = { kind: 'off' };
  }

  /**
   * stopActing is the shared guard exit for the loop guard and the pass cap.
   * Whichever mechanism was acting is the one switched off: auto if auto was
   * driving, otherwise the empty-window floor. Both are runaway protections
   * and both have to be able to actually stop the thing that is running --
   * before the floor existed, suspendAuto returned silently when auto was
   * already off, which would have left a wedged floor posting forever.
   */
  private stopActing(reason: AutoOffReason) {
    if (this.fastForward) {
      this.fastForward = false;
      this.autoRun = 0;
      this.autoActedSeq = null;
      this.note = { kind: 'fast-stopped', reason };
      return;
    }
    if (this.auto) {
      this.suspendAuto(reason);
      return;
    }
    this.skipEmpty = false;
    this.autoRun = 0;
    this.autoActedSeq = null;
    this.note = { kind: 'skip-off', reason };
  }

  /** Start a bounded one-shot run. It uses decide(), but never changes the persistent Auto mode. */
  startFastForward() {
    if (this.busy) return;
    this.auto = false;
    this.fastForward = true;
    this.fastPassed = 0;
    this.autoRun = 0;
    this.autoActedSeq = null;
    // Fast forward runs on its own acknowledgement machinery; Auto's stop
    // exception has no owner while the mode is off.
    this.autoStoppedSeq = null;
    // Restarting on the set stop that ended the previous run means "I have
    // seen this one; continue". No other stop reason earns this token.
    this.fastAcknowledgedSeq = this.pending?.seq === this.fastStoppedSeq ? this.fastStoppedSeq : null;
    this.note = { kind: 'fast-armed' };
  }

  /** Any other pointer/key/answer hands control back immediately. */
  cancelFastForward(say = true) {
    if (!this.fastForward) return;
    this.fastForward = false;
    this.autoRun = 0;
    this.autoActedSeq = null;
    this.fastAcknowledgedSeq = null;
    if (say) this.note = { kind: 'fast-cancelled' };
  }

  /**
   * handAnswer is the takeover step every human answering path runs before it
   * posts. A human always wins — with the one exception the stops feature
   * exists to create: answering the window Auto stopped at for the player's
   * own set stop keeps Auto armed, because that answer is what the stop
   * invited, and Auto resumes from the next window. The exception is scoped
   * to that one seq (consumed by the answer, re-minted at the next stop-set
   * stop); a window Auto stopped at for any other reason, and any window
   * with no stop pending, still disarms.
   */
  private handAnswer() {
    this.cancelFastForward();
    if (this.autoStoppedSeq !== null && this.pending?.seq === this.autoStoppedSeq) {
      this.autoStoppedSeq = null;
      return;
    }
    this.suspendAuto('human');
  }

  /** suspendAuto switches auto off with a stated reason. A human always wins: any answer this seat gives by hand takes the wheel back. */
  suspendAuto(reason: AutoOffReason) {
    if (!this.auto) return;
    this.auto = false;
    this.autoRun = 0;
    this.autoActedSeq = null;
    this.autoStoppedSeq = null;
    this.note = { kind: 'stopped', reason };
  }

  /** onKeydown is the panel's key handler: Escape, and only Escape, suspends auto. */
  onKeydown(key: string) {
    if (key !== 'Escape') return;
    this.cancelFastForward();
    this.suspendAuto('escape');
  }

  /**
   * considerAuto is the whole autopilot loop, run once per decision/view
   * change. Order matters and every early return is a refusal to act:
   * nothing pending, in flight, already answered, seen before (loop), the
   * run cap, then and only then decide().
   */
  considerAuto(view: View) {
    const d = this.pending;
    if (d === null || this.busy || d.seq === this.postedSeq) return;

    // The empty-window floor runs whether or not auto is on, so a Manual
    // seat is still not stopped at a window that asks nothing. When auto IS
    // on this is redundant -- decide()'s own !actionable branch reaches the
    // same pass -- and that is the point: one shape test, two callers.
    const emptyIndex = this.skipEmpty ? emptyPriorityWindow(d) : null;
    if (!this.auto && !this.fastForward && emptyIndex === null) return;

    // Loop guard: we already answered this seq and here it is again. The
    // answer did not take, so posting it a second time is the start of an
    // unbounded retry against the server.
    if (this.autoActedSeq !== null && d.seq === this.autoActedSeq) {
      this.stopActing('loop');
      return;
    }

    let index: number;
    if (this.auto || this.fastForward) {
      // ffwd marks the one-shot run: it passes through has-action-and-stack
      // windows (pressing FFWD is the player's own "no more actions"), while
      // persistent Auto keeps that guard.
      const verdict = decide({ decision: d, view, seat: this.ctx.seat, stops: this.stops, enabled: true, ffwd: this.fastForward });
      if (verdict.act === 'stop') {
        // decide() remains the safety oracle. The caller may acknowledge only
        // the player's own set-stop verdict, only at the exact seq where the
        // previous run stopped, and consumes that acknowledgement now so it
        // cannot leak to the next window. decide() already proved this is the
        // understood one-pass-option priority shape, so passOption supplies
        // the wire index without relying on list position.
        const acknowledged = this.fastForward
          && verdict.reason === 'stop-set'
          && d.seq === this.fastAcknowledgedSeq;
        if (acknowledged) {
          this.fastAcknowledgedSeq = null;
          this.fastStoppedSeq = null;
          const pass = this.passOption;
          if (pass === null) return;
          index = pass.index;
        } else {
          this.autoRun = 0;
          if (this.fastForward) {
            // Every safety stop ends the run. Only stop-set records a seq that
            // a deliberate restart may acknowledge; all other reasons must
            // stop dead again on every restart.
            this.fastForward = false;
            this.fastStoppedSeq = verdict.reason === 'stop-set' ? d.seq : null;
            this.fastAcknowledgedSeq = null;
            this.autoActedSeq = null;
            this.note = { kind: 'fast-stopped', reason: verdict.reason };
          } else {
            // A stop in persistent Auto leaves the mode armed: the player may
            // answer this window and Auto resumes after it. Only the player's
            // own set-stop verdict earns the keep-armed exception, and only for
            // exactly this window: the token is consumed by the hand answer and
            // re-minted fresh at the next stop-set stop. Any other reason means
            // the client saw something it does not understand, so a hand answer
            // there must still take the wheel.
            this.autoStoppedSeq = verdict.reason === 'stop-set' ? d.seq : null;
            this.note = { kind: 'waiting', reason: verdict.reason };
          }
          return;
        }
      } else {
        index = verdict.index;
      }
    } else {
      index = emptyIndex as number;
    }

    // The cap bounds an unbroken run of machine-made passes, and it bounds
    // the floor for the same reason it bounds auto: a stuck window answered
    // forever is a denial of service the player never asked for.
    if (this.autoRun >= AUTO_PASS_CAP) {
      this.stopActing('cap');
      return;
    }

    this.autoActedSeq = d.seq;
    this.autoRun += 1;
    if (this.fastForward) {
      this.fastPassed += 1;
      this.note = { kind: 'fast-passing', count: this.fastPassed };
    } else if (this.auto) {
      this.autoPassed += 1;
      this.note = { kind: 'passing', count: this.autoPassed };
    } else {
      this.emptySkipped += 1;
      this.note = { kind: 'skipped', count: this.emptySkipped };
    }
    void this.post([index]);
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
    this.fastForward = false;
    this.autoRun = 0;
    this.autoPassed = 0;
    this.fastPassed = 0;
    this.fastStoppedSeq = null;
    this.fastAcknowledgedSeq = null;
    this.autoStoppedSeq = null;
    // The floor is a preference, not an opt-in, so it comes back on across a
    // match boundary the way it starts: on. Only its runaway guards or the
    // player's own switch turn it off.
    this.skipEmpty = true;
    this.emptySkipped = 0;
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

  /** passOption is the dedicated HUD action, resolved by kind and carrying its own wire index. */
  get passOption(): Option | null {
    const d = this.active;
    if (d?.kind !== 'priority') return null;
    return d.options.find((o) => o.kind === 'pass') ?? null;
  }

  /** concedeOption is rendered by the page-level quiet control, never in the action list. */
  get concedeOption(): Option | null {
    const d = this.active;
    return d?.options.find(isConcede) ?? null;
  }

  /**
   * active is the decision this seat must answer right now — pending, and
   * not already posted. The board indexes THIS to mark cards with options
   * and to hang each card's options on its tile, and it is exactly the
   * decision the seat panel surfaces, so the board and the panel cannot
   * disagree about what this seat is being asked (and, once answered, both
   * drop it together).
   */
  get active(): Decision | null {
    return this.pending !== null && this.pending.seq !== this.postedSeq ? this.pending : null;
  }

  /**
   * showSubmit renders the commit button for every decision a single click
   * cannot answer — which is exactly the complement of click()'s post-on-click
   * shape (min == max == 1, where the click IS the answer and a submit button
   * would be a redundant second control).
   *
   * The gate used to be `d.max > 1`, which deadlocked a Min 0 / Max 1
   * decision: declare-attackers on a 1v1 board with exactly one creature able
   * to attack. click() correctly declined to post it (min is 0, so a click is
   * a selection and not an answer), the pick sat in `picked`, and no button
   * was ever drawn to commit it or to decline — the seat could select its
   * attacker and then had no way forward at all. Attacking with two creatures
   * was fine, which is why it went unseen. KAttackers also carries no `pass`
   * or `resolve` option, so the primary-by-kind button (R-E4-1) is not there
   * to fall back on.
   */
  get showSubmit(): boolean {
    const d = this.pending;
    return d !== null && d.seq !== this.postedSeq && !(d.min === 1 && d.max === 1);
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
    const opt = optionAt(d, index);
    if (opt === undefined) return;
    if (isConcede(opt)) {
      // Concede never earns the stop's keep-armed exception — it is not an
      // answer the stop existed to invite, and Auto must not survive it.
      this.cancelFastForward();
      this.suspendAuto('human');
      if (this.confirming) void this.post([index]);
      else this.confirming = true;
      return;
    }
    this.handAnswer();
    this.confirming = false;
    if (d.min === 1 && d.max === 1) {
      void this.post([index]);
      return;
    }
    this.picked = pickOption(d, index, this.picked);
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
    if (optionAt(d, index) === undefined) return;
    this.handAnswer();
    this.confirming = false;
    this.picked = pickOption(d, index, this.picked);
  }

  /** passClick posts only the pass-by-kind option, using its own wire index. */
  passClick() {
    const d = this.pending;
    const pass = this.passOption;
    if (d === null || pass === null || this.busy) return;
    this.handAnswer();
    void this.post([pass.index]);
  }

  /** primaryClick posts the primary-by-kind option directly. */
  primaryClick() {
    const d = this.pending;
    const p = d ? primaryOf(d) : null;
    if (d === null || p === null || d.seq === this.postedSeq || this.busy) return;
    this.handAnswer();
    void this.post([p.index]);
  }

  /** confirmConcede posts the armed concede option — the second, explicit confirmation. */
  confirmConcede() {
    const d = this.pending;
    if (d === null || !this.confirming || this.busy) return;
    const concede = d.options.find(isConcede);
    if (!concede) return;
    this.cancelFastForward();
    this.suspendAuto('human');
    void this.post([concede.index]);
  }

  /** submit posts the picked set — gated on min/max; a rejected answer is recovered from, never treated as impossible. */
  submit() {
    const d = this.pending;
    if (d === null || d.seq === this.postedSeq || this.busy) return;
    if (this.picked.length < d.min || this.picked.length > d.max) return;
    this.handAnswer();
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
