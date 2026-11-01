/**
 * Test fixtures for the draft-event screens: a server-shaped EventState,
 * DraftPack and EventPool, and a fetch double that answers /api/events
 * routes from a handler table and records every request. Test-only; no
 * production module imports it.
 */
import { vi } from 'vitest';
import type { DraftCard, DraftPack, EventPool, EventState } from '../protocol';

/**
 * WIRE is the printed facts the server puts on each DraftCard, in its
 * wire spelling: a Forge-notation cost, the mana value (omitted at zero),
 * the colour identity as a WUBRG subset and the space-joined type line. A
 * name missing here arrives with no facts, as a card the server's corpus
 * does not know would.
 */
export const WIRE: Record<string, Pick<DraftCard, 'mana_cost' | 'mana_value' | 'colors' | 'type_line'>> = {
  'Serra Angel': { mana_cost: '3 W W', mana_value: 5, colors: 'W', type_line: 'Creature Angel' },
  'Lightning Bolt': { mana_cost: 'R', mana_value: 1, colors: 'R', type_line: 'Instant' },
  'Llanowar Elves': { mana_cost: 'G', mana_value: 1, colors: 'G', type_line: 'Creature Elf Druid' },
  'Giant Growth': { mana_cost: 'G', mana_value: 1, colors: 'G', type_line: 'Instant' },
  'Shivan Dragon': { mana_cost: '4 R R', mana_value: 6, colors: 'R', type_line: 'Creature Dragon' },
  'Evolving Wilds': { type_line: 'Land' },
  'Ornithopter': { type_line: 'Artifact Creature Thopter' },
  'Fusion Elemental': { mana_cost: 'W U B R G', mana_value: 5, colors: 'WUBRG', type_line: 'Creature Elemental' },
};

export const card = (id: string, name: string, rarity = 'C'): DraftCard => ({ id, name, set: 'FDN', number: '1', rarity, ...WIRE[name] });

export function eventState(over: Partial<EventState> = {}): EventState {
  return {
    event: 'e1',
    set: 'FDN',
    phase: 'DRAFT',
    round: 0,
    best_of: 1,
    you: 0,
    seats: [
      { seat: 0, kind: 'human', name: 'Ada', joined: true, picks: 2, ready: false },
      { seat: 1, kind: 'bot', name: 'Bot 2 (lethal-pressure)', policy: 'lethal-pressure', drafter: 'rating-colour', joined: true, picks: 2, ready: false },
      { seat: 2, kind: 'bot', name: 'Bot 3 (bot)', policy: 'bot', drafter: 'rating-colour', joined: true, picks: 2, ready: false },
      { seat: 3, kind: 'bot', name: 'Bot 4 (bot)', policy: 'bot', drafter: 'rating-colour', joined: true, picks: 2, ready: false },
    ],
    series: [],
    standings: [],
    ...over,
  };
}

export function draftPack(over: Partial<DraftPack> = {}): DraftPack {
  return {
    event: 'e1',
    seat: 0,
    waiting: false,
    round: 1,
    pick_no: 3,
    pack: [card('c10', 'Llanowar Elves'), card('c11', 'Shivan Dragon', 'R'), card('c12', 'Giant Growth')],
    pool: [card('c1', 'Serra Angel', 'U'), card('c2', 'Lightning Bolt')],
    ...over,
  };
}

export function eventPool(over: Partial<EventPool> = {}): EventPool {
  return {
    event: 'e1',
    seat: 0,
    pool: [card('c1', 'Serra Angel'), card('c2', 'Lightning Bolt'), card('c3', 'Llanowar Elves'), card('c4', 'Giant Growth')],
    deck: null,
    rev: 0,
    ready: false,
    suggestion: { main: ['c2', 'c3'], basics: [0, 0, 0, 8, 9] },
    ...over,
  };
}

export interface Recorded {
  method: string;
  path: string;
  auth: string | null;
  body: unknown;
}

type Reply = { status?: number; body?: unknown };
type Handler = (req: Recorded) => Reply | Promise<Reply>;

/**
 * fakeFetch installs a global fetch that routes "METHOD /path" (query
 * stripped) to a handler. An unrouted request answers 500 so a test sees it.
 */
export function fakeFetch(routes: Record<string, Handler | Reply>) {
  const calls: Recorded[] = [];
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const path = url.replace(/^https?:\/\/[^/]+/, '').split('?')[0];
    const method = init?.method ?? 'GET';
    const headers = (init?.headers ?? {}) as Record<string, string>;
    const rec: Recorded = { method, path, auth: headers.Authorization ?? null, body: init?.body ? JSON.parse(String(init.body)) : undefined };
    calls.push(rec);
    const h = routes[`${method} ${path}`];
    const reply = typeof h === 'function' ? await h(rec) : (h ?? { status: 500, body: { code: 'internal', message: `unrouted ${method} ${path}` } });
    const status = reply.status ?? 200;
    return new Response(status === 204 ? null : JSON.stringify(reply.body ?? {}), { status, headers: { 'Content-Type': 'application/json' } });
  });
  vi.stubGlobal('fetch', fn);
  return { calls, fn };
}
