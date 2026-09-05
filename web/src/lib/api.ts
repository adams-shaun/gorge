import type { ErrorBody, EventBody, MatchInfo, TableInfo, View } from '../protocol';

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string, public head?: number) {
    super(message);
  }
}

const enc = encodeURIComponent;
export const tablesURL = () => '/api/tables';
export const matchesURL = (t: string) => `/api/tables/${enc(t)}/matches`;
export const viewURL = (t: string, k: number, seq?: number) =>
  `/api/tables/${enc(t)}/matches/${k}/view${seq === undefined ? '' : `?seq=${seq}`}`;
export const eventsURL = (t: string, k: number, since: number) =>
  `/api/tables/${enc(t)}/matches/${k}/events?since=${since}`;

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
export const fetchView = (t: string, k: number, seq?: number) => getJSON<View>(viewURL(t, k, seq));
export const fetchEvents = (t: string, k: number, since: number) => getJSON<EventBody[]>(eventsURL(t, k, since));
export const subscribe = (session: string, table: string, mode: 'overview' | 'focus') =>
  postJSON('/api/subscribe', { session, table, mode });
export const unsubscribe = (session: string, table: string) => postJSON('/api/unsubscribe', { session, table });
