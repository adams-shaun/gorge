import type { Decision, Option } from '../../protocol';

/**
 * Divided-as-you-choose in one prompt. The engine asks twice (CR 601.2c
 * targets, then CR 601.2d / 603.3d the division); a target ask whose spell
 * will divide a total publishes it as `divide_total`, and the client collects
 * both answers at once. This module is the pure half: the stepper rules, the
 * target answer, and matching the engine's follow-up division ask back to the
 * stored counts so it can be answered without being shown. The engine's two
 * decisions are unchanged; the combat damage division (division.ts) is a
 * different ask and is not touched here.
 */

/** isDivideTarget reports a target ask that carries a total to divide among its targets. */
export function isDivideTarget(d: Decision): boolean {
  return d.kind === 'target' && (d.divide_total ?? 0) > 1 && d.options.length > 0;
}

/** Counts maps an option index to the points assigned to that target. */
export type Counts = Readonly<Record<number, number>>;

/** assigned is the total of all points placed. */
export function assigned(counts: Counts): number {
  let n = 0;
  for (const k of Object.keys(counts)) n += counts[Number(k)] ?? 0;
  return n;
}

/** remaining is how many points are still to place. */
export function remaining(d: Decision, counts: Counts): number {
  return (d.divide_total ?? 0) - assigned(counts);
}

function included(counts: Counts): number[] {
  return Object.keys(counts).map(Number).filter((i) => (counts[i] ?? 0) > 0);
}

/** canStepUp: a point is left, and a target not yet included fits under the ask's max and its exclusive group. */
export function canStepUp(d: Decision, counts: Counts, index: number): boolean {
  if (remaining(d, counts) <= 0) return false;
  const have = counts[index] ?? 0;
  if (have > 0) return true;
  const inc = included(counts);
  if (inc.length >= d.max) return false;
  const g = d.options.find((o) => o.index === index)?.group;
  if (g) {
    const same = inc.filter((i) => d.options.find((o) => o.index === i)?.group === g).length;
    const cap = d.groupLimits?.[g] ?? (d.groupLimit && d.groupLimit > 1 ? d.groupLimit : 1);
    if (same >= cap) return false;
  }
  return true;
}

/** canStepDown: the target holds a point. Dropping to 0 un-targets it. */
export function canStepDown(counts: Counts, index: number): boolean {
  return (counts[index] ?? 0) > 0;
}

/** step moves one target's count by +1 or -1, or returns the counts unchanged when the move is illegal. */
export function step(d: Decision, counts: Counts, index: number, delta: 1 | -1): Counts {
  if (delta === 1 ? !canStepUp(d, counts, index) : !canStepDown(counts, index)) return counts;
  return { ...counts, [index]: (counts[index] ?? 0) + delta };
}

/** canSubmitDivide: every point is assigned and the number of targets fits the ask's own bounds. */
export function canSubmitDivide(d: Decision, counts: Counts): boolean {
  const n = included(counts).length;
  return remaining(d, counts) === 0 && n >= Math.max(d.min, 1) && n <= d.max;
}

/** targetChoices is the target answer: the targets holding a point, in the list's (option) order. */
export function targetChoices(d: Decision, counts: Counts): number[] {
  return d.options.filter((o) => (counts[o.index] ?? 0) > 0).map((o) => o.index);
}

/** One stored share: the target (by object or player, never by position) and its points. */
export interface Share { obj: number | null; player: number | null; count: number }

/** sharesOf freezes the counts into per-target shares, in the target answer's order. */
export function sharesOf(d: Decision, counts: Counts): Share[] {
  return d.options
    .filter((o) => (counts[o.index] ?? 0) > 0)
    .map((o) => ({ ...identity(o), count: counts[o.index] ?? 0 }));
}

function identity(o: Pick<Option, 'obj' | 'player'>): { obj: number | null; player: number | null } {
  return o.obj !== undefined && o.obj !== 0 ? { obj: o.obj, player: null } : { obj: null, player: o.player };
}

/** What the target answer left behind for the follow-up division ask to be matched against. */
export interface Expectation { source: number; shares: Share[] }

/** expectationOf builds the expectation stored with the posted target answer. */
export function expectationOf(d: Decision, counts: Counts): Expectation {
  return { source: d.source ?? 0, shares: sharesOf(d, counts) };
}

/**
 * divisionAnswer returns the picks that answer the engine's follow-up division
 * ask from the stored shares (each target's count minus its floor of 1, as that
 * many picks of its option), or null when the ask is not the one expected: not
 * a repeatable choose of `divide` options, a different source, a different set
 * of targets (matched by obj / player), or a pick total that is not Min == Max.
 */
export function divisionAnswer(d: Decision, want: Expectation): number[] | null {
  if (d.kind !== 'choose' || !d.repeatable) return null;
  if (d.options.length === 0 || d.options.some((o) => o.kind !== 'divide')) return null;
  // An activated ability's target ask carries no source (it may target its own
  // source); the division ask then names the card, so only a known source is compared.
  if (want.source !== 0 && d.source !== want.source) return null;
  if (d.options.length !== want.shares.length) return null;
  const picks: number[] = [];
  const used = new Set<number>();
  for (const o of d.options) {
    const id = identity(o);
    const at = want.shares.findIndex((s, i) => !used.has(i) && s.obj === id.obj && s.player === id.player);
    if (at < 0) return null;
    used.add(at);
    for (let k = 1; k < want.shares[at]!.count; k++) picks.push(o.index);
  }
  if (picks.length !== d.min || picks.length !== d.max) return null;
  return picks;
}
