/**
 * Client-side rules-log filtering (Task U5).
 *
 * The omniscient log drowns real game actions under engine bookkeeping:
 * against 341 live events, 109 were `priority`, 123 decision asks/answers,
 * and only 4 were actual plays. Which kinds read as noise is a product
 * decision, not a wire/protocol one, so it is said once here and the
 * transcript just filters through it. Revisiting the default later is an
 * edit to `hiddenKinds`, not a hunt through markup.
 *
 * Only these four kinds are hidden — everything else (land plays, casts,
 * `draw`, `move_zone`, triggers, …) is kept, because narrowing further is a
 * judgment nobody has made: `draw` and `move_zone` are noisy in the sample
 * but they are real game events. The fourth, `end_combat_reset`, is the
 * engine's whole-combat reset bookkeeping ("Combat ends", view/describe.go)
 * — not a play, not the `step` line for the end-combat step (that is its own
 * kind, filtered behind the independent step toggle below), and measured on a
 * live match at 23 firings in 2176 events, which on a quiet board is most of
 * what the transcript shows. It is hidden by default and still reachable
 * behind the existing "Show engine noise" toggle.
 *
 * NOTE (out of scope, finding only): the lobby Feed rail shows the same
 * drowning, but its cause is Go-side — it is fed by `widget.last`,
 * host/fanout.go's "last described line in the burst", which lands on
 * priority lines almost every burst. That belongs in a Go-side fix, not here.
 */
export const hiddenKinds: ReadonlySet<string> = new Set([
  'priority',
  'decision_ask',
  'decision_made',
  'end_combat_reset',
]);

/** isHiddenKind reports whether a single event kind is log-noise by default. */
export function isHiddenKind(kind: string): boolean {
  return hiddenKinds.has(kind);
}

/**
 * The step-line kind (ui9, "suppress step lines"). view/describe.go renders
 * a StepChange event as "Step: <phase>"; a step line names a phase boundary,
 * not a game action, and every phase yields one on its way through a corridor
 * ("Step: main-1", "Step: main-2"...). So it is filtered out by default, behind
 * its own toggle (independent of the engine-noise toggle), so a reader who
 * wants the phase corridor can put it back without also exposing priority asks.
 */
export const STEP_KIND = 'step';

/** isStepKind reports whether a single event kind is a phase/step line. */
export function isStepKind(kind: string): boolean {
  return kind === STEP_KIND;
}

export interface LogLine {
  line: string;
  event: { kind: string };
}

/**
 * visibleLog filters a list of described event lines for the transcript:
 * blank lines are always dropped; the four engine-noise kinds are dropped
 * unless `revealAll` is true; and step lines are dropped unless `revealSteps`
 * is true. The two toggles are independent (show step lines without showing
 * the noise, or the reverse). `revealAll`, when set, shows everything
 * regardless of either filter. No toggle affects DVR state, only what is
 * rendered. The event type is preserved, so callers keep the full event
 * (seq, …).
 */
export function visibleLog<T extends { kind: string }>(
  lines: { line: string; event: T }[],
  revealAll: boolean,
  revealSteps = false,
): { line: string; event: T }[] {
  return lines.filter(
    (l) =>
      Boolean(l.line) &&
      (revealAll || (!isHiddenKind(l.event.kind) && (revealSteps || !isStepKind(l.event.kind)))),
  );
}
