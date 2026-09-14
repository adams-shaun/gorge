import type { Decision, Intent, Option, View } from '../protocol';
import { fetchPending, postIntent, ApiError } from './api';
import type { SeatCtx } from './seat';
import { STOPPABLE_STEPS, decide, emptyPriorityWindow, type StopReason, type Stops, type TurnSide } from './autopilot';
import {
  applyPreset,
  defaultSettings,
  loadSettings,
  presetPatch,
  saveSettings,
  withChange,
  type PlaySettings,
  type PresetName,
  type StepStop,
  type StoppableStep,
} from './playsettings';

/**
 * actedOption reports whether the posted `choices` (wire indices) contain at
 * least one real action on a priority decision — an option whose kind is
 * neither pass nor concede (cast, ability, play_land, ...). The kind comes
 * off the wire option itself, resolved by index; a choice that names no
 * option on the decision is not an action. This is the arming test for
 * pass-after-acting, and it deliberately mirrors `actionable`'s kind test
 * in autopilot.ts (per-choice rather than per-decision).
 * (Moved here from actpass.ts, which prio3 deleted with the per-table keys.)
 */
export function actedOption(d: Decision, choices: number[]): boolean {
  if (d.kind !== 'priority') return false;
  return choices.some((i) => {
    const o = d.options.find((opt) => opt.index === i);
    return o !== undefined && o.kind !== 'pass' && o.kind !== 'concede';
  });
}

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
export type AutoOffReason = 'loop' | 'cap';

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
  | { kind: 'end-turn-armed' }
  | { kind: 'end-turn-passing'; count: number }
  | { kind: 'end-turn-stopped'; reason: StopReason | AutoOffReason }
  | { kind: 'skip-turn-armed' }
  | { kind: 'skip-turn-passing'; count: number }
  | { kind: 'skip-turn-stopped'; reason: StopReason | AutoOffReason }
  | { kind: 'act-passed'; count: number };

const WAITING_TEXT: Record<StopReason, string> = {
  'disabled': 'Auto is off.',
  'not-priority': 'Auto is waiting: this decision needs you, not a pass.',
  'unexpected-shape': 'Auto is waiting: it does not recognise this window.',
  'stop-set': 'Auto stopped here: you set a stop on this step.',
  'opponent-object': "Auto stopped here: an opponent's object is on the stack and you can respond.",
  'own-object': 'Auto stopped here: your own object is on the stack and you can respond.',
};

const OFF_TEXT: Record<AutoOffReason, string> = {
  'loop': 'Auto switched itself off: the same decision came back after it answered. Press the Auto switch to rearm it.',
  'cap': `Auto switched itself off after ${AUTO_PASS_CAP} passes in a row. Press the Auto switch to rearm it.`,
};

/**
 * RUN_*_TEXT re-words a stop reason for the one-shot runs, which are not
 * "waiting" — a stop ENDS an End Turn / hard-skip run. The wording is
 * reason-loyal (the same fact the Auto wording states), only re-anchored.
 */
