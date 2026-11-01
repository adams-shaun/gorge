import type { EventBody } from '../protocol';

/**
 * The DVR is a cursor over a match's event sequence. The match never
 * pauses; the client does. `live` pins the cursor to `head`; pause/step/
 * scrub move it; `live` snaps back. Events are kept contiguous: an event
 * that is not head+1 is a gap, and the owner re-snapshots.
 */
export interface DvrState {
  match: string | null;
  head: number;
  cursor: number;
  live: boolean;
  events: EventBody[];
  turnStarts: number[];
  gap: boolean;
  /**
   * losses holds the player_lost events trimmed out of a live window (see
   * LIVE_KEEP), so a seat's loss reason outlives the window. Optional: a
   * state built without it has trimmed nothing.
   */
  losses?: EventBody[];
}

/**
 * LIVE_KEEP bounds the events a LIVE DVR holds. A spectator receives every
 * event of the match (~100/s on a 50ms-paced commander table), and holding
 * them all grew memory and every per-event pass over the log without bound.
 * Older events are refetched on demand: showCursor backfills when the
 * cursor precedes the first event held. A paused DVR is never trimmed.
 */
export const LIVE_KEEP = 3000;
// Trim in steps rather than per event, so the slice is amortised.
const LIVE_SLACK = 500;

export type DvrAction =
  | { type: 'snapshot'; match: string; head: number; turnStarts: number[] }
  // A rewind reuses the same match identity with a shorter history. Unlike a
  // same-match snapshot, it must discard the old tail and return to live.
  | { type: 'rewind'; match: string; head: number; turnStarts: number[] }
  | { type: 'event'; body: EventBody }
  // 'head' advances the cursor/head WITHOUT storing a transcript line: the
  // seated path receives the spectator frame stream but must not render its
  // bodies (they are redacted for the table's spectator visibility, not for
  // a seat), so it chits the public seq and backfills redacted lines via
  // REST instead.
  | { type: 'head'; seq: number }
  | { type: 'backfill'; events: EventBody[] }
  | { type: 'pause' }
  | { type: 'live' }
  | { type: 'step'; by: number }
  | { type: 'scrub'; seq: number }
  | { type: 'reset' };

export const initialDvr: DvrState = { match: null, head: 0, cursor: 0, live: true, events: [], turnStarts: [], gap: false };

const clamp = (n: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, n));

export function dvrReducer(s: DvrState, a: DvrAction): DvrState {
  switch (a.type) {
    case 'reset':
      return initialDvr;
    case 'snapshot': {
      const same = s.match === a.match;
      const live = same ? s.live : true;
      return {
        match: a.match, head: a.head, live,
        cursor: live ? a.head : clamp(s.cursor, 0, a.head),
        events: [], turnStarts: [...a.turnStarts], gap: false,
      };
    }
    case 'rewind':
      return {
        match: a.match, head: a.head, cursor: a.head, live: true,
        events: [], turnStarts: [...a.turnStarts], gap: false,
      };
    case 'event': {
      const seq = a.body.event.seq;
      if (seq <= s.head) return s; // already accounted for: a duplicate, or redelivered after a snapshot dropped events
      // sticky until 'snapshot' or 'reset': the caller must re-snapshot to clear it
      if (seq !== s.head + 1) return { ...s, gap: true };
      const next = { ...s, head: seq, cursor: s.live ? seq : s.cursor, events: [...s.events, a.body] };
      return s.live && next.events.length > LIVE_KEEP + LIVE_SLACK ? trimLive(next) : next;
    }
    case 'head': {
      if (a.seq <= s.head) return s;
      return { ...s, head: a.seq, cursor: s.live ? a.seq : s.cursor };
    }
    case 'backfill': {
      // The batch is deduped against what we already hold AND against itself.
      // Only the first guard existed, so a batch that repeated a seq internally
      // put both copies in the log -- and the transcript keys its rows by
      // event.seq, so Svelte threw each_key_duplicate and stopped rendering
      // that subtree. Adding each accepted seq to `known` as we go makes the
      // filter idempotent over the batch.
      const known = new Set(s.events.map((e) => e.event.seq));
      const older = a.events.filter((e) => {
        const seq = e.event.seq;
        if (known.has(seq) || seq > s.head) return false;
        known.add(seq);
        return true;
      });
      const events = [...older, ...s.events].sort((x, y) => x.event.seq - y.event.seq);
      return { ...s, events };
    }
    case 'pause':
      return { ...s, live: false };
    case 'live':
      return { ...s, live: true, cursor: s.head };
    case 'step':
      return { ...s, live: false, cursor: clamp(s.cursor + a.by, 0, s.head) };
    case 'scrub':
      return { ...s, live: false, cursor: clamp(a.seq, 0, s.head) };
  }
}

/** trimLive keeps the newest LIVE_KEEP events, setting aside trimmed losses. */
function trimLive(s: DvrState): DvrState {
  const cut = s.events.length - LIVE_KEEP;
  const lost = s.events.slice(0, cut).filter((e) => e.event.kind === 'player_lost');
  return { ...s, events: s.events.slice(cut), losses: lost.length ? [...(s.losses ?? []), ...lost] : s.losses };
}

/**
 * railEvents is what the seat rail reads (loss reasons, the last resolved
 * card): the held events, preceded by any trimmed losses older than them.
 */
export function railEvents(s: DvrState): EventBody[] {
  const first = s.events[0]?.event.seq ?? Infinity;
  const older = (s.losses ?? []).filter((e) => e.event.seq < first);
  return older.length ? [...older, ...s.events] : s.events;
}

export const behindLive = (s: DvrState): number => s.head - s.cursor;

export function eventAt(s: DvrState, seq: number): EventBody | undefined {
  return s.events.find((e) => e.event.seq === seq);
}

/** turnOf is the index of the turn containing seq: the last turn start <= seq, or -1. */
export function turnOf(s: DvrState, seq: number): number {
  let i = -1;
  for (let k = 0; k < s.turnStarts.length && s.turnStarts[k] <= seq; k++) i = k;
  return i;
}
