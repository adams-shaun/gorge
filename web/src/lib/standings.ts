import type { EventPodGame, EventSeat, EventSeries, EventStanding } from '../protocol';
import { seatLabel } from './draftcards';

/** One rendered standings row. The ORDER is the server's (MTR Appendix C: points, OMW%, GW%, OGW%, seat). */
export interface StandingRow {
  rank: number;
  seat: number;
  label: string;
  you: boolean;
  points: number;
  record: string;
  mw: string;
  omw: string;
  gw: string;
  ogw: string;
}

/** pct renders a server ratio (0..1) as a whole percentage. */
export const pct = (x: number): string => `${Math.round(x * 100)}%`;

/** record is a seat's match W-L-D over ended series, read from the server's series rows. */
export function record(seat: number, series: EventSeries[]): string {
  let w = 0, l = 0, d = 0;
  for (const s of series) {
    if (s.state !== 'ended' || (s.a !== seat && s.b !== seat)) continue;
    if (s.winner === null || s.winner === undefined) d++;
    else if (s.winner === seat) w++;
    else l++;
  }
  return d > 0 ? `${w}-${l}-${d}` : `${w}-${l}`;
}

/**
 * podRecord is a seat's W-L(-D) over a Commander draft's ended pod games:
 * a win when the server named the seat the winner, a loss when it named
 * another seat, a draw when it named none.
 */
export function podRecord(seat: number, pod: EventPodGame[]): string {
  let w = 0, l = 0, d = 0;
  for (const g of pod) {
    if (g.state !== 'ended') continue;
    if (g.winner === null || g.winner === undefined) d++;
    else if (g.winner === seat) w++;
    else l++;
  }
  return d > 0 ? `${w}-${l}-${d}` : `${w}-${l}`;
}

/** standingRows labels the server's standings, in the server's order. With `pod` (a Commander draft) the record counts pod games. */
export function standingRows(
  standings: EventStanding[],
  seats: EventSeat[],
  series: EventSeries[],
  you: number | null,
  pod: EventPodGame[] | null = null,
): StandingRow[] {
  return (standings ?? []).map((s, i) => ({
    rank: i + 1,
    seat: s.seat,
    label: seatLabel(s.seat, seats[s.seat]?.name, you),
    you: you === s.seat,
    points: s.points,
    record: pod ? podRecord(s.seat, pod) : record(s.seat, series ?? []),
    mw: pct(s.mw),
    omw: pct(s.omw),
    gw: pct(s.gw),
    ogw: pct(s.ogw),
  }));
}
