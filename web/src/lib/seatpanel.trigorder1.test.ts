import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Decision, Option } from '../protocol';
import { SeatPanelState, triggerOrderPermutation } from './seatpanel.svelte';

// fb-trigorder1: casual should not make the player hand-order simultaneous
// triggers. The prio6 identical-trigger auto-order exists but only fires on
// equal labels; this file pins the BROADER path — settings.autoOrderAllTriggers
// auto-answers ANY trigger_order ask satisfying the same shape contract
// (min == max == N over options, N >= 2), submitting the OFFERED order, with a
// note distinct from the identical path's. The fixtures mirror the decision
// dumped live from the engine (rules/trigger_queue.go's askTriggerOrder) and
// the exact ask from the player report: two DIFFERENT cards whose labels
// differ (Razorkin Needlehead vs Fate Unraveler).
//
// Guards pinned here: the setting off keeps the ask manual; machinePaused
// (the undo pause / runaway brake) silences it; the autoOrderedSeq guard
// still prevents an infinite retry on a server rejection.

const { postIntentMock, fetchPendingMock } = vi.hoisted(() => ({ postIntentMock: vi.fn(), fetchPendingMock: vi.fn() }));
vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  postIntent: postIntentMock,
  fetchPending: fetchPendingMock,
}));

const ctx = { seat: 0, token: 'tok' };

const trigger = (i: number, label: string, obj: number): Option =>
  ({ index: i, kind: 'trigger', label, obj, player: 0 });

/** report is the EXACT trigger_order decision captured in the player's feedback report (two different cards). */
const report = (seq: number): Decision => ({
  seq,
  player: 0,
  kind: 'trigger_order',
  prompt: 'Order your simultaneous triggered abilities: the one you choose first is put on the stack first, and so resolves last',
  min: 2,
  max: 2,
  options: [
    trigger(0, 'Razorkin Needlehead: Whenever an opponent draws a card, CARDNAME deals 1 damage to them.', 60),
    trigger(1, 'Fate Unraveler: Whenever an opponent draws a card, CARDNAME deals 1 damage to that player.', 26),
  ],
});

function immediateSeat(table = 'to1'): SeatPanelState {
  const p = new SeatPanelState(table, 1, ctx, null, null);
  p.stops = { yours: new Set(), opponents: new Set() };
  p.settings = { ...p.settings, pacing: { stepMs: 0, resolveMs: 0 }, autoPass: false };
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

describe('triggerOrderPermutation — the shared shape contract', () => {
  it('holds for the reported two-card ask', () => {
    expect(triggerOrderPermutation(report(1))).toBe(true);
  });

  it('fails for another kind, a partial permutation, or a single-option ask', () => {
    expect(triggerOrderPermutation({ ...report(2), kind: 'modes' })).toBe(false);
    const partial = report(3);
    (partial as { min: number }).min = 1;
    expect(triggerOrderPermutation(partial)).toBe(false);
    const single = report(4);
    (single as { options: Option[] }).options = [single.options[0]];
    (single as { min: number }).min = 1;
    (single as { max: number }).max = 1;
    expect(triggerOrderPermutation(single)).toBe(false);
  });
});

describe('auto-ordering ALL trigger_order asks (autoOrderAllTriggers, casual on by default)', () => {
  it('auto-submits the offered order for the reported two-DIFFERENT-card ask, with the distinct note', async () => {
    const p = immediateSeat();
    p.adoptView(report(1));
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock.mock.calls[0][2].choices).toEqual([0, 1]); // the OFFERED order
    expect(p.pending).toBeNull();
    const note = p.autoLog.find((n) => n.text.includes('triggers automatically'));
    expect(note?.text).toBe('Ordered 2 triggers automatically'); // NOT the identical path's wording
  });

  it('an identical pair still answers through the identical path (its pinned wording is unchanged)', async () => {
    const p = immediateSeat();
    const identical: Decision = {
      seq: 1, player: 0, kind: 'trigger_order', prompt: 'p', min: 2, max: 2,
      options: [
        trigger(0, 'Blood Artist: Whenever a creature dies, each opponent loses 1 life', 60),
        trigger(1, 'Blood Artist: Whenever a creature dies, each opponent loses 1 life', 61),
      ],
    };
    p.adoptView(identical);
    await settle(() => p.postedSeq === 1);
    const note = p.autoLog.find((n) => n.text.includes('triggers automatically'));
    expect(note?.text).toBe('Ordered 2 identical triggers automatically');
  });

  it('the setting off keeps a non-identical ask manual (even with autoOrderIdenticalTriggers still on)', () => {
    const p = immediateSeat();
    p.editSettings({ autoOrderAllTriggers: false });
    p.adoptView(report(1));
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.pending?.seq).toBe(1);
  });

  it('machinePaused (the undo pause / runaway brake) silences the broader path', () => {
    const p = immediateSeat();
    p.machinePaused = true;
    p.adoptView(report(1));
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.pending?.seq).toBe(1);
  });

  it('a rejected auto-order is never retried forever (the autoOrderedSeq guard, shared with prio6)', async () => {
    postIntentMock.mockRejectedValueOnce(new Error('stale seq'));
    fetchPendingMock.mockResolvedValue(null);
    const p = immediateSeat();
    p.adoptView(report(1));
    await settle(() => p.error === 'stale seq');
    await settle(() => p.pending === null);
    // The same decision comes back: the guard refuses a seq already auto-ordered.
    p.adoptView(report(1));
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.pending?.seq).toBe(1);
  });

  it('a live one-shot run is cancelled by the broader auto-order (a non-priority decision ends a run)', async () => {
    const p = immediateSeat();
    const armed = { turn: 2, stack: [{ id: 9, controller: 1, kind: 'spell', name: 'Lightning Bolt', targets: [] }], players: [], active: 0, step: 'draw' } as never;
    const quiet = (seq: number): Decision =>
      ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [
        { index: 0, kind: 'pass', label: 'Pass priority', player: 0 },
        { index: 1, kind: 'concede', label: 'Concede', player: 0 },
      ] });
    p.adoptView(quiet(1));
    p.startResolveAll(armed);
    p.considerAuto(armed);
    await settle(() => p.postedSeq === 1);
    expect(p.oneShot).toBe('resolve-all');
    // The trigger_order arrives while the run is live: the run must end and
    // the broader auto-order must still answer.
    p.adoptView(report(2));
    await settle(() => p.postedSeq === 2);
    expect(p.oneShot).toBe('none');
    expect(postIntentMock.mock.calls[1][2].choices).toEqual([0, 1]);
  });
});
