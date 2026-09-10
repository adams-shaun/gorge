import type { Decision, ErrorBody, EventBody, Intent, MatchInfo, TableInfo, View } from '../protocol';
import type { SeatCtx } from './seat';
import { withBase } from './basepath';

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string, public head?: number) {
    super(message);
  }
}

const enc = encodeURIComponent;
// Every path below is built with withBase (./basepath), the one base-path
// value the client reads once at startup. With an empty base (the default)
// each one is byte-identical to the pre-base client.
export const tablesURL = () => withBase('/api/tables');
export const matchesURL = (t: string) => withBase(`/api/tables/${enc(t)}/matches`);
export const pendingURL = (t: string, k: number) => withBase(`/api/tables/${enc(t)}/matches/${k}/pending`);
export const intentURL = (t: string, k: number) => withBase(`/api/tables/${enc(t)}/matches/${k}/intent`);
export const gamesURL = () => withBase('/api/games');
export const decksURL = () => withBase('/api/decks');

// seatQuery is the seat/token query threading on the seat-scoped GETs
// (M2e-3's FL-99: ?seat=N&token=…). The token is a bearer credential for
// the local fixture — it rides in the URL exactly as the join URL printed
// it, and never in a rendered string.
export const seatQuery = (ctx: SeatCtx) => `?seat=${ctx.seat}&token=${encodeURIComponent(ctx.token)}`;

// viewURL appends the optional ?seq= and, for a seated request, the
// seat/token pair. With neither, it is byte-identical to the pre-seat URL
// (R-E4-4: no seat in the URL, no behaviour change).
export const viewURL = (t: string, k: number, seq?: number, ctx?: SeatCtx) => {
  const q = new URLSearchParams();
  if (seq !== undefined) q.set('seq', String(seq));
  if (ctx) {
    q.set('seat', String(ctx.seat));
    q.set('token', ctx.token);
  }
  const s = q.toString();
  return withBase(`/api/tables/${enc(t)}/matches/${k}/view${s === '' ? '' : `?${s}`}`);
};
export const eventsURL = (t: string, k: number, since: number, ctx?: SeatCtx) => {
  const q = new URLSearchParams({ since: String(since) });
  if (ctx) {
    q.set('seat', String(ctx.seat));
    q.set('token', ctx.token);
  }
  return withBase(`/api/tables/${enc(t)}/matches/${k}/events?${q.toString()}`);
};

export async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Accept: 'application/json' } });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as Partial<ErrorBody>;
    throw new ApiError(res.status, body.code ?? 'http', body.message ?? res.statusText, body.head);
  }
  return (await res.json()) as T;
}

async function postJSON(path: string, body: unknown): Promise<void> {
  const res = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  if (!res.ok) {
    const e = (await res.json().catch(() => ({}))) as Partial<ErrorBody>;
    throw new ApiError(res.status, e.code ?? 'http', e.message ?? res.statusText);
  }
}

export const fetchTables = () => getJSON<TableInfo[]>(tablesURL());
export const fetchMatches = (t: string) => getJSON<MatchInfo[]>(matchesURL(t));

export interface DeckInfo {
  id: string;
  name: string;
  format: 'constructed' | 'commander';
  archetype: string;
  commander?: string;
}

export const fetchDecks = () => getJSON<DeckInfo[]>(decksURL());

/** A game a successful POST /api/games returns: the new table's identity, the human seat and its bearer token, and the join path that carries them (Task ui11). */
export interface CreateGame {
  table: string;
  match: number;
  seed: number;
  seat: number;
  token: string;
  join: string;
}

export interface CreateGameRequest {
  format: 'constructed' | 'commander';
  human_deck?: string;
  bot_deck?: string;
}

/** createGame asks the server to seat the player against a bot in the given format: a fresh single-shot table is created and started, and the join path it returns is the URL the player follows to sit in the seat. Omitted deck ids retain random assignment. A server that did not enable the play-vs-bot flow answers 404, surfaced as ApiError. */
export async function createGame(request: CreateGameRequest): Promise<CreateGame> {
  const res = await fetch(gamesURL(), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });
  if (!res.ok) {
    const e = (await res.json().catch(() => ({}))) as Partial<ErrorBody>;
    throw new ApiError(res.status, e.code ?? 'http', e.message ?? res.statusText);
  }
  return (await res.json()) as CreateGame;
}
export const fetchView = (t: string, k: number, seq?: number, ctx?: SeatCtx) => getJSON<View>(viewURL(t, k, seq, ctx));
export const fetchEvents = (t: string, k: number, since: number, ctx?: SeatCtx) => getJSON<EventBody[]>(eventsURL(t, k, since, ctx));

/** fetchPending is the seat-scoped GET that names the decision currently asked of the seat — the full Decision, options included. A 409 conflict (nothing pending for this seat) rejects like any other server answer. */
export const fetchPending = (t: string, k: number, ctx: SeatCtx) => getJSON<Decision>(pendingURL(t, k) + seatQuery(ctx));

/** postIntent answers a decision. It takes no ?seat= (the claim is the fence, FL-99: Authorization: Bearer is the accepted second form), and the intent body's own seq/player/choices are validated server-side. */
export async function postIntent(t: string, k: number, intent: Intent, ctx: SeatCtx): Promise<void> {
  const res = await fetch(intentURL(t, k), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${ctx.token}` },
    body: JSON.stringify(intent),
  });
  if (!res.ok) {
    const e = (await res.json().catch(() => ({}))) as Partial<ErrorBody>;
    throw new ApiError(res.status, e.code ?? 'http', e.message ?? res.statusText);
  }
}
export const subscribe = (session: string, table: string, mode: 'overview' | 'focus') =>
  postJSON(withBase('/api/subscribe'), { session, table, mode });
export const unsubscribe = (session: string, table: string) => postJSON(withBase('/api/unsubscribe'), { session, table });
