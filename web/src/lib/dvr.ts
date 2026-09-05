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
}

export type DvrAction =
  | { type: 'snapshot'; match: string; head: number; turnStarts: number[] }
  | { type: 'event'; body: EventBody }
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
    case 'event': {
      const seq = a.body.event.seq;
      if (seq <= s.head && s.events.some((e) => e.event.seq === seq)) return s; // duplicate
      if (seq !== s.head + 1) return { ...s, gap: true };
      return { ...s, head: seq, cursor: s.live ? seq : s.cursor, events: [...s.events, a.body] };
    }
    case 'backfill': {
      const known = new Set(s.events.map((e) => e.event.seq));
      const older = a.events.filter((e) => !known.has(e.event.seq) && e.event.seq <= s.head);
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
