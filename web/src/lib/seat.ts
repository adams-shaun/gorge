/**
 * seat.ts is the page's seat identity, read ONCE from the join URL that
 * M2e-3's gorged prints on startup (FL-99): `?seat=N&token=…`. The seat is
 * "which player am I acting for" — a number and a bear credential, nothing
 * more. No rules knowledge lives here; the server's decision.Validate is
 * the whole contract.
 *
 * The token is a bearer credential for a local fixture. It is held here and
 * built into request URLs in api.ts (and, for the ManaBrew wire,
 * lib/manabrew/transport.ts), and nowhere else: never logged, never
 * put in the transcript, never rendered, never echoed in an error message.
 * The render path (routes/components) receives only SeatCtx via getSeat,
 * and no component string ever interpolates `token`.
 *
 * The join URL's seat is also remembered per table in localStorage, exactly
 * as eventseat.ts remembers an event seat. A table's match link
 * (/t/{table}/m/{k}) carries no query, so without this a reload or a new tab
 * on a private play-vs-bot table's replay would load spectator-only and the
 * fenced REST reads would answer not-found. The remembered token is
 * re-validated through parseSeat before use, and is restored ONLY on that
 * match-replay path: a bare /t/{table} load stays spectator-only, as it was
 * before this store existed. A seat the server refuses is forgotten
 * (forgetTableSeat), so a stale token cannot bounce every later load.
 */

import type { Decision, Intent } from '../protocol';
import { safeStorage } from './storage';

/**
 * SeatTransport is an alternative game channel for the seat's answers (the
 * ManaBrew adapter, lib/manabrew). When a SeatCtx carries one, api.ts's
 * seat calls (postIntent, fetchPending, postUndo) go through it instead of
 * the native routes; the seat panel never knows which wire answered.
 */
export interface SeatTransport {
  postIntent(t: string, k: number, intent: Intent): Promise<void>;
  fetchPending(t: string, k: number): Promise<Decision>;
  postUndo(t: string, k: number): Promise<void>;
}

export interface SeatCtx {
  seat: number;
  token: string;
  transport?: SeatTransport;
}

const TOKEN_RE = /^[A-Za-z0-9._~+/-]+$/;

/** parseSeat reads ?seat= and ?token= out of a search string. Both must be present and well-formed, or there is no seat. */
export function parseSeat(search: string): SeatCtx | null {
  const q = new URLSearchParams(search);
  const seatRaw = q.get('seat');
  const token = q.get('token');
  if (seatRaw === null || token === null || token === '') return null;
  // a seat must be a non-negative integer; the token must be ordinary
  // bearer text (rejecting characters that could smuggle markup into a
  // render is the fixture's only hygiene — parse failures simply mean
  // "not a seat", and the page stays spectator-only)
  if (!/^\d+$/.test(seatRaw)) return null;
  if (!TOKEN_RE.test(token)) return null;
  return { seat: Number(seatRaw), token };
}

const TABLE_KEY = 'gorge.table.seat.';

/** tableOfPath is the /t/{table} prefix of a pathname, or null for another route. */
function tableOfPath(pathname: string): string | null {
  const m = pathname.match(/^\/t\/([^/]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

/**
 * isMatchReplayPath is /t/{table}/m/{k}, the only path that restores a
 * remembered seat. A bare /t/{table} load (a live table, a bookmark) keeps
 * the pre-store spectator default even when the browser remembers a seat.
 */
function isMatchReplayPath(pathname: string): boolean {
  return /^\/t\/[^/]+\/m\/[^/]+/.test(pathname);
}

function rememberSeat(table: string, ctx: SeatCtx): void {
  try {
    safeStorage()?.setItem(TABLE_KEY + table, JSON.stringify({ seat: ctx.seat, token: ctx.token }));
  } catch {
    /* quota or private mode: the URL still carries the seat for this page */
  }
}

function rememberedSeat(table: string): SeatCtx | null {
  try {
    const raw = safeStorage()?.getItem(TABLE_KEY + table);
    if (!raw) return null;
    const v = JSON.parse(raw) as Partial<SeatCtx>;
    if (typeof v.seat !== 'number' || typeof v.token !== 'string') return null;
    return parseSeat(`?seat=${v.seat}&token=${encodeURIComponent(v.token)}`);
  } catch {
    return null;
  }
}

let current: SeatCtx | null = null;
let currentTable: string | null = null;

/**
 * initSeatContext is called once at startup (main.ts) with location.search
 * and location.pathname. A join URL's seat is remembered for its table; a
 * match replay path (/t/{table}/m/{k}) restores the remembered seat for that
 * table. Any other table path stays spectator-only.
 */
export function initSeatContext(search: string, pathname = ''): SeatCtx | null {
  const table = tableOfPath(pathname);
  current = parseSeat(search);
  currentTable = table;
  if (current) {
    if (table) rememberSeat(table, current);
    return current;
  }
  currentTable = null;
  if (table && isMatchReplayPath(pathname)) {
    current = rememberedSeat(table);
    if (current) currentTable = table;
  }
  return current;
}

/**
 * forgetTableSeat drops a remembered seat the server refused (a redeployed
 * gorged rejects every old token), and clears it from the live page context
 * when it belongs to that table. Without this a stale token would be
 * restored on every later match link and bounce the browser to the lobby.
 */
export function forgetTableSeat(table: string): void {
  try {
    safeStorage()?.removeItem(TABLE_KEY + table);
  } catch {
    /* best-effort */
  }
  if (currentTable === table) {
    current = null;
    currentTable = null;
  }
}

/** getSeat returns the page's seat, if the join URL had one. */
export function getSeat(): SeatCtx | null {
  return current;
}
