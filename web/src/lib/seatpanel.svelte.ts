import type { Decision, Intent, Option } from '../protocol';
import { fetchPending, postIntent, ApiError } from './api';
import type { SeatCtx } from './seat';

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

export class SeatPanelState {
  readonly table: string;
  readonly ctx: SeatCtx;

  constructor(table: string, readonly match: number, ctx: SeatCtx) {
    this.table = table;
    this.ctx = ctx;
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

  /** begin resets the seat across a match boundary. (The component keys the panel by match, so a new match is a fresh instance — this is belt and braces.) */
  begin() {
    this.pending = null;
    this.picked = [];
    this.postedSeq = null;
    this.confirming = false;
    this.error = null;
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
    void this.post([p.index]);
  }

  /** confirmConcede posts the armed concede option — the second, explicit confirmation. */
  confirmConcede() {
    const d = this.pending;
    if (d === null || !this.confirming || this.busy) return;
    const idx = d.options.findIndex(isConcede);
    if (idx < 0) return;
    void this.post([idx]);
  }

  /** submit posts the picked set — gated on min/max; a rejected answer is recovered from, never treated as impossible. */
  submit() {
    const d = this.pending;
    if (d === null || d.seq === this.postedSeq || this.busy) return;
    if (this.picked.length < d.min || this.picked.length > d.max) return;
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
