import type { EventBody } from '../protocol';
import type { DvrState } from './dvr';

/**
 * Replay playback over the DVR cursor: where a step lands, what the turn
 * timeline draws, and the shareable URL query. Pure functions; the
 * ReplayTransport component owns the timer and dispatches 'scrub'.
 *
 * Units:
 *  - event: one event of the log (seq ± 1);
 *  - decision: one answered decision — the cursor sits on each intent's
 *    decision_ask, so the board shows the decision the player faced;
 *  - turn: one engine turn (a 'turn' event).
 */
export type StepUnit = 'event' | 'decision' | 'turn';
export const STEP_UNITS: readonly StepUnit[] = ['event', 'decision', 'turn'];

export const SPEEDS: readonly number[] = [0.5, 1, 2, 4, 8];

/** Base milliseconds per step at 1x, per unit: an event is tiny, a turn is a lot. */
const BASE_MS: Record<StepUnit, number> = { event: 150, decision: 700, turn: 1800 };

export function intervalMs(unit: StepUnit, speed: number): number {
  return Math.max(16, Math.round(BASE_MS[unit] / (speed > 0 ? speed : 1)));
}

/** seqsOfKind lists the seqs of one event kind in log order. */
export function seqsOfKind(events: readonly EventBody[], kind: string): number[] {
  const out: number[] = [];
  for (const e of events) if (e.event.kind === kind) out.push(e.event.seq);
  return out;
}

/** The first stop strictly after (dir 1) or before (dir -1) cursor in a sorted list, or null. */
export function nextStop(stops: readonly number[], cursor: number, dir: 1 | -1): number | null {
  if (dir > 0) {
    for (const s of stops) if (s > cursor) return s;
    return null;
  }
  for (let i = stops.length - 1; i >= 0; i--) if (stops[i] < cursor) return stops[i];
  return null;
}

/**
 * stepTarget is where one step of `unit` in `dir` moves the cursor. Past the
 * last stop it lands on head (forward) or 0 (back), so a step is never a no-op
 * while there is log left to move through. decisionStops, when given,
 * replaces the decision unit's stops (a review table skipping pass-only
 * decisions, lib/review decisionStops); otherwise every decision_ask stops.
 */
export function stepTarget(dvr: DvrState, unit: StepUnit, dir: 1 | -1, decisionStops?: readonly number[] | null): number {
  const clamp = (n: number) => Math.max(0, Math.min(dvr.head, n));
  if (unit === 'event') return clamp(dvr.cursor + dir);
  const stops = unit === 'turn' ? dvr.turnStarts : (decisionStops ?? seqsOfKind(dvr.events, 'decision_ask'));
  return nextStop(stops, dvr.cursor, dir) ?? (dir > 0 ? dvr.head : 0);
}

/** One engine turn on the timeline: [start, end) in seqs, who was active, its round. */
export interface TurnBand {
  start: number;
  end: number;
  turn: number;
  round: number;
  player: number | null;
}

/**
 * turnBands draws the timeline: one band per engine turn, from its 'turn'
 * event to the next (the last runs to head). The active player comes from the
 * turn event; the round, which the phase bar labels "Turn N", is derived
 * from the turn number and the seat count (an extra turn shifts it, the
 * same approximation the client's snapshot path already uses).
 */
export function turnBands(dvr: DvrState, seats: number): TurnBand[] {
  const byStart = new Map<number, EventBody>();
  for (const e of dvr.events) if (e.event.kind === 'turn') byStart.set(e.event.seq, e);
  const n = Math.max(1, seats);
  return dvr.turnStarts.map((start, i) => {
    const e = byStart.get(start);
    const turn = e?.event.amount ?? i + 1;
    return {
      start,
      end: dvr.turnStarts[i + 1] ?? dvr.head + 1,
      turn,
      round: Math.floor((turn - 1) / n) + 1,
      player: e?.event.player ?? null,
    };
  });
}

/** bandAt is the band containing seq, or null before turn 1. */
export function bandAt(bands: readonly TurnBand[], seq: number): TurnBand | null {
  let hit: TurnBand | null = null;
  for (const b of bands) {
    if (b.start > seq) break;
    hit = b;
  }
  return hit;
}

/** The replay position and playback settings a URL query carries. */
export interface ReplayQuery {
  seq?: number;
  /** turn is the round the phase bar shows as "Turn N" (1-based). */
  turn?: number;
  /** intent N is the board with intent N pending — the same point `repro -at N` prints. */
  intent?: number;
  unit?: StepUnit;
  speed?: number;
  play?: boolean;
}

const nonNegInt = (v: string | null): number | undefined => {
  if (v === null || v.trim() === '' || !/^\d+$/.test(v.trim())) return undefined;
  return Number(v.trim());
};

/** parseReplayQuery reads ?seq= / ?event=, ?turn=, ?intent= / ?decision=, ?unit=, ?speed=, ?play=. */
export function parseReplayQuery(search: string): ReplayQuery {
  const p = new URLSearchParams(search);
  const q: ReplayQuery = {};
  const seq = nonNegInt(p.get('seq') ?? p.get('event'));
  if (seq !== undefined) q.seq = seq;
  const turn = nonNegInt(p.get('turn'));
  if (turn !== undefined && turn > 0) q.turn = turn;
  const intent = nonNegInt(p.get('intent') ?? p.get('decision'));
  if (intent !== undefined) q.intent = intent;
  const unit = p.get('unit');
  if (unit && (STEP_UNITS as readonly string[]).includes(unit)) q.unit = unit as StepUnit;
  const speed = Number(p.get('speed'));
  if (SPEEDS.includes(speed)) q.speed = speed;
  const play = p.get('play');
  if (play === '1' || play === 'true') q.play = true;
  return q;
}

/**
 * resolveStart turns a query into a cursor. Precedence: seq, then intent,
 * then turn; null when the query names no position (the caller keeps its
 * default, the end of the game).
 */
export function resolveStart(q: ReplayQuery, dvr: DvrState, seats: number): number | null {
  const clamp = (n: number) => Math.max(0, Math.min(dvr.head, n));
  if (q.seq !== undefined) return clamp(q.seq);
  if (q.intent !== undefined) {
    const asks = seqsOfKind(dvr.events, 'decision_ask');
    return asks[q.intent] ?? dvr.head;
  }
  if (q.turn !== undefined) {
    const band = turnBands(dvr, seats).find((b) => b.round >= q.turn!);
    return band ? band.start : dvr.head;
  }
  return null;
}

/**
 * replaySearch writes the current position back as a query, keeping any
 * unrelated parameters, so a copied URL reopens exactly here. Only seq is
 * written for the position (it is exact); unit and speed only when not the
 * default.
 */
export function replaySearch(search: string, cursor: number, unit: StepUnit, speed: number): string {
  const p = new URLSearchParams(search);
  for (const k of ['seq', 'event', 'turn', 'intent', 'decision', 'play']) p.delete(k);
  p.set('seq', String(cursor));
  if (unit !== 'decision') p.set('unit', unit);
  else p.delete('unit');
  if (speed !== 1) p.set('speed', String(speed));
  else p.delete('speed');
  const s = p.toString();
  return s ? `?${s}` : '';
}
