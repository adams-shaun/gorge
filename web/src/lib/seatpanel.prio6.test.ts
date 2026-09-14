import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Decision, Option, View } from '../protocol';
import { SeatPanelState, autoNoteText, identicalTriggerOrder } from './seatpanel.svelte';
import { stackYieldKey } from './yields';

// Prio6: always-yield per ability, Resolve All, auto-ordered identical
// triggers. The pieces and where each is tested:
//
//  - decide()'s yield/baseline rules: lib/autopilot.test.ts (pure rules).
//  - the yield STORE's persistence: lib/yields.test.ts.
//  - HERE: the SeatPanelState loop around them — the Resolve All one-shot
//    run (arm → pass → expire/stop edges), the identical-trigger_order
//    auto-submit (against the REAL measured wire shape — the fixtures below
//    mirror the decision dumped live from the engine: min == max == N over
//    options `kind: "trigger"` with label "<source name>: <TriggerDescription>"),
//    and the yield write paths (addYield applies to the window pending
//    RIGHT NOW; clearYields empties; the set survives a new SeatPanelState
//    of the same table).
//
// The yield-store tests use ONE DISTINCT TABLE ID EACH, because the store's
// in-memory layer is module-level and shared by every SeatPanelState in
// this file — a fresh table id is what makes each test's layer claim
// independently checkable.

const { postIntentMock, fetchPendingMock } = vi.hoisted(() => ({ postIntentMock: vi.fn(), fetchPendingMock: vi.fn() }));
vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  postIntent: postIntentMock,
  fetchPending: fetchPendingMock,
}));

const ctx = { seat: 0, token: 'tok' };

const pass = (i: number): Option => ({ index: i, kind: 'pass', label: 'Pass priority', player: 0 });
const concede = (i: number): Option => ({ index: i, kind: 'concede', label: 'Concede', player: 0 });
const cast = (i: number): Option => ({ index: i, kind: 'cast', label: `Cast spell ${i}`, player: 0 });
const trigger = (i: number, label: string, obj: number): Option =>
  ({ index: i, kind: 'trigger', label, obj, player: 0 });

/** quiet is a priority window with nothing to do: pass and concede only. */
const quiet = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [pass(0), concede(1)] });

/** live is a respondable window (a cast is offered): the stack rules can stop for it. */
const live = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [cast(0), pass(1), concede(2)] });

/** entry is one wire StackView for an ability: name is the SOURCE's name, text the ability's text. */
const entry = (
  id: number,
  controller: number,
  name: string,
  text: string,
  targets: View['stack'][number]['targets'] = [],
): View['stack'][number] =>
  ({ id, controller, kind: 'trigger', name, text, targets }) as unknown as View['stack'][number];

/** stackView is one object on the stack, shaped like the wire's StackView. */
const stackView = (id: number, name: string, controller = 1): View['stack'][number] =>
  ({ id, controller, kind: 'spell', name, targets: [] }) as unknown as View['stack'][number];

const view = (step = 'draw', active = 0, turn = 2, stack: View['stack'] = []): View =>
  ({ active, step, turn, stack, players: [] }) as unknown as View;

/**
 * immediateSeat is casual with zero pacing (the synchronous pass path), no
 * step stops, and — for the Resolve All runs — autoPass OFF, so that after
 * a run ends nothing else answers the seat's windows and the test sees the
 * run's own edges exactly. (The run itself forces autoPass on through
 * runSettings; the press is the consent.)
 */
function immediateSeat(table = 't1', storage: Storage | null = null): SeatPanelState {
  const p = new SeatPanelState(table, 1, ctx, storage);
  p.stops = { yours: new Set(), opponents: new Set() };
  p.settings = { ...p.settings, pacing: { stepMs: 0, resolveMs: 0 }, autoPass: false };
  return p;
}

