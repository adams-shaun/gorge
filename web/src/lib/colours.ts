import type { SeatInfo } from '../protocol';

/** Mirrors protocol.SeatColours; the server's SeatInfo.colour wins when known. */
export const SEAT_COLOURS: readonly string[] = ['#e5484d', '#3b82f6', '#22c55e', '#eab308', '#a855f7', '#f97316', '#14b8a6', '#ec4899'];

export function seatColour(i: number, seats?: SeatInfo[]): string {
  return seats?.[i]?.colour ?? SEAT_COLOURS[i % SEAT_COLOURS.length];
}
