import { withBase } from '../basepath';
import { bounded, openStream, WireError, type WireStream } from '../wire/http';
import type { Sbv2Request, Sbv2Send } from './wire';

const enc = encodeURIComponent;
const root = (t: string, k: number) => withBase(`/api/sbv2/v1/tables/${enc(t)}/matches/${k}`);
// asks=native: a person answers the asks the host otherwise answers for a v2
// agent (a land's colour, a triggered cost, a division), so the server poses them
// as native_only requests.
export const streamURL = (t: string, k: number, token: string) => `${root(t, k)}/stream?asks=native&token=${enc(token)}`;

async function refusal(res: Response): Promise<WireError> {
  const body = (await res.json().catch(() => ({}))) as { error?: string; code?: string; message?: string };
  const code = body.error ?? body.code ?? 'http';
  return new WireError(res.status, code, body.message ?? code);
}

/** requestOf parses one state document or stream frame: the seat's open request, or null between decisions. */
function requestOf(raw: unknown): Sbv2Request | null {
  return (raw as { request?: Sbv2Request | null } | null)?.request ?? null;
}

/** fetchState is GET …/state: the seat's open request, or null between decisions. */
export async function fetchState(t: string, k: number, token: string): Promise<Sbv2Request | null> {
  const res = await bounded(`${root(t, k)}/state?asks=native&token=${enc(token)}`, { headers: { Accept: 'application/json' } });
  if (!res.ok) throw await refusal(res);
  return requestOf(await res.json().catch(() => null));
}

/** send posts one answer; 204 is accepted. */
export async function send(t: string, k: number, token: string, msg: Sbv2Send): Promise<void> {
  const res = await bounded(`${root(t, k)}/send`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(msg),
  });
  if (!res.ok) throw await refusal(res);
}

export interface Transport {
  fetchState: typeof fetchState;
  send: typeof send;
  openStream: (url: string, onRequest: (r: Sbv2Request | null) => void, onError: () => void) => WireStream;
}

export const httpTransport: Transport = {
  fetchState,
  send,
  openStream: (url, onRequest, onError) => openStream(url, (data) => onRequest(requestOf(JSON.parse(data))), onError),
};