/** triggerOrder is the REAL measured shape of a trigger_order decision (dumped from the engine, see the report). */
const triggerOrder = (seq: number, labels: [string, string]): Decision => ({
  seq,
  player: 0,
  kind: 'trigger_order',
  prompt: 'Order your simultaneous triggered abilities: the one you choose first is put on the stack first, and so resolves last',
  min: 2,
  max: 2,
  options: labels.map((label, i) => trigger(i, label, 81 + i)),
});

const ARTIST_LABEL = 'Blood Artist: Whenever a creature dies, each opponent loses 1 life';
const WARDEN_LABEL = 'Soul Warden: Whenever a creature enters, its controller gains 1 life';
const IDENTICAL: [string, string] = [ARTIST_LABEL, ARTIST_LABEL];
const MIXED: [string, string] = [ARTIST_LABEL, WARDEN_LABEL];

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
afterEach(() => {
  vi.useRealTimers();
});

describe('identicalTriggerOrder — the identical check, grounded in the measured wire shape', () => {
  it('is true for the measured identical pair: min==max==N, every option the same label', () => {
    expect(identicalTriggerOrder(triggerOrder(7, IDENTICAL))).toBe(true);
  });

  it('is false when the options differ (mixed pair), for another kind, or for a shape that is not a full permutation', () => {
    expect(identicalTriggerOrder(triggerOrder(8, MIXED))).toBe(false);
    expect(identicalTriggerOrder({ ...triggerOrder(9, IDENTICAL), kind: 'modes' })).toBe(false);
    const partial = triggerOrder(10, IDENTICAL);
    (partial as { min: number }).min = 1;
    expect(identicalTriggerOrder(partial)).toBe(false);
  });

  it('needs at least two options — a one-trigger ask is never posed', () => {
    const single: Decision = {
      seq: 11, player: 0, kind: 'trigger_order', prompt: 'p', min: 1, max: 1,
      options: [trigger(0, ARTIST_LABEL, 81)],
    };
    expect(identicalTriggerOrder(single)).toBe(false);
  });
});

describe('auto-ordered identical triggers', () => {
  it('auto-submits the default (offered) order and logs the note, when the setting is on', async () => {
    const p = immediateSeat();
    p.adoptView(triggerOrder(1, IDENTICAL));
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock.mock.calls[0][2].choices).toEqual([0, 1]);
    expect(p.pending).toBeNull(); // answered, options hidden
    const note = p.autoLog.find((n) => n.text.includes('identical triggers'));
    expect(note?.text).toBe('Ordered 2 identical triggers automatically');
  });

  it('a mixed pair stays manual as today', () => {
    const p = immediateSeat();
    p.adoptView(triggerOrder(1, MIXED));
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.pending?.seq).toBe(1);
  });

  it('the setting off stays manual', () => {
    const p = immediateSeat();
    p.editSettings({ autoOrderIdenticalTriggers: false });
    p.adoptView(triggerOrder(1, IDENTICAL));
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.pending?.seq).toBe(1);
  });

  it('a rejected auto-order is never retried forever (the autoOrderedSeq guard)', async () => {
    postIntentMock.mockRejectedValueOnce(new Error('stale seq'));
    fetchPendingMock.mockResolvedValue(null);
    const p = immediateSeat();
    p.adoptView(triggerOrder(1, IDENTICAL));
    await settle(() => p.error === 'stale seq'); // the post failed and surfaced
    await settle(() => p.pending === null); // the recovery refetch adopted "nothing pending"
    // The same decision comes back: the guard refuses a seq already auto-ordered.
    p.adoptView(triggerOrder(1, IDENTICAL));
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.pending?.seq).toBe(1);
  });

  it('a live one-shot run is cancelled by the auto-order (a non-priority decision ends a run)', async () => {
    const p = immediateSeat();
    const armed = view('draw', 0, 2, [stackView(9, 'Lightning Bolt')]);
    p.adoptView(quiet(1));
    p.startResolveAll(armed);
    p.considerAuto(armed);
    await settle(() => p.postedSeq === 1);
    expect(p.oneShot).toBe('resolve-all'); // still running on the next window
    // An identical trigger_order arrives for the seat: the run must not
    // survive it, and the answer still goes out under the setting.
    p.adoptView(triggerOrder(2, IDENTICAL));
    await settle(() => p.postedSeq === 2);
    expect(p.oneShot).toBe('none');
    expect(postIntentMock.mock.calls[1][2].choices).toEqual([0, 1]);
  });
});

