import { withBase } from '../basepath';
import { bounded, openStream, WireError, type ESCtor, type WireStream } from '../wire/http';
import { parseEngineMessage, type ClientMessage, type EngineMessage, type ProtocolError } from './wire';

/**
 * transport.ts is gorge's ManaBrew wire transport, client side
 * (host/manabrewhttp): an SSE stream of engine messages,
 * a POST per client message, and a one-shot state read. Every route is
 * seat-claim fenced; EventSource cannot set headers, so the token rides as
 * `?token=` on the GETs and as a Bearer header on the POST.
 */

const enc = encodeURIComponent;
const root = (t: string, k: number) => withBase(`/api/manabrew/v0/tables/${enc(t)}/matches/${k}`);
export const streamURL = (t: string, k: number, token: string) => `${root(t, k)}/stream?token=${enc(token)}`;
export const stateURL = (t: string, k: number, token: string) => `${root(t, k)}/state?token=${enc(token)}`;
export const sendURL = (t: string, k: number) => `${root(t, k)}/send`;

async function refusal(res: Response): Promise<WireError> {
  const body = (await res.json().catch(() => ({}))) as Partial<ProtocolError>;
  return new WireError(res.status, String(body.code ?? 'http'), body.message ?? res.statusText, typeof body.promptId === 'number' ? body.promptId : undefined);
}

/** fetchState is GET …/state: the current state and the open prompt, if any. */
export async function fetchState(t: string, k: number, token: string): Promise<EngineMessage[]> {
  const res = await bounded(stateURL(t, k, token), { headers: { Accept: 'application/json' } });
  if (!res.ok) throw await refusal(res);
  const body = (await res.json().catch(() => null)) as unknown;
  if (!Array.isArray(body)) throw new WireError(res.status, 'invalid', 'the ManaBrew state route did not answer a message list');
  return body.map((m) => parseEngineMessage(JSON.stringify(m))).filter((m): m is EngineMessage => m !== null);
}

/** send posts one client message; 204 is accepted, anything else rejects with the server's ProtocolError. */
export async function send(t: string, k: number, token: string, msg: ClientMessage): Promise<void> {
  const res = await bounded(sendURL(t, k), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(msg),
  });
  if (!res.ok) throw await refusal(res);
}

/**
 * openMbStream owns the ManaBrew stream. The server answers every connect
 * with the full state and the open prompt, so a reconnect needs no bookkeeping.
 */
export function openMbStream(url: string, onMessage: (m: EngineMessage) => void, onError: () => void, es?: ESCtor): WireStream {
  return openStream(url, (data) => {
    const m = parseEngineMessage(data);
    if (m) onMessage(m);
  }, onError, es);
}

export interface Transport {
  fetchState: typeof fetchState;
  send: typeof send;
  openStream: (url: string, onMessage: (m: EngineMessage) => void, onError: () => void) => WireStream;
}

export const httpTransport: Transport = { fetchState, send, openStream: (u, m, e) => openMbStream(u, m, e) };
