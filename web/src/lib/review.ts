import type { ReviewDecision } from '../protocol';

/**
 * The match reviewer's decision overlay (design §3-§4): pure functions over
 * the review server's decision index (GET .../decisions, one entry per
 * intent). The DecisionPanel and the transport's marks read them; nothing
 * here touches the DVR — a jump is the caller's 'scrub' to an entry's
 * ask_seq.
 */

/** Who actually answered a decision (bots/explain.Source). */
export type ExplainSource =
  | 'policy'
  | 'heuristic'
  | 'forced'
  | 'prior_skip'
  | 'fallback_feed'
  | 'fallback_root'
  | 'refusal_retry'
  | 'refusal_fallback';

/** One searched candidate (bots/explain.Row). Values are the deciding seat's perspective. */
export interface ExplainRow {
  choices: number[];
  visits: number;
  prior: number;
  q: number;
  chosen?: boolean;
  bot?: boolean;
}

/** bots/explain.Search. */
export interface ExplainSearch {
  engine: string;
  sims?: number;
  root_value: number;
  cheap?: boolean;
  ms?: number;
  rows: ExplainRow[];
  dropped?: number;
}

/** bots/explain.Net. */
export interface ExplainNet {
  value: number;
  scores?: number[];
}

/** bots/explain.Note: the record's explain entry for one intent. */
export interface ExplainNote {
  seat: number;
  policy?: string;
  source: ExplainSource | string;
  reason?: string;
  rung?: number;
  bot?: number[];
  search?: ExplainSearch;
  net?: ExplainNet;
}

/** note is the entry's explain note, or null when none was recorded. */
export function note(e: ReviewDecision | null | undefined): ExplainNote | null {
  const n = e?.explain;
  return n !== null && typeof n === 'object' && !Array.isArray(n) ? (n as ExplainNote) : null;
}

// Kinds whose answer is an ORDER, not a set: their choices compare as
// sequences. Every other kind's answer is a set of option indices.
const ORDERED_KINDS = new Set(['trigger_order', 'arrange']);

function sameAnswer(kind: string, a: readonly number[], b: readonly number[]): boolean {
  if (a.length !== b.length) return false;
  if (ORDERED_KINDS.has(kind)) return a.every((x, i) => x === b[i]);
  const x = [...a].sort((p, q) => p - q);
  const y = [...b].sort((p, q) => p - q);
  return x.every((v, i) => v === y[i]);
}

/** isOverride: the note carries the wrapped heuristic's answer and the seat chose something else. */
export function isOverride(e: ReviewDecision): boolean {
  const n = note(e);
  return !!n && Array.isArray(n.bot) && !sameAnswer(e.kind, n.bot, e.chosen ?? []);
}

/** isFallback: a driver ladder answered — the feed or root fell back, or the engine refused the seat. */
export function isFallback(e: ReviewDecision): boolean {
  const s = note(e)?.source ?? '';
  return s.startsWith('fallback_') || s.startsWith('refusal_');
}

/** isSearched: the note carries a search table. */
export function isSearched(e: ReviewDecision): boolean {
  return !!note(e)?.search;
}

/**
 * isPassOnly: every option offered was "Pass priority" or "Concede" -- a
 * priority window with nothing to do, which is most of a game's decisions.
 * A capped option list (options_total above what was sent) is never
 * pass-only: the unsent options were something else.
 */
export function isPassOnly(e: ReviewDecision): boolean {
  return (
    e.options.length > 0 &&
    e.options.length === e.options_total &&
    e.options.every((o) => o.kind === 'pass' || o.kind === 'concede')
  );
}

/** decisionStops is the Decision step's stops when passes are skipped: every ask_seq but the pass-only ones'. */
export function decisionStops(entries: readonly ReviewDecision[]): number[] {
  return entries.filter((e) => !isPassOnly(e)).map((e) => e.ask_seq);
}

/** hasExplain: any entry of the index carries a note. */
export function hasExplain(entries: readonly ReviewDecision[]): boolean {
  return entries.some((e) => note(e) !== null);
}

/** bySeat is the predicate for one seat's decisions. */
export const bySeat = (seat: number) => (e: ReviewDecision) => e.seat === seat;

/**
 * entryIndexAt is the index of the entry whose ask_seq <= cursor < the next
 * entry's ask_seq: the decision the board at `cursor` is showing or has just
 * passed. -1 before the first decision. Entries are in ask_seq order.
 */
export function entryIndexAt(entries: readonly ReviewDecision[], cursor: number): number {
  let lo = 0;
  let hi = entries.length - 1;
  let hit = -1;
  while (lo <= hi) {
    const mid = (lo + hi) >> 1;
    if (entries[mid].ask_seq <= cursor) {
      hit = mid;
      lo = mid + 1;
    } else hi = mid - 1;
  }
  return hit;
}