describe('Resolve All', () => {
  it('passes while the stack is non-empty and ends when the stack is empty', async () => {
    const p = immediateSeat();
    p.skipEmpty = false;
    // Arm on a stack holding one opponent spell (id 9): the run plays through it.
    const withStack = view('draw', 0, 2, [stackView(9, 'Lightning Bolt')]);
    p.adoptView(quiet(1));
    p.startResolveAll(withStack);
    expect(p.oneShot).toBe('resolve-all');
    expect(p.playMode).toBe('resolve-all');
    expect(autoNoteText(p.note)).toContain('Resolve All');
    p.considerAuto(withStack);
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.runPassed).toBe(1);
    expect(autoNoteText(p.note)).toBe('Resolve All passed 1 priority window.');

    // Still resolving: another window, same stack.
    const stillResolving = view('draw', 0, 2, [stackView(9, 'Lightning Bolt')]);
    p.adoptView(quiet(2));
    p.considerAuto(stillResolving);
    await settle(() => p.postedSeq === 2);
    expect(p.runPassed).toBe(2);

    // The stack has emptied: the run ends here, even though the seat still
    // holds priority — the decision is left unanswered for the player.
    const empty = view('draw', 0, 2, []);
    p.adoptView(quiet(3));
    p.considerAuto(empty);
    expect(p.oneShot).toBe('none');
    expect(postIntentMock).toHaveBeenCalledTimes(2);
    expect(postIntentMock.mock.calls[1][2].seq).toBe(2); // the run's second pass; nothing answered seq 3
  });

  it('plays through arm-time opponent objects (casual would stop for them) but stops on a NEW opponent spell', async () => {
    const p = immediateSeat();
    p.skipEmpty = false;
    // Arm with an OPPONENT trigger targeting me on the stack; casual's
    // opponentTrigger rule would stop here — the baseline is exactly what
    // the run ignores.
    const artist = entry(9, 1, 'Blood Artist', 'Whenever a creature dies, each opponent loses 1 life', [
      { player: 0, is_player: true },
    ]);
    const armed = view('draw', 0, 2, [artist]);
    p.adoptView(live(1));
    p.startResolveAll(armed);
    p.considerAuto(armed);
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);

    // A NEW opponent object the settings say to stop for: the run stops,
    // the decision is not answered, and the stopped note names the reason.
    const bolt = stackView(10, 'Lightning Bolt');
    const withNew = view('draw', 0, 2, [artist, bolt]);
    p.adoptView(live(2));
    p.considerAuto(withNew);
    expect(p.oneShot).toBe('none');
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock.mock.calls[0][2].seq).toBe(1); // the arm-time window's pass; seq 2 is unanswered
    expect(autoNoteText(p.note)).toContain('an opponent');
  });

  it('a new opponent object the settings say never to stop for does not stop the run', async () => {
    const p = immediateSeat();
    p.skipEmpty = false;
    p.settings = { ...p.settings, opponentSpell: 'never', opponentTrigger: 'never', opponentAbility: 'never' };
    const armed = view('draw', 0, 2, [stackView(9, 'Lightning Bolt')]);
    p.adoptView(quiet(1));
    p.startResolveAll(armed);
    p.considerAuto(armed);
    await settle(() => p.postedSeq === 1);
    const withNew = view('draw', 0, 2, [stackView(9, 'Lightning Bolt'), stackView(10, 'Shock')]);
    p.adoptView(quiet(2));
    p.considerAuto(withNew);
    await settle(() => p.postedSeq === 2);
    expect(p.oneShot).toBe('resolve-all'); // still running
    expect(p.runPassed).toBe(2);
  });

  it('any non-priority decision ends the run (the not-priority stop verdict)', async () => {
    const p = immediateSeat();
    const armed = view('draw', 0, 2, [stackView(9, 'Lightning Bolt')]);
    p.adoptView(quiet(1));
    p.startResolveAll(armed);
    p.considerAuto(armed);
    await settle(() => p.postedSeq === 1);
    const target: Decision = { seq: 2, player: 0, kind: 'target', prompt: 'Choose a target', min: 1, max: 1, options: [] };
    p.adoptView(target);
    p.considerAuto(armed);
    expect(p.oneShot).toBe('none');
    expect(autoNoteText(p.note)).toContain('this decision needs you');
  });

  it('Escape cancels the run, and the cap shares AUTO_PASS_CAP with the other machine paths', () => {
    const p = immediateSeat();
    const armed = view('draw', 0, 2, [stackView(9, 'Lightning Bolt')]);
    p.adoptView(quiet(1));
    p.startResolveAll(armed);
    p.onKeydown('Escape');
    expect(p.oneShot).toBe('none');
    // The cap is the shared runaway bound: forty unbroken machine passes.
    p.startResolveAll(armed);
    p.autoRun = 40; // forty passes already counted by this run
    p.considerAuto(armed);
    expect(p.oneShot).toBe('none');
  });

  it('cannot start on an empty stack (the button never offers it, so the state refuses it too)', () => {
    const p = immediateSeat();
    p.startResolveAll(view('draw', 0, 2, []));
    expect(p.oneShot).toBe('none');
  });
});

