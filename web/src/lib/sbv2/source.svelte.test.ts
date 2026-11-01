import { afterEach, describe, expect, it, vi } from 'vitest';
import type { MatchInfo, View } from '../../protocol';
import { fetchPending, postIntent, postUndo } from '../api';
import { WireError } from '../wire/http';
import { Sbv2Match } from './source.svelte';
import type { Transport } from './transport';
import type { NativeOp, Sbv2Request, Sbv2Send } from './wire';

const seats = [{ name: 'You', deck: 'a', colour: '', human: true }, { name: 'Bot', deck: 'b', colour: '' }];
const info: MatchInfo = { table: 'g1', match: 1, seed: 1, seats, state: 'live', winner: null, events: 10, turns: 1, bot_policy: 'bot' };

const board = (extra: Partial<View> = {}) => ({ turn: 1, step: 'main1', active: 0, over: false, stack: [], pending: [], players: [{ seat: 0, name: 'You', battlefield: [], graveyard: [], exile: [], command: [], hand: [], life: 20 }, { seat: 1, name: 'Bot', battlefield: [], graveyard: [], exile: [], command: [], life: 20 }], ...extra }) as unknown as View;

const passOrAttack = [{ index: 0, kind: 'activate', label: 'Tap', player: 0 }, { index: 1, kind: 'pass', label: 'Pass', player: 0 }, { index: 2, kind: 'concede', label: 'Concede', player: 0 }];

function request(id: number, nativeIndex: number, opts: { ops?: NativeOp[]; kind?: string; options?: unknown[]; log?: string[]; nativeOnly?: boolean; substep?: number; view?: View } = {}): Sbv2Request {
  const ops = opts.ops ?? [{ op: 'choose', option: 1 }];
  return {
    request_id: String(id),
    decision: {
      group: { substep_index: opts.substep ?? 0 },
      candidates: ops.map((_, i) => ({ candidate_id: i, semantic: { kind: 'pick' } })),
      extensions: {
        x_gorge_view_v1: { version: 1, native_index: opts.nativeOnly ? 0 : nativeIndex, view: opts.view ?? board(), decision: { seq: nativeIndex, player: 0, kind: opts.kind ?? 'priority', prompt: 'p', min: 1, max: 1, options: opts.options ?? passOrAttack } as never, ops: opts.nativeOnly ? null : ops },
        x_gorge_human_v1: { log: opts.log ?? [], labels: {}, can_undo: true, native_only: opts.nativeOnly ?? false },
      },
    },
  };
}

function fake(first: Sbv2Request | null, opts: { stateError?: WireError; sendError?: WireError } = {}) {
  const sent: Sbv2Send[] = [];
  const urls: string[] = [];
  let push: ((r: Sbv2Request | null) => void) | null = null;
  const transport: Transport = {
    fetchState: async () => {
      if (opts.stateError) throw opts.stateError;
      return first;
    },
    send: async (_t, _k, _tok, msg) => {
      if (opts.sendError) throw opts.sendError;
      sent.push(msg);
    },
    openStream: (url, onRequest) => {
      urls.push(url);
      push = onRequest;
      return { close: () => { push = null; } };
    },
  };
  return { transport, sent, urls, emit: (r: Sbv2Request | null) => push?.(r) };
}