/** entryAt is entryIndexAt's entry, or null before the first decision. */
export function entryAt(entries: readonly ReviewDecision[], cursor: number): ReviewDecision | null {
  const i = entryIndexAt(entries, cursor);
  return i < 0 ? null : entries[i];
}

/**
 * nextEntry is the first entry matching pred strictly after (dir 1) or
 * before (dir -1) the cursor's decision, or null. Sitting between two
 * decisions, backward finds the one the cursor is past if it matches — the
 * way a "previous" key returns to the decision just shown dimmed.
 */
export function nextEntry(
  entries: readonly ReviewDecision[],
  cursor: number,
  dir: 1 | -1,
  pred: (e: ReviewDecision) => boolean,
): ReviewDecision | null {
  const at = entryIndexAt(entries, cursor);
  if (dir > 0) {
    for (let i = at + 1; i < entries.length; i++) if (pred(entries[i])) return entries[i];
    return null;
  }
  const exact = at >= 0 && entries[at].ask_seq === cursor;
  for (let i = exact ? at - 1 : at; i >= 0; i--) if (pred(entries[i])) return entries[i];
  return null;
}

/** labelOf is the option labels of a choice list, as the decision listed them. */
export function labelOf(e: ReviewDecision, choices: readonly number[]): string {
  if (choices.length === 0) return 'Nothing';
  return choices.map((c) => e.options.find((o) => o.index === c)?.label ?? `option ${c}`).join('; ');
}

/** duplicateLabels is the option labels the entry lists more than once (two copies of a card in hand). */
export function duplicateLabels(e: ReviewDecision | null): Set<string> {
  const seen: Record<string, number> = {};
  for (const o of e?.options ?? []) seen[o.label] = (seen[o.label] ?? 0) + 1;
  return new Set(Object.keys(seen).filter((l) => seen[l] > 1));
}

/** A source badge: short text and a plain-language explanation. */
export interface SourceBadge {
  text: string;
  tip: string;
  tone: 'policy' | 'plain' | 'fallback';
}

export function sourceBadge(n: ExplainNote): SourceBadge {
  switch (n.source) {
    case 'policy':
      return { text: 'Policy', tip: `The seat's ${n.policy ? `${n.policy} ` : ''}net or search chose this answer.`, tone: 'policy' };
    case 'heuristic':
      return { text: 'Heuristic', tip: "The seat's own heuristic bot answered: a plain bot seat, or a kind the policy does not search.", tone: 'plain' };
    case 'forced':
      return { text: 'Forced', tip: 'Not searched: there were fewer than two candidate answers.', tone: 'plain' };
    case 'prior_skip':
      return { text: 'Prior skip', tip: 'Not searched: too many options, so the prior ranked them instead.', tone: 'plain' };
    case 'fallback_feed':
      return { text: 'Feed fallback', tip: 'The search feed stopped, so the driver answered with its fallback.', tone: 'fallback' };
    case 'fallback_root':
      return { text: 'Root fallback', tip: `The search root was refused${n.reason ? ` (${n.reason})` : ''}, so the driver answered with its fallback.`, tone: 'fallback' };
    case 'refusal_retry':
      return { text: 'Refusal retry', tip: `The engine refused the seat's answer${n.reason ? ` (${n.reason})` : ''}; its second answer was accepted.`, tone: 'fallback' };
    case 'refusal_fallback':
      return { text: 'Refusal fallback', tip: `The engine refused the seat's answer${n.reason ? ` (${n.reason})` : ''}; shared fallback${n.rung ? ` rung ${n.rung}` : ''} was accepted.`, tone: 'fallback' };
    default:
      return { text: String(n.source || 'Unknown'), tip: 'An explain source this client does not know.', tone: 'plain' };
  }
}

/** A tick on the transport's timeline. */
export interface TimelineMark {
  seq: number;
  kind: 'override' | 'fallback';
  tip: string;
}

/**
 * timelineMarks is one tick per override and per fallback entry (a fallback
 * that also overrode is drawn as a fallback). Empty when the record carries
 * no explain: a tick for every decision would be noise.
 */
export function timelineMarks(entries: readonly ReviewDecision[], seatName: (seat: number) => string): TimelineMark[] {
  const out: TimelineMark[] = [];
  for (const e of entries) {
    const n = note(e);
    if (!n) continue;
    if (isFallback(e)) out.push({ seq: e.ask_seq, kind: 'fallback', tip: `Decision ${e.intent}: ${sourceBadge(n).text}, ${seatName(e.seat)}` });
    else if (isOverride(e)) out.push({ seq: e.ask_seq, kind: 'override', tip: `Decision ${e.intent}: ${seatName(e.seat)} overrode the bot` });
  }
  return out;
}

/** signed prints a value with its sign as given: +0.25, −0.40, 0.00. */
export function signed(v: number, digits = 2): string {
  const s = Math.abs(v).toFixed(digits);
  if (Number(s) === 0) return s;
  return v < 0 ? `−${s}` : `+${s}`;
}