describe('always-yield write paths', () => {
  const artistKey = stackYieldKey({
    controller: 1, name: 'Blood Artist', text: 'Whenever a creature dies, each opponent loses 1 life',
  });
  /** targetingArtist stops casual (opponentTrigger targets-me, I can respond). */
  const targetingArtist = entry(9, 1, 'Blood Artist', 'Whenever a creature dies, each opponent loses 1 life', [
    { player: 0, is_player: true },
  ]);

  it('addYield applies to the window pending RIGHT NOW: the stop becomes a pass', async () => {
    const p = immediateSeat('yt1');
    // The machine must be RUNNING for a yield to matter: it removes the
    // opponent-object RULE (decide), it does not switch auto on.
    p.settings = { ...p.settings, autoPass: true };
    const v = view('draw', 0, 2, [targetingArtist]);
    p.adoptView(live(1));
    // Casual stops: an opponent trigger that targets me, and I can respond.
    p.considerAuto(v);
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(autoNoteText(p.note)).toContain("an opponent's object");
    // Yield it: the SAME pending window is re-derived immediately.
    p.addYield(artistKey);
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock.mock.calls[0][2].choices).toEqual([1]); // live's pass index
  });

  it('the yield set is game-scoped per table: it survives a new SeatPanelState and persists to storage', () => {
    const storage = { getItem: () => null, setItem: vi.fn() } as unknown as Storage;
    const p = immediateSeat('yt2', storage);
    p.addYield(artistKey);
    expect([...p.yields]).toEqual([artistKey]);
    expect(storage.setItem).toHaveBeenCalledWith('gorge.yields.yt2', JSON.stringify([artistKey]));
    // A new panel for the same table (a new match) inherits the yield.
    const next = immediateSeat('yt2', storage);
    expect([...next.yields]).toEqual([artistKey]);
  });

  it('clearYields empties the set and persists the empty set', () => {
    const storage = { getItem: () => null, setItem: vi.fn() } as unknown as Storage;
    const p = immediateSeat('yt3', storage);
    p.addYield(artistKey);
    p.clearYields();
    expect(p.yields.size).toBe(0);
    expect(storage.setItem).toHaveBeenLastCalledWith('gorge.yields.yt3', '[]');
    // A new panel starts clean.
    expect(immediateSeat('yt3', storage).yields.size).toBe(0);
  });
});
