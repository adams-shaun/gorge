/**
 * eventseat.ts is the draft-event page's seat identity: the invite link's
 * `?seat=N&token=T`, read once when the event page mounts — the same shape
 * and the same hygiene as a table's join URL (./seat.ts parseSeat). The
 * token is the event seat token the server minted at create time; the
 * server alone maps it to a seat (the `seat` number is display-only, never
 * an authority).
 *
 * The URL keeps its query, exactly as a table's join URL does, so a reload
 * keeps the seat. It is also remembered per event in localStorage, so the
 * player who comes back to a bare `/e/{id}` (a bookmark, a link out of a
 * finished game) is still seated. Like ./seat.ts, the token is never
 * logged, rendered or interpolated into a visible string.
 */

import { parseSeat } from './seat';
import { safeStorage } from './storage';

export interface EventSeat {
  seat: number;
  token: string;
}

const KEY = 'gorge.event.seat.';
const NAME_KEY = 'gorge.event.name';

/** eventSeatFor resolves the viewer's seat for one event: the URL's ?seat=&token= wins (and is remembered), else the remembered pair, else null (a spectator). */
export function eventSeatFor(event: string, search: string, storage: Storage | null = safeStorage()): EventSeat | null {
  const fromURL = parseSeat(search);
  if (fromURL) {
    const seat = { seat: fromURL.seat, token: fromURL.token };
    try {
      storage?.setItem(KEY + event, JSON.stringify(seat));
    } catch {
      /* quota or private mode: the URL still carries the seat */
    }
    return seat;
  }
  try {
    const raw = storage?.getItem(KEY + event);
    if (!raw) return null;
    const v = JSON.parse(raw) as Partial<EventSeat>;
    if (typeof v.seat !== 'number' || typeof v.token !== 'string' || v.token === '') return null;
    // Re-validate through the same parser a URL goes through.
    const again = parseSeat(`?seat=${v.seat}&token=${encodeURIComponent(v.token)}`);
    return again ? { seat: again.seat, token: again.token } : null;
  } catch {
    return null;
  }
}

/** forgetEventSeat drops a remembered seat the server refused (an unknown token). */
export function forgetEventSeat(event: string, storage: Storage | null = safeStorage()): void {
  try {
    storage?.removeItem(KEY + event);
  } catch {
    /* best-effort */
  }
}

/** rememberedName is the display name the player last joined an event with. */
export function rememberedName(storage: Storage | null = safeStorage()): string {
  try {
    return storage?.getItem(NAME_KEY) ?? '';
  } catch {
    return '';
  }
}

export function rememberName(name: string, storage: Storage | null = safeStorage()): void {
  try {
    storage?.setItem(NAME_KEY, name);
  } catch {
    /* best-effort */
  }
}

/**
 * tableEventSeat is the table page's event context. A series game's
 * join link is `/t/{table}?seat=&token=&event=<id>`: its token is the GAME
 * token, so the event seat comes ONLY from the pair the event page
 * remembered for that event — the URL's seat and token are never read (or
 * stored) as an event seat. Null without `event=` or without a remembered
 * seat (a spectator, or a browser that never opened the event page).
 */
export function tableEventSeat(search: string, storage: Storage | null = safeStorage()): { event: string; seat: EventSeat } | null {
  const event = new URLSearchParams(search).get('event');
  if (!event) return null;
  const seat = eventSeatFor(event, '', storage);
  return seat ? { event, seat } : null;
}
