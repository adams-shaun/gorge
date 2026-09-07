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
 * Only these three kinds are hidden — everything else (land plays, casts,
 * `draw`, `move_zone`, triggers, …) is kept, because narrowing further is a
 * judgment nobody has made: `draw` and `move_zone` are noisy in the sample
 * but they are real game events.
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
]);

/** isHiddenKind reports whether a single event kind is log-noise by default. */
export function isHiddenKind(kind: string): boolean {
  return hiddenKinds.has(kind);
}

export interface LogLine {
  line: string;
  event: { kind: string };
}

/**
 * visibleLog filters a list of described event lines for the transcript:
 * blank lines are always dropped, and the three hidden kinds are dropped
 * unless `revealAll` is true. `revealAll` is the transcript's "show
 * everything" toggle; it never affects DVR state, only what is rendered.
 * The event type is preserved, so callers keep the full event (seq, …).
 */
export function visibleLog<T extends { kind: string }>(
  lines: { line: string; event: T }[],
  revealAll: boolean,
): { line: string; event: T }[] {
  return lines.filter((l) => Boolean(l.line) && (revealAll || !isHiddenKind(l.event.kind)));
}
