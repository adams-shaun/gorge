import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Decision, Option } from '../protocol';
import { SeatPanelState } from './seatpanel.svelte';

// The demo freeze of 2026-09-24 (table g2, Ingot Chewer evoked): the client
// auto-ordered a trigger_order ask (seq 851), the /pending poll then handed
// the panel the next ask (the target, seq 854), and the board SeatPanel --
// mounted for that required prompt -- re-adopted the seat view's STALE
// decision, the already-answered 851. adopt() only refused the exact posted
// or pending seq, so 851 replaced 854 and the panel sat on an answered ask
// with its submit disabled. An older seq is never the seat's current ask
// within one seq space; only begin() (a rewind or a match boundary) opens a
// new space where a lower seq is legitimate again.

const { postIntentMock, fetchPendingMock } = vi.hoisted(() => ({ postIntentMock: vi.fn(), fetchPendingMock: vi.fn() }));
vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  postIntent: postIntentMock,
  fetchPending: fetchPendingMock,
}));

const ctx = { seat: 0, token: 'tok' };

const trigger = (i: number, label: string): Option => ({ index: i, kind: 'trigger', label, obj: 93, player: 0 });

const order = (seq: number): Decision => ({
  seq,
  player: 0,
  kind: 'trigger_order',
  prompt: 'Order your simultaneous triggered abilities',
  min: 2,
  max: 2,
  options: [
    trigger(0, 'Ingot Chewer: When CARDNAME enters, destroy target artifact.'),
    trigger(1, 'Ingot Chewer: sacrifice it (evoked)'),
  ],
});

const target = (seq: number): Decision => ({
  seq,
  player: 0,
  kind: 'target',
  prompt: 'Choose a target for Ingot Chewer',
  min: 1,
  max: 1,
  options: [{ index: 0, kind: 'target', label: 'Chrome Mox (You)', obj: 71, player: 0 }],
});

function seat(): SeatPanelState {
  const p = new SeatPanelState('g2', 1, ctx, null, null);
  p.stops = { yours: new Set(), opponents: new Set() };
  p.settings = { ...p.settings, pacing: { stepMs: 0, resolveMs: 0 }, autoPass: false, autoOrderAllTriggers: true };
  return p;
}

async function settle(predicate: () => boolean, maxTicks = 200): Promise<void> {
  for (let i = 0; i < maxTicks; i++) {
    if (predicate()) return;
    await Promise.resolve();
  }
  throw new Error(`settle: condition still false after ${maxTicks} microtask ticks`);
}

beforeEach(() => {
  postIntentMock.mockReset();
  fetchPendingMock.mockReset();
  postIntentMock.mockResolvedValue(undefined);
});

describe('adopt refuses a decision older than the seat has already seen', () => {
  it('keeps the target ask when a stale view re-offers the auto-ordered trigger_order', async () => {
    const p = seat();
    p.adoptView(order(851));
    await settle(() => postIntentMock.mock.calls.length === 1 && !p.busy);
    expect(postIntentMock.mock.calls[0][2]).toMatchObject({ seq: 851, choices: [0, 1] });

    fetchPendingMock.mockResolvedValue(target(854));
    await p.refreshPending();
    expect(p.pending?.seq).toBe(854);

    p.adoptView(order(851));
    expect(p.pending?.seq).toBe(854);
    expect(p.pending?.kind).toBe('target');
  });

  it('accepts a lower seq again after begin() opens a new seq space (rewind)', () => {
    const p = seat();
    p.settings = { ...p.settings, autoOrderAllTriggers: false };
    p.adoptView(target(854));
    p.rewind();
    p.adoptView(target(700));
    expect(p.pending?.seq).toBe(700);
  });
});
