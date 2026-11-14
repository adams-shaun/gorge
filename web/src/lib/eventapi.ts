/**
 * eventapi.ts is the client side of /api/events (host/httpapi/events.go).
 * Every call is an INTENT: the client sends card instance IDs, basic-land
 * counts and a display name; the server decides legality, auto lands, the
 * bot suggestion, pairings, standings and results, and this module only
 * carries its answers back. The event seat token rides
 * `Authorization: Bearer` (the server's first-choice form), never the URL,
 * and is never put into an error message.
 */

import type {
  CreateEventRequest,
  CreateEventResponse,
  DeckCheck,
  DeckRequest,
  DraftAdvice,
  DraftPack,
  ErrorBody,
  EventLog,
  EventPersonas,
  EventPool,
  EventSetInfo,
  EventState,
  PickRequest,
  SideboardRequest,
} from '../protocol';
import { ApiError, TIMEOUT_CODE } from './api';
import { withBase } from './basepath';

const enc = encodeURIComponent;
const TIMEOUT = 10_000;

export const eventsListURL = () => withBase('/api/events');
export const eventURL = (id: string, rest = '') => withBase(`/api/events/${enc(id)}${rest}`);

async function call<T>(method: 'GET' | 'POST', path: string, token: string | null, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (token) headers.Authorization = `Bearer ${token}`;
  const control = new AbortController();
  const timer = setTimeout(() => control.abort(), TIMEOUT);
  let res: Response;
  try {
    res = await fetch(path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body), signal: control.signal });
  } catch (e) {
    if (control.signal.aborted) throw new ApiError(0, TIMEOUT_CODE, `request timed out after ${TIMEOUT}ms`);
    throw e;
  } finally {
    clearTimeout(timer);
  }
  if (!res.ok) {
    const e = (await res.json().catch(() => ({}))) as Partial<ErrorBody>;
    throw new ApiError(res.status, e.code ?? 'http', e.message ?? res.statusText);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

/** EventApi is the seam the event page's state classes call through; tests may pass their own. */
export interface EventApi {
  state(id: string, token: string | null): Promise<EventState>;
  join(id: string, token: string, name: string): Promise<EventState>;
  pack(id: string, token: string): Promise<DraftPack>;
  pick(id: string, token: string, req: PickRequest): Promise<DraftPack>;
  pool(id: string, token: string): Promise<EventPool>;
  check(id: string, token: string, deck: DeckRequest): Promise<DeckCheck>;
  save(id: string, token: string, deck: DeckRequest): Promise<DeckCheck>;
  ready(id: string, token: string, rev: number): Promise<EventState>;
  /** sideboard locks the seat's deck for its series' next game: keep last game's, or a new list. */
  sideboard(id: string, token: string, req: SideboardRequest): Promise<EventState>;
  log(id: string, token: string | null): Promise<EventLog>;
  /** advice is the seat's own drafter's suggestion for the pack at its queue head; a 409 means this event's drafter gives none. */
  advice?(id: string, token: string): Promise<DraftAdvice>;
  subscribe(id: string, token: string | null, session: string): Promise<void>;
}

export const eventApi: EventApi = {
  state: (id, token) => call<EventState>('GET', eventURL(id), token),
  join: (id, token, name) => call<EventState>('POST', eventURL(id, '/join'), token, { name }),
  pack: (id, token) => call<DraftPack>('GET', eventURL(id, '/pack'), token),
  pick: (id, token, req) => call<DraftPack>('POST', eventURL(id, '/pick'), token, req),
  pool: (id, token) => call<EventPool>('GET', eventURL(id, '/pool'), token),
  check: (id, token, deck) => call<DeckCheck>('POST', eventURL(id, '/deck/check'), token, deck),
  save: (id, token, deck) => call<DeckCheck>('POST', eventURL(id, '/deck'), token, deck),
  ready: (id, token, rev) => call<EventState>('POST', eventURL(id, '/ready'), token, { rev }),
  sideboard: (id, token, req) => call<EventState>('POST', eventURL(id, '/sideboard'), token, req),
  log: (id, token) => call<EventLog>('GET', eventURL(id, '/log'), token),
  advice: (id, token) => call<DraftAdvice>('GET', eventURL(id, '/advice'), token),
  subscribe: (id, token, session) => call<void>('POST', eventURL(id, '/subscribe'), token, { session }),
};

/** listEvents is GET /api/events: every event's public state. A server without -events answers 404. */
export const listEvents = () => call<EventState[]>('GET', eventsListURL(), null);

/** createEvent is POST /api/events; the response carries each human seat's invite once. */
export const createEvent = (req: CreateEventRequest) => call<CreateEventResponse>('POST', eventsListURL(), null, req);

/** listEventSets is GET /api/events/sets: the server's draftable sets, with each one's pack size and rating source. */
export const listEventSets = () => call<EventSetInfo[]>('GET', withBase('/api/events/sets'), null);

/** listEventPersonas is GET /api/events/personas: the pickbot persona names a create body's drafter fields accept. */
export const listEventPersonas = () => call<EventPersonas>('GET', withBase('/api/events/personas'), null);

/** isStale reports a pick the server refused because the pod moved on (or the card left the pack): refetch, do not retry. */
export function isStale(e: unknown): boolean {
  return e instanceof ApiError && (e.code === 'stale_pick' || e.code === 'not_your_turn' || e.code === 'not_in_pack');
}