const RUN_WAITING_TEXT: Record<StopReason, string> = {
  'disabled': 'the Auto switch is off.',
  'not-priority': 'this decision needs you, not a pass.',
  'unexpected-shape': 'it does not recognise this window.',
  'stop-set': 'you set a stop on this step.',
  'opponent-object': "an opponent's object is on the stack and you can respond.",
  'own-object': 'your own object is on the stack and you can respond.',
};
const RUN_OFF_TEXT: Record<AutoOffReason, string> = {
  'loop': 'the same decision came back after it answered.',
  'cap': `it passed ${AUTO_PASS_CAP} windows in a row.`,
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
    case 'end-turn-armed':
      return 'End Turn: passing the rest of this turn — it still stops for opponent plays. Esc cancels.';
    case 'end-turn-passing':
      return note.count === 1
        ? 'End Turn passed 1 priority window.'
        : `End Turn passed ${note.count} priority windows.`;
    case 'end-turn-stopped':
      return `End Turn stopped: ${
        note.reason in RUN_WAITING_TEXT
          ? RUN_WAITING_TEXT[note.reason as StopReason]
          : RUN_OFF_TEXT[note.reason as AutoOffReason]
      }`;
    case 'skip-turn-armed':
      return 'Skipping turn — Esc to stop.';
    case 'skip-turn-passing':
      return note.count === 1
        ? 'Skipping turn: passed 1 priority window.'
        : `Skipping turn: passed ${note.count} priority windows.`;
    case 'skip-turn-stopped':
      return `Skipping turn stopped: ${
        note.reason in RUN_WAITING_TEXT
          ? RUN_WAITING_TEXT[note.reason as StopReason]
          : RUN_OFF_TEXT[note.reason as AutoOffReason]
      }`;
    case 'act-passed':
      return note.count === 1
        ? 'Passed 1 priority window after your action.'
        : `Passed ${note.count} priority windows after your action.`;
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
    // The settings load here rather than at mount so the very first render
    // — and every test — sees the player's saved preferences. SSR and a
    // browser that refuses site data both pass null and get casual.
    this.settings = loadSettings(storage);
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

  /**
   * settings is the player's play settings (playsettings.ts): the presets,
   * the per-step stop rules, the opponent-object rules and pass-after-acting
   * in ONE object — the source of truth for every machine decision this
   * state makes. `auto` is settings.autoPass, the stops are settings.steps,
   * actPass is settings.passAfterAct. It is loaded from the ONE global
   * localStorage key at construction (SSR has no storage and gets the
   * defaults, casual) and saved on every change, so a preference survives a
   * reload and a match boundary: begin() must not — and does not — reset it.
   */
  settings = $state<PlaySettings>(defaultSettings());

  /**
   * machinePaused is the runaway brake, and it is deliberately NOT a
   * settings change: the loop guard and the pass cap set it, and only the
   * player's own Auto switch clears it. Flipping the persisted autoPass from
   * a guard would have written a preference the player never chose — a
   * reload would then come back with auto off for good.
   */
  machinePaused = $state(false);

  /**
   * skipEmpty is the empty-window floor, and it is ON by default -- the one
   * thing the panel does for the player without being asked. A priority
   * window whose only options are pass and concede asks nothing: there is no
   * action to take, so collecting a click there is pure friction between the
   * player and the next real decision. It is separate from `auto` because it
   * is a different promise: auto decides FOR you (a persisted preference),
   * whereas this only declines to interrupt you when there was nothing to
   * decide. Turning it off restores the old stop-at-every-window behaviour.
   */
  skipEmpty = $state(true);

  /** autoPassed counts every window auto has answered this session, so the pass is visible after the fact. */
  autoPassed = $state(0);

  /** emptySkipped counts the no-action windows the floor passed, kept apart from autoPassed so the panel never credits auto with a pass it did not make. */
  emptySkipped = $state(0);
  /** actPassed counts the windows the pass-after-acting preference answered, kept apart from autoPassed and emptySkipped for the same reason: each mechanism's passes are its own. */
  actPassed = $state(0);
  /** autoRun is the current unbroken run of machine passes; Auto and the one-shot runs share its hard cap. */
  autoRun = $state(0);
  /**
   * oneShot is the current one-shot run: 'end-turn' (End Turn — passes the
   * rest of THIS turn with the player's own rules minus the step stops),
   * 'hard-skip' (passes everything including opponent objects, MTGO F6), or
   * 'none'. Both end when the turn number changes or the step reaches
   * cleanup, on Escape, on any non-priority decision, on any other stop
   * verdict, and at the shared pass cap.
   */
  oneShot = $state<'none' | 'end-turn' | 'hard-skip'>('none');
  /** runPassed is the current one-shot's visible pass count. */
  runPassed = $state(0);
  /** the view turn the one-shot was armed at; a turn change ends the run. */
  private oneShotTurn: number | null = null;
  /** note is what the panel says about automatic action, as a value — autoNoteText turns it into words. */
  note = $state<AutoNote>({ kind: 'off' });
  /**
   * autoActedSeq is the seq auto last posted for. If a decision with that
   * seq is put in front of auto again, the answer did not take and auto
   * would post it forever: that is the loop guard, and it pauses the
   * machine (machinePaused), never the persisted preference.
   */
  private autoActedSeq: number | null = null;
  /**
   * actPassArmed is the one-shot token: set when this seat POSTS a hand
   * answer to a priority decision containing a real action, consumed by the
   * FIRST priority window considerAuto sees while neither auto nor a
   * one-shot run is live. It is deliberately not reactive state — it is only
   * ever read inside considerAuto, which the component runs once per
   * decision/view change, so a token minted by a hand answer is always seen
   * by the next decision's pass through the loop. It survives intermediate
   * NON-priority hand answers on purpose: the common cast flow is cast → the
   * spell's target decision (hand-answered) → the next priority window, and
   * disarming on the target answer would break exactly the flow the
   * preference exists to smooth.
   */
  private actPassArmed = false;
  /**
   * actPassActedSeq is the seq the armed token last answered — the same loop
   * guard autoActedSeq is for the other machine paths. In practice the token
   * is consumed before the post is even attempted, so a failed post leaves
   * nothing armed and no retry can start; the guard is the belt to that
   * braces: if the very same seq ever comes back armed again, it is not
   * answered a second time.
   */
  private actPassActedSeq: number | null = null;
  /** presetBackup holds the settings Ctrl+Shift+F replaced, so toggling back restores them exactly. Session-scoped: the backup is a convenience, not a preference. */
  private presetBackup: PlaySettings | null = null;

  /** auto is settings.autoPass: the persisted preference, ON by default (casual). Reading it is a read of settings. */
  get auto(): boolean {
    return this.settings.autoPass;
  }

  /** actPass is settings.passAfterAct, likewise persisted. */
  get actPass(): boolean {
    return this.settings.passAfterAct;
  }

  /** endTurn mirrors oneShot for the template and tests. */
  get endTurn(): boolean {
    return this.oneShot === 'end-turn';
  }

  /** hardSkip mirrors oneShot for the template, the chip and tests. */
  get hardSkip(): boolean {
    return this.oneShot === 'hard-skip';
  }

  /**
   * stops is the Set-shaped view of settings.steps that the phase track and
   * the stop grid read: a step is "stopped" when its rule is not 'off'. The
   * setter writes back — present becomes 'smart', absent 'off' — so the old
   * Set-based callers keep working; a 'forced' rule reads as set and is
   * rewritten to 'smart' by the same assignment (the Set shape cannot
   * express 'forced').
   */
  get stops(): Stops {
    const rules = this.settings.steps;
    const side = (r: Record<StoppableStep, StepStop>): Set<string> =>
      // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a fresh ephemeral read view per access, never stored; the reactive source is settings.steps
      new Set(STOPPABLE_STEPS.filter((s) => r[s as StoppableStep] !== 'off'));
    return { yours: side(rules.yours), opponents: side(rules.opponents) };
  }

  set stops(next: Stops) {
    const side = (set: Set<string>): Record<StoppableStep, StepStop> => {
      const out = {} as Record<StoppableStep, StepStop>;
      for (const s of STOPPABLE_STEPS) out[s as StoppableStep] = set.has(s) ? 'smart' : 'off';
      return out;
    };
    this.applySettings(withChange(this.settings, {
      steps: { yours: side(next.yours), opponents: side(next.opponents) },
    }));
  }

  /** playMode is the status chip's value: the live one-shot beats the preset label. */
  get playMode(): 'end-turn' | 'skip-turn' | PlaySettings['preset'] {
    if (this.oneShot === 'end-turn') return 'end-turn';
    if (this.oneShot === 'hard-skip') return 'skip-turn';
    return this.settings.preset;
  }

  /**
   * applySettings swaps the whole settings object and persists it. Every
   * settings change funnels through here (or patchSettings below) so the
   * global key never goes stale.
   */
  private applySettings(next: PlaySettings) {
    this.settings = next;
    saveSettings(this.storage, next);
  }

  /** patchSettings applies a partial change through withChange (which relabels the preset when the result matches one) and persists. */
  private patchSettings(patch: Partial<PlaySettings>) {
    this.applySettings(withChange(this.settings, patch));
  }

  /**
   * editSettings is the GAME OPTIONS editor's write path (prio4): a partial
   * change applied through withChange — so the preset relabels itself
   * Custom while the configuration matches none, and back to a named preset
   * when an edit is undone — and persisted. A settings edit ends a live
   * one-shot run (editing the rules is the player taking the controls) but
   * touches nothing else: the runaway brake still clears only on the Auto
   * switch or on applying a named preset that runs auto (applyNamedPreset
   * below), which is what its note tells the player to press.
   */
  editSettings(patch: Partial<PlaySettings>) {
    this.cancelRun(false);
    this.patchSettings(patch);
  }

  /**
   * applyNamedPreset is the GAME OPTIONS preset picker's and "Reset to
   * Casual" button's write path: the named preset applied through
   * withChange (presetPatch covers every field, so withChange relabels the
   * preset itself) PLUS the machine-side re-arm effects setAuto carries.
   * The runaway brake is a pause of the machine, not a setting — and
   * picking Casual or No tells after the brake tripped is the player asking
   * the machine to run again, so leaving machinePaused set would leave the
   * panel reading auto-pass on while considerAuto keeps refusing to act.
   * The cleared run counters and the armed/off note mirror setAuto exactly;
   * on full-control (autoPass false) the brake clear is harmless and the
   * note is off, exactly as setAuto(false) would leave it.
   */
  applyNamedPreset(id: PresetName) {
    this.cancelRun(false);
    this.machinePaused = false;
    this.autoRun = 0;
    this.autoActedSeq = null;
    this.patchSettings(presetPatch(id));
    this.note = this.auto ? { kind: 'armed' } : { kind: 'off' };
  }

  /** setAuto is the Auto/Manual control: a settings change (autoPass), persisted. Turning it on — or re-arming it while it is on — clears the runaway brake and the previous run so an old count never trips the cap. */
  setAuto(on: boolean) {
    this.cancelRun(false);
    this.machinePaused = false;
    this.autoRun = 0;
    this.autoActedSeq = null;
    this.patchSettings({ autoPass: on });
    this.note = on ? { kind: 'armed' } : { kind: 'off' };
  }

  /** setSkipEmpty is the empty-window floor's control. Turning it back on clears the run so an old count never trips the cap. */
  setSkipEmpty(on: boolean) {
    this.cancelRun();
    this.skipEmpty = on;
    this.autoRun = 0;
    this.autoActedSeq = null;
    if (!this.auto) this.note = { kind: 'off' };
  }

  /** setActPass is the pass-after-acting preference's control — a settings change (passAfterAct), persisted globally. Turning it OFF disarms a still-armed pass: a stale one-shot firing several windows after the player switched the preference off would be exactly the surprise the switch exists to prevent. Turning it ON arms nothing — only a future hand action does. */
  setActPass(on: boolean) {
    if (!on) this.actPassArmed = false;
    this.patchSettings({ passAfterAct: on });
  }

  /**
   * toggleStop flips one step's stop rule on one turn side and persists it.
   * The rule cycles 'off' → 'smart' → 'off' (a 'forced' rule goes straight
   * back to 'off' — the click always means "stop here" or "stop ignoring
   * here"); a step that grants no priority (untap, cleanup) is refused. The
   * two sides are separate records on purpose: stopping in your own combat
   * and stopping in an opponent's are different intentions, and a stop set
   * on one side must never mark the other.
   */
  toggleStop(step: string, side: TurnSide) {
    this.cancelRun();
    if (!(STOPPABLE_STEPS as readonly string[]).includes(step)) return;
    const rule = this.settings.steps[side][step as StoppableStep] ?? 'off';
    // The steps patch is a deep partial at runtime (withChange merges field-wise); the cast states that.
    const patch = { steps: { [side]: { [step as StoppableStep]: rule === 'off' ? 'smart' : 'off' } } } as unknown as Partial<PlaySettings>;
    this.patchSettings(patch);
  }

  /**
   * toggleFullControl is the Ctrl+Shift+F hotkey's action: swap the current
   * settings for the full-control preset, or — when full-control is already
   * the live preset — restore exactly what it replaced. The backup is
   * session-scoped and consumed by the restore; a second full-control press
   * without a backup restores the defaults rather than guessing.
   */
  toggleFullControl() {
    this.cancelRun(false);
    this.autoRun = 0;
    this.autoActedSeq = null;
    if (this.settings.preset === 'full-control') {
      const back = this.presetBackup;
      this.presetBackup = null;
      this.applySettings(back ?? defaultSettings());
    } else {
      this.presetBackup = this.settings;
      this.applySettings(applyPreset('full-control'));
    }
    this.note = this.auto ? { kind: 'armed' } : { kind: 'off' };
  }

  /**
   * stopActing is the shared guard exit for the loop guard and the pass cap.
   * Whichever mechanism was acting is the one stopped: the one-shot run if
   * one is live, otherwise the machine (paused, NOT un-preferenced),
   * otherwise the empty-window floor. All are runaway protections and all
   * have to be able to actually stop the thing that is running -- before
   * the floor existed, suspendAuto returned silently when auto was already
   * off, which would have left a wedged floor posting forever.
   */
  private stopActing(reason: AutoOffReason) {
    if (this.oneShot !== 'none') {
      const mode = this.oneShot;
      this.oneShot = 'none';
      this.autoRun = 0;
      this.autoActedSeq = null;
      this.note = { kind: mode === 'end-turn' ? 'end-turn-stopped' : 'skip-turn-stopped', reason };
      return;
    }
    if (this.auto && !this.machinePaused) {
      this.suspendAuto(reason);
      return;
    }
    this.skipEmpty = false;
    this.autoRun = 0;
    this.autoActedSeq = null;
    this.note = { kind: 'skip-off', reason };
  }

  /**
   * startEndTurn arms the END TURN one-shot: every priority window for the
   * rest of THIS turn is passed, under decide()'s ordinary rules EXCEPT the
   * step stops, which the run ignores — pressing END TURN at your own main2
   * with a playable card is exactly the point of the button. The opponent-
   * object rules still apply: the run stops for an opponent's spell or
   * ability as the player's settings say. The run ends when the turn number
   * changes or the step reaches cleanup, on Escape, on any non-priority
   * decision, on any other stop verdict, and at the shared pass cap. The
   * current view is required because the press is itself consent.
   */
  startEndTurn(view: View) {
    this.startRun('end-turn', view);
  }

  /**
   * startHardSkip arms the hard skip (MTGO F6, Shift+Enter or shift-click
   * END TURN): EVERYTHING is passed for the rest of the turn, including
   * opponent objects — the strip shows a warning chip while it runs. The
   * same expiry and cap as End Turn apply.
   */
  startHardSkip(view: View) {
    this.startRun('hard-skip', view);
  }

  private startRun(kind: 'end-turn' | 'hard-skip', view: View) {
    if (this.busy) return;
    this.oneShot = kind;
    this.runPassed = 0;
    this.autoRun = 0;
    this.autoActedSeq = null;
    this.oneShotTurn = view.turn;
    this.note = kind === 'end-turn' ? { kind: 'end-turn-armed' } : { kind: 'skip-turn-armed' };
  }

  /** Any other pointer/key/answer hands control back immediately. When it actually fires (a run was live) it also clears an armed pass-after-acting token: the player took the controls back mid-run. */
  cancelRun(say = true) {
    if (this.oneShot === 'none') return;
    this.actPassArmed = false;
    this.oneShot = 'none';
    this.autoRun = 0;
    this.autoActedSeq = null;
    if (say) this.note = this.auto ? { kind: 'armed' } : { kind: 'off' };
  }

  /**
   * handAnswer is the takeover step every human answering path runs before
   * it posts. A hand answer is NOT a request to stop auto-passing (prio3):
   * the persisted autoPass survives it, and Escape is the only key that
   * ends a run without a decision of its own. The one thing it does is end
   * a live one-shot run — taking the controls mid-run cancels it.
   */
  private handAnswer() {
    this.cancelRun();
  }

  /**
   * suspendAuto pauses the machine with a stated reason. Only the runaway
   * guards (loop, cap) reach it now — a hand answer and Escape no longer
   * flip the persisted preference. The pause clears on the player's next
   * Auto switch.
   */
  suspendAuto(reason: AutoOffReason) {
    if (!this.auto || this.machinePaused) return;
    this.machinePaused = true;
    this.autoRun = 0;
    this.autoActedSeq = null;
    this.note = { kind: 'stopped', reason };
  }

  /** onKeydown is the panel's key handler: Escape cancels the one-shot run — it does NOT flip the persisted autoPass (a panic key is not a settings change). */
  onKeydown(key: string) {
    if (key !== 'Escape') return;
    this.actPassArmed = false;
    this.cancelRun();
  }

  /**
   * expireRun ends the one-shot when its turn is over — the turn number
   * changed since the press, or the step reached cleanup. It is called from
   * considerAuto (so a decision arriving in a new turn never sees the run
   * armed) and from the component's per-view effect (so the chip drops even
   * while no decision is pending for this seat).
   */
  expireRun(view: View) {
    if (this.oneShot === 'none') return;
    if (view.turn !== this.oneShotTurn || view.step === 'cleanup') this.cancelRun(false);
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
    // A one-shot run ends the moment its turn is over, whether or not a
    // decision is pending (see expireRun).
    this.expireRun(view);

    const autoOn = this.auto && !this.machinePaused;

    // Pass after acting: the one-shot token, spent at the FIRST priority
    // window that arrives while neither auto nor a one-shot run is live
    // (with either of those live, their own rules govern and the token stays
    // dormant). It runs BEFORE the early return below, because a window with
    // actions on it is exactly the one this preference exists to pass, and
    // that window is precisely the one the empty-window floor never touches.
    if (!autoOn && this.oneShot === 'none' && this.actPassArmed && d.kind === 'priority') {
      this.consumeActPass(view);
      return;
    }

    // The empty-window floor runs whether or not auto is on, so a Manual
    // seat is still not stopped at a window that asks nothing. When auto IS
    // on this is redundant -- decide()'s own !actionable branch reaches the
    // same pass -- and that is the point: one shape test, two callers.
    const emptyIndex = this.skipEmpty ? emptyPriorityWindow(d) : null;
    if (!autoOn && this.oneShot === 'none' && emptyIndex === null) return;

    // Loop guard: we already answered this seq and here it is again. The
    // answer did not take, so posting it a second time is the start of an
    // unbounded retry against the server.
    if (this.autoActedSeq !== null && d.seq === this.autoActedSeq) {
      this.stopActing('loop');
      return;
    }

    let index: number;
    if (this.oneShot !== 'none' || autoOn) {
      // A one-shot run feeds decide() its OWN settings: autoPass forced on
      // (the press is the consent), the step rules off (End Turn ignores
      // step stops), and — hard skip only — every opponent-object rule
      // 'never'. The player's real settings govern persistent Auto.
      const verdict = decide({
        decision: d,
        view,
        seat: this.ctx.seat,
        settings: this.oneShot !== 'none' ? this.runSettings(this.oneShot) : this.settings,
      });
      if (verdict.act === 'stop') {
        // decide() remains the safety oracle. A one-shot run ENDS on every
        // stop verdict — there is no acknowledgement machinery any more,
        // because a run honours no step stops and the press itself moved
        // the window it was pressed on. Persistent Auto stays armed: the
        // player answers this window and Auto resumes after it — and with
        // hand answers no longer disarming Auto (prio3), no exception
        // token is needed or minted.
        this.autoRun = 0;
        if (this.oneShot !== 'none') {
          const mode = this.oneShot;
          this.oneShot = 'none';
          this.autoActedSeq = null;
          this.note = { kind: mode === 'end-turn' ? 'end-turn-stopped' : 'skip-turn-stopped', reason: verdict.reason };
        } else {
          this.note = { kind: 'waiting', reason: verdict.reason };
        }
        return;
      }
      index = verdict.index;
    } else {
      index = emptyIndex as number;
    }

    // The cap bounds an unbroken run of machine-made passes, and it bounds
    // the floor for the same reason it bounds auto and the one-shot runs: a
    // stuck window answered forever is a denial of service the player never
    // asked for.
    if (this.autoRun >= AUTO_PASS_CAP) {
      this.stopActing('cap');
      return;
    }

    this.autoActedSeq = d.seq;
    this.autoRun += 1;
    if (this.oneShot !== 'none') {
      this.runPassed += 1;
      this.note = this.oneShot === 'end-turn'
        ? { kind: 'end-turn-passing', count: this.runPassed }
        : { kind: 'skip-turn-passing', count: this.runPassed };
    } else if (autoOn) {
      this.autoPassed += 1;
      this.note = { kind: 'passing', count: this.autoPassed };
    } else {
      this.emptySkipped += 1;
      this.note = { kind: 'skipped', count: this.emptySkipped };
    }
    void this.post([index]);
  }

  /**
   * runSettings is the settings a one-shot run feeds decide(): the player's
   * own settings with autoPass forced on (the press is the consent), every
   * step rule off (End Turn ignores step stops), and — hard skip only —
   * every opponent-object rule 'never' and ownObjects 'never' (it passes
   * everything, MTGO F6). Everything else — the opponent-object rules for a
   * plain End Turn — stands as the player set it.
   */
  private runSettings(kind: 'end-turn' | 'hard-skip'): PlaySettings {
    const off = {} as Record<StoppableStep, StepStop>;
    for (const s of STOPPABLE_STEPS) off[s as StoppableStep] = 'off';
    const s: PlaySettings = {
      ...this.settings,
      preset: 'custom',
      autoPass: true,
      steps: { yours: { ...off }, opponents: { ...off } },
    };
    if (kind === 'hard-skip') {
      s.opponentSpell = 'never';
      s.opponentAbility = 'never';
      s.opponentTrigger = 'never';
      s.ownObjects = 'never';
    }
    return s;
  }

  /**
   * consumeActPass spends the armed pass-after-acting token on ONE priority
   * window and is the only place the token ever posts. The invariants, in
   * order:
   *
   *  - the token is consumed FIRST, whatever happens after: it is one-shot,
   *    and a stale armed pass firing several windows later is forbidden;
   *  - the loop guard refuses a seq the token already answered, so a failed
   *    post can never retry (in practice the consumption above already
   *    guarantees this; the guard is the belt to that braces);
   *  - decide() stays the safety oracle, consulted with enabled: true and
   *    the player's own stops. Its pass verdict's index ALWAYS points at the
   *    pass option the shape check found — this method is structurally
   *    incapable of posting anything but that pass, and never a concede;
   *  - on a stop verdict (a stop the player set on this step/side, or an
   *    opponent-controlled stack object) the window is surfaced exactly as
   *    it would have been without the preference: no post, no note change.
   *    The token is still spent — that window was the one shot.
   *
   * The pass is counted and worded under the preference's own name
   * (actPassed / act-passed), never auto's.
   */
  private consumeActPass(view: View) {
    const d = this.pending;
    if (d === null) return;
    this.actPassArmed = false;
    if (this.actPassActedSeq !== null && d.seq === this.actPassActedSeq) return;
    this.actPassActedSeq = d.seq;
    // The armed token is itself the consent: the preference fires whether or
    // not the master switch is on (a manual seat with the preference on —
    // the mode this preference exists for).
    const verdict = decide({ decision: d, view, seat: this.ctx.seat, settings: { ...this.settings, autoPass: true } });
    if (verdict.act !== 'pass') return;
    this.actPassed += 1;
    this.note = { kind: 'act-passed', count: this.actPassed };
    void this.post([verdict.index]);
  }

  /** begin resets the seat across a match boundary. (The component keys the panel by match, so a new match is a fresh instance — this is belt and braces.) The settings are a property of the PLAYER, not of the match: auto (autoPass), the stops and pass-after-acting all survive begin() untouched. */
  begin() {
    this.pending = null;
    this.picked = [];
    this.postedSeq = null;
    this.confirming = false;
    this.error = null;
    this.oneShot = 'none';
    this.oneShotTurn = null;
    this.machinePaused = false;
    this.autoRun = 0;
    this.runPassed = 0;
    this.autoPassed = 0;
    // An armed pass-after-acting token is a one-shot about a window that no
    // longer exists once the match does; the PREFERENCE persists across the
    // boundary in the settings, only the pending token clears.
    this.actPassArmed = false;
    this.actPassActedSeq = null;
    this.actPassed = 0;
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

  /** click handles one option click. A single-required-option decision posts immediately (the click IS the answer); a multi-pick toggles into `picked` for submit. The concede option never posts on the first click (R-E4-1). Passing `holdPriority` (Ctrl held) skips the pass-after-acting arming for this one action. */
  click(index: number, opts?: { holdPriority?: boolean }) {
    const d = this.pending;
    if (d === null || d.seq === this.postedSeq || this.busy) return;
    const opt = optionAt(d, index);
    if (opt === undefined) return;
    if (isConcede(opt)) {
      // Concede is not an action: it never arms pass-after-acting, and an
      // ALREADY armed token must die here — the seat's next priority window
      // would otherwise be passed for a player who is no longer in the game.
      // It also ends a live one-shot run. It does NOT flip the persisted
      // autoPass: conceding is a move, not a settings change.
      this.actPassArmed = false;
      this.cancelRun();
      if (this.confirming) void this.post([index]);
      else this.confirming = true;
      return;
    }
    this.handAnswer();
    this.confirming = false;
    if (d.min === 1 && d.max === 1) {
      void this.post([index], opts?.holdPriority ?? false);
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

  /** primaryClick posts the primary-by-kind option directly. `holdPriority` (Ctrl held) skips the pass-after-acting arming for this one action. */
  primaryClick(holdPriority = false) {
    const d = this.pending;
    const p = d ? primaryOf(d) : null;
    if (d === null || p === null || d.seq === this.postedSeq || this.busy) return;
    this.handAnswer();
    void this.post([p.index], holdPriority);
  }

  /** confirmConcede posts the armed concede option — the second, explicit confirmation. It does not flip the persisted autoPass; it does kill an armed pass-after-acting token and a live run. */
  confirmConcede() {
    const d = this.pending;
    if (d === null || !this.confirming || this.busy) return;
    const concede = d.options.find(isConcede);
    if (!concede) return;
    this.actPassArmed = false;
    this.cancelRun();
    void this.post([concede.index]);
  }

  /** submit posts the picked set — gated on min/max; a rejected answer is recovered from, never treated as impossible. `holdPriority` (Ctrl held) skips the pass-after-acting arming for this one action. */
  submit(holdPriority = false) {
    const d = this.pending;
    if (d === null || d.seq === this.postedSeq || this.busy) return;
    if (this.picked.length < d.min || this.picked.length > d.max) return;
    this.handAnswer();
    void this.post([...this.picked], holdPriority);
  }

  private async post(choices: number[], holdPriority = false) {
    const d = this.pending;
    if (d === null || this.busy) return;
    this.busy = true;
    this.error = null;
    try {
      await postIntent(this.table, this.match, { seq: d.seq, player: d.player, choices } satisfies Intent, this.ctx);
      this.postedSeq = d.seq;
      // The hand answers that can carry a real action are click()'s post-on-click
      // (min == max == 1) and submit()'s multi-pick commit; both funnel through
      // here, so the pass-after-arming test lives on the ACCEPTED post — a
      // rejected intent never arms, and the gates are the preference itself
      // and the hold-priority modifier: with actPass off nothing is ever
      // armed, and a Ctrl-held action (hold priority) skips the arming for
      // that one post. passClick/primaryClick post only pass/resolve options,
      // the machine paths (auto, the one-shot runs, the empty-window floor,
      // the act-pass pass itself) post only the pass verdict's index, and a
      // non-priority decision fails the kind test, so none of them arm.
      // Concede never reaches here as an action: click() returns before
      // posting it once and confirmConcede posts a concede kind, which the
      // test rejects.
      if (this.actPass && !holdPriority && actedOption(d, choices)) this.actPassArmed = true;
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
