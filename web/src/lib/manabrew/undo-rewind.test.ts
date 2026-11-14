import { describe, expect, it, vi } from 'vitest';
import type { MatchInfo } from '../../protocol';
import { postUndo } from '../api';
import { ManaBrewMatch } from './source.svelte';
import { runs } from './testdata/fixture';
import { WireError } from '../wire/http';
import type { Transport } from './transport';
import type { ClientMessage, EngineMessage } from './wire';

/**
 * undo-rewind tests: an undo's restored ask must never be answered by the
 * adapter's own compat auto-pass. The compat layer passes every priority ask
 * with nothing to do (compat.ts), and rewind detection (a lower promptId, or
 * the very promptId this seat answered) is what keeps a restored ask off that
 * path. These tests pin the cases where that detection would otherwise miss:
 * a refused answer resets answeredPromptId (send.ts's catch), so the restored
 * ask — redelivered under the SAME promptId — reads as fresh and the client
 * answers away the rewind it just asked for.
 */

const seats = [{ name: 'You', deck: 'a', colour: '', human: true }, { name: 'Bot', deck: 'b', colour: '' }];
const info: MatchInfo = { table: 'g1', match: 1, seed: 1, seats, state: 'live', winner: null, events: 10, turns: 1, bot_policy: 'bot' };

const idlePrompt = (promptId: number): EngineMessage =>
  ({ kind: 'prompt', promptId, decidingPlayerId: 'p0', input: { type: 'chooseAction', actions: [] } }) as EngineMessage;
const realPrompt = (promptId: number): EngineMessage =>
  ({ kind: 'prompt', promptId, decidingPlayerId: 'p0', input: { type: 'chooseAction', actions: [{ id: 'pay-5', type: 'cast', cardId: 'c5' }] } }) as EngineMessage;

function fake(first: EngineMessage[], opts: { refuseFirst?: number } = {}) {
  const sent: ClientMessage[] = [];
  let refuse = opts.refuseFirst ?? 0;
  let push: ((m: EngineMessage) => void) | null = null;
  const transport: Transport = {
    fetchState: async () => first,
    send: async (_t, _k, _tok, msg) => {
      if (refuse > 0) {
        refuse -= 1;
        const id = msg.kind === 'response' ? msg.promptId : -1;
        throw new WireError(409, 'stalePrompt', `response names promptId ${id}`, id);
      }
      sent.push(msg);
    },
    openStream: (_url, onMessage) => {
      push = onMessage;
      return { close: () => { push = null; } };
    },
  };
  return { transport, sent, emit: (m: EngineMessage) => push?.(m) };
}

async function started(opts: { refuseFirst?: number } = {}) {
  const f = fake(runs.find((r) => r.run === 'cast')!.records[0].manabrew, opts);
  const m = new ManaBrewMatch('g1', { seat: 0, token: 'tok' }, { transport: f.transport, fetchMatches: async () => [info], compat: true });
  const res = await m.start();
  expect(res).toEqual({ ok: true });
  return { m, f };
}

describe('ManaBrewMatch undo vs compat auto-pass', () => {
  it('a refused answer does not turn the undo redelivery into a compat pass', async () => {
    const { m, f } = await started({ refuseFirst: 1 });
    // The seat passes an idle ask; the send is refused (the decision moved
    // under it — an undo rewind landing mid-flight), so answeredPromptId
    // resets while lastPromptId stays at the refused ask.
    f.emit(idlePrompt(9006));
    await Promise.resolve();
    expect(f.sent).toEqual([]); // the pass was refused, not recorded
    let rewound = 0;
    m.onRewind = () => rewound++;
    const fetchMock = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetchMock);
    try {
      // The undo goes down the native path (nothing is bound) and rewinds
      // the game back to the very ask this seat was refused on.
      await postUndo('g1', 1, m.ctx);
      f.emit(idlePrompt(9006));
      await Promise.resolve();
      expect(rewound).toBe(1);
      expect(f.sent).toEqual([]); // never a compat pass for the restored ask
      expect(m.openPrompt?.promptId).toBe(9006);
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it('the walk-back shows every restored ask, however far the undos go', async () => {
    const { m, f } = await started();
    f.emit(idlePrompt(9002)); // a pass the seat will undo twice over
    await Promise.resolve();
    f.emit(realPrompt(9008)); // the ask the undo is clicked at
    expect(m.openPrompt?.promptId).toBe(9008);
    let rewound = 0;
    m.onRewind = () => rewound++;
    await postUndo('g1', 1, m.ctx);
    const undoSend = f.sent.at(-1);
    expect(undoSend?.kind === 'response' && undoSend.action.output).toMatchObject({ type: 'restoreSnapshot' });
    const sentAtUndo = f.sent.length;
    // The server re-poses the still-open ask after the rewind frame: shown
    // again, never auto-passed, and no second rewind event for what the
    // panel is already looking at.
    f.emit(idlePrompt(9008));
    await Promise.resolve();
    expect(rewound).toBe(0);
    expect(m.openPrompt?.promptId).toBe(9008);
    // The rewind itself restores the older ask the undo walked back to.
    f.emit(idlePrompt(9002));
    await Promise.resolve();
    expect(rewound).toBe(1);
    expect(f.sent.slice(sentAtUndo)).toEqual([]); // no compat pass for either restored ask
    expect(m.openPrompt?.promptId).toBe(9002);
  });
});