async function started(first: Sbv2Request | null, opts: Parameters<typeof fake>[1] & { fetchMatches?: () => Promise<MatchInfo[]>; fetchView?: () => Promise<View> } = {}) {
  const f = fake(first, opts);
  const m = new Sbv2Match('g1', { seat: 0, token: 'tok' }, { transport: f.transport, fetchMatches: opts.fetchMatches ?? (async () => [info]), fetchView: opts.fetchView ?? (async () => board()) });
  const res = await m.start();
  return { m, f, res };
}

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('Sbv2Match', () => {
  it('falls back quietly on a table v2 does not cover', async () => {
    const { res, f } = await started(null, { stateError: new WireError(409, 'unsupported', 'unsupported') });
    expect(res).toMatchObject({ ok: false, quiet: true });
    expect(f.urls).toHaveLength(0);
  });

  it('falls back with a reason on a server without the routes', async () => {
    const { res } = await started(null, { stateError: new WireError(404, 'not_found', 'no such endpoint') });
    expect(res).toMatchObject({ ok: false, reason: expect.stringContaining('not enabled') });
    expect((res as { quiet?: boolean }).quiet).toBeUndefined();
  });

  it('paints the request: board, decision numbered by request, taps kept', async () => {
    const { m } = await started(request(3, 40));
    expect(m.view?.decision?.seq).toBe(3);
    expect(m.view?.decision?.options.map((o) => o.kind)).toEqual(['activate', 'pass', 'concede']);
    expect(await fetchPending('g1', 1, m.ctx)).toMatchObject({ seq: 3 });
  });

  it('ignores a reconnect re-serving the open request', async () => {
    const { m, f } = await started(request(3, 40, { log: ['You draw a card'] }));
    f.emit(request(3, 40, { log: ['You draw a card'] }));
    expect(m.dvr.events).toHaveLength(1);
  });

  it('answers a candidate, withdraws the decision, and walks the next substeps unseen', async () => {
    const options = [{ index: 0, kind: 'card', label: 'A', player: 0 }, { index: 1, kind: 'card', label: 'B', player: 0 }];
    const ops = [{ op: 'choose', option: 0 }, { op: 'choose', option: 1 }];
    const { m, f } = await started(request(5, 40, { kind: 'choose', options, ops }));
    await postIntent('g1', 1, { seq: 5, player: 0, choices: [1, 0] }, m.ctx);
    expect(f.sent).toEqual([{ request_id: '5', candidate_id: 1 }]);
    expect(m.view?.decision).toBeNull();
    f.emit(null);
    f.emit(request(6, 40, { kind: 'choose', options, ops: [{ op: 'choose', option: 0 }] }));
    expect(f.sent[1]).toEqual({ request_id: '6', candidate_id: 0 });
    expect(m.view?.decision).toBeNull();
    f.emit(request(7, 41));
    expect(m.view?.decision?.seq).toBe(7);
  });

  it('shows a substep the Intent does not settle as its candidates, answered by id', async () => {
    const options = [{ index: 0, kind: 'dig', label: 'A', player: 0 }, { index: 1, kind: 'dig', label: 'B', player: 0 }];
    const { m, f } = await started(request(5, 40, { kind: 'choose', options, ops: [{ op: 'choose', option: 0 }, { op: 'choose', option: 1 }] }));
    await postIntent('g1', 1, { seq: 5, player: 0, choices: [0] }, m.ctx);
    const order = request(6, 40, { kind: 'choose', options, ops: [{ op: 'list', option: 0, list: 'followup:dig_bottom' }, { op: 'list', option: 1, list: 'followup:dig_bottom' }] });
    f.emit(order);
    expect(f.sent).toHaveLength(1);
    expect(m.view?.decision).toMatchObject({ seq: 6, min: 1, max: 1 });
    await postIntent('g1', 1, { seq: 6, player: 0, choices: [1] }, m.ctx);
    expect(f.sent[1]).toEqual({ request_id: '6', candidate_id: 1 });
    f.emit(request(7, 40, { kind: 'choose', options, ops: [{ op: 'list', option: 0, list: 'followup:dig_bottom' }, { op: 'list', option: 2, list: 'followup:dig_bottom' }] }));
    expect(m.view?.decision).toMatchObject({ seq: 7, min: 1, max: 1 });
  });

  it('shows the order of a dig remainder as its own decision, and walks the order', async () => {
    const options = [{ index: 0, kind: 'dig', label: 'A', player: 0 }, { index: 1, kind: 'dig', label: 'B', player: 0 }];
    const { m, f } = await started(request(5, 40, { kind: 'choose', options, ops: [{ op: 'choose', option: 0 }, { op: 'choose', option: 1 }] }));
    await postIntent('g1', 1, { seq: 5, player: 0, choices: [0] }, m.ctx);
    const order = (id: number, position: number, ids: number[]) => {
      const r = request(id, 40, { kind: 'choose', options, ops: ids.map((option) => ({ op: 'list', option, list: 'followup:dig_bottom', position })) });
      r.decision.extensions.x_gorge_view_v1.followups = { dig_bottom: { seq: 40, player: 0, kind: 'arrange', prompt: 'Order', min: 2, max: 2, options: [{ index: 0, kind: 'bottom', label: 'X', player: 0 }, { index: 1, kind: 'bottom', label: 'Y', player: 0 }] } as never };
      return r;
    };
    f.emit(order(6, 0, [0, 1]));
    expect(m.view?.decision).toMatchObject({ seq: 6, kind: 'arrange', prompt: 'Order' });
    await postIntent('g1', 1, { seq: 6, player: 0, choices: [1, 0] }, m.ctx);
    expect(f.sent[1]).toEqual({ request_id: '6', candidate_id: 1 });
    f.emit(order(7, 1, [0]));
    expect(f.sent[2]).toEqual({ request_id: '7', candidate_id: 0 });
    expect(m.view?.decision).toBeNull();
  });

  it('shows a reload landing mid-decision as the candidates', async () => {
    const { m } = await started(request(5, 40, { kind: 'choose', substep: 2 }));
    expect(m.view?.decision).toMatchObject({ seq: 5, min: 1, max: 1 });
    expect(m.view?.decision?.options).toHaveLength(1);
  });

  it('concedes through the human action, and taps over the native route', async () => {
    const fetched = vi.fn(async (_url: string, _init?: RequestInit) => new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetched);
    const { m, f } = await started(request(5, 40));
    await postIntent('g1', 1, { seq: 5, player: 0, choices: [0] }, m.ctx);
    expect(f.sent).toEqual([]);
    expect(fetched.mock.calls[0][0]).toContain('/api/tables/g1/matches/1/intent');
    expect(JSON.parse(String(fetched.mock.calls[0][1]?.body))).toMatchObject({ seq: 40, choices: [0] });
    f.emit(request(6, 42));
    await postIntent('g1', 1, { seq: 6, player: 0, choices: [2] }, m.ctx);
    expect(f.sent).toEqual([{ request_id: '6', x_gorge_human_v1: { concede: true } }]);
  });

  it('answers a native-only request on the native route, with its native seq', async () => {
    const fetched = vi.fn(async (_url: string, _init?: RequestInit) => new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetched);
    const { m, f } = await started(request(5, 700, { nativeOnly: true }));
    expect(m.view?.decision?.seq).toBe(5);
    await postIntent('g1', 1, { seq: 5, player: 0, choices: [1] }, m.ctx);
    expect(f.sent).toEqual([]);
    expect(JSON.parse(String(fetched.mock.calls[0][1]?.body))).toMatchObject({ seq: 700, choices: [1] });
  });

  it('undoes through the human action, and says so when the rewound ask arrives', async () => {
    const { m, f } = await started(request(5, 40));
    const rewound = vi.fn();
    m.onRewind = rewound;
    await postUndo('g1', 1, m.ctx);
    expect(f.sent).toEqual([{ request_id: '5', x_gorge_human_v1: { undo: true } }]);
    f.emit(request(6, 30));
    expect(rewound).toHaveBeenCalledOnce();
    expect(m.view?.decision?.seq).toBe(6);
  });

  it('undoes on the native route when no request is open', async () => {
    const fetched = vi.fn(async (_url: string, _init?: RequestInit) => new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetched);
    const { m, f } = await started(request(5, 40));
    f.emit(null);
    await postUndo('g1', 1, m.ctx);
    expect(f.sent).toEqual([]);
    expect(fetched.mock.calls[0][0]).toContain('/api/tables/g1/matches/1/undo');
  });

  it('keeps a stale answer off the banner: the request simply closed first', async () => {
    const { m } = await started(request(5, 40), { sendError: new WireError(409, 'stale', 'stale') });
    await expect(postIntent('g1', 1, { seq: 5, player: 0, choices: [1] }, m.ctx)).rejects.toMatchObject({ status: 409 });
    expect(m.notice).toBeNull();
  });

  it('hides the log lines a native log hides, by their spelling', async () => {
    const lines = ['You have priority', 'Bot has priority', 'You are asked: priority', 'You answer priority', 'Step: main1', 'Combat ends', 'Turn 2: You', 'You play a land'];
    const { m } = await started(request(5, 40, { log: lines }));
    expect(m.dvr.events.map((e) => e.event.kind)).toEqual(['priority', 'priority', 'decision_ask', 'decision_made', 'step', 'end_combat_reset', 'sbv2', 'sbv2']);
  });

  it('reads the end of a match from the native routes once no request is open', async () => {
    vi.useFakeTimers();
    let state = 'live';
    const fetchMatches = vi.fn(async () => [{ ...info, state }]);
    const fetchView = vi.fn(async () => board({ over: true, winner: 1 }));
    vi.stubGlobal('fetch', vi.fn(async () => new Response('[]', { status: 200 })));
    const { m, f } = await started(request(5, 40), { fetchMatches, fetchView });
    f.emit(null);
    await vi.advanceTimersByTimeAsync(1000);
    expect(m.view?.over).toBe(false);
    state = 'done';
    await vi.advanceTimersByTimeAsync(3000);
    expect(m.view?.over).toBe(true);
    expect(f.urls).toHaveLength(1);
  });

  it('stops looking for the end while a request is open', async () => {
    vi.useFakeTimers();
    const fetchMatches = vi.fn(async () => [info]);
    const { f } = await started(request(5, 40), { fetchMatches });
    fetchMatches.mockClear();
    f.emit(null);
    f.emit(request(6, 41));
    await vi.advanceTimersByTimeAsync(10_000);
    expect(fetchMatches).not.toHaveBeenCalled();
  });

  it('shows the native board when it loads between decisions', async () => {
    const fetchView = vi.fn(async () => board({ turn: 4 }));
    const { m, f } = await started(null, { fetchView });
    expect(m.view?.turn).toBe(4);
    expect(f.urls).toHaveLength(1);
  });
});
