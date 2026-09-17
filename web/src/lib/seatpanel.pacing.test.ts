import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Decision, Option, View } from '../protocol';
import { SeatPanelState, autoNoteText } from './seatpanel.svelte';
import { defaultSettings } from './playsettings';
import { AUTO_LOG_CAP, autoPassLogText, stepLabel } from './autolog';

// Prio5: the paced, logged auto-pass. The loop is seatpanel.svelte.ts's
// considerAuto/dispatchPass/firePass; what these tests hold down is the WAIT
// (stepMs with an empty stack, resolveMs with a resolving object), its
// cancellation edges (decision change, Escape, a hand click, the panel's
// destruction, the machine paths dying under it), the synchronous 0 ms path,
// and the client-local log notes (settings.logAutoPasses).
const { postIntentMock, fetchPendingMock } = vi.hoisted(() => ({ postIntentMock: vi.fn(), fetchPendingMock: vi.fn() }));
vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  postIntent: postIntentMock,
  fetchPending: fetchPendingMock,
}));

const ctx = { seat: 0, token: 'tok' };

const pass = (i: number): Option => ({ index: i, kind: 'pass', label: 'Pass priority', player: 0 });
const concede = (i: number): Option => ({ index: i, kind: 'concede', label: 'Concede', player: 0 });
const activate = (i: number): Option => ({ index: i, kind: 'activate', label: `Tap ${i}`, player: 0 });
const land = (i: number): Option => ({ index: i, kind: 'play_land', label: `Play land ${i}`, player: 0 });
const cast = (i: number): Option => ({ index: i, kind: 'cast', label: `Cast spell ${i}`, player: 0 });

/** quiet is a priority window with nothing to do: pass and concede only. */
const quiet = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [pass(0), concede(1)] });

/** manaLand is a window whose only real action is a land drop — the empty-window floor's shape. */
const manaLand = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [activate(0), land(1), pass(2), concede(3)] });

/** live is a respondable priority window: the safety rules can stop for its cast option. */
const live = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [cast(0), pass(1), concede(2)] });

/** stackView is one object on the stack, shaped like the wire's StackView. */
const stackView = (name: string, controller = 1): View['stack'][number] =>
  ({ id: 9, controller, kind: 'spell', name, targets: [] }) as unknown as View['stack'][number];

const view = (step = 'draw', active = 0, turn = 2, stack: View['stack'] = []): View =>
  ({ active, step, turn, stack, players: [] }) as unknown as View;

/** pacedSeat is casual with the stops off and the given pacing; auto on. */
function pacedSeat(pacing: { stepMs: number; resolveMs: number }, logAutoPasses = true): SeatPanelState {
  const p = new SeatPanelState('t1', 1, ctx, null);
  p.stops = { yours: new Set(), opponents: new Set() };
  p.settings = { ...p.settings, pacing: { ...pacing }, logAutoPasses };
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
afterEach(() => {
  vi.useRealTimers();
});

describe('the wait — stepMs on a bare step, resolveMs under a resolving object', () => {
  it('an empty stack waits stepMs (casual: 200) before the pass posts', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    p.adoptView(quiet(1));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled(); // nothing synchronous
    await vi.advanceTimersByTimeAsync(199);
    expect(postIntentMock).not.toHaveBeenCalled(); // still inside the beat
    await vi.advanceTimersByTimeAsync(1);
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock.mock.calls[0][2].choices).toEqual([0]);
    expect(p.autoPassed).toBe(1); // counted at POST time, not at schedule time
    expect(autoNoteText(p.note)).toBe('Auto passed 1 priority window.');
  });

  it('a non-empty stack waits resolveMs (casual: 400), not stepMs', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    const v = view('main1', 0, 2, [stackView('Lightning Bolt')]);
    p.adoptView(quiet(1));
    p.considerAuto(v);
    expect(postIntentMock).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(200);
    expect(postIntentMock).not.toHaveBeenCalled(); // stepMs would have fired here
    await vi.advanceTimersByTimeAsync(200);
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
  });

  it('0/0 posts synchronously — the pre-pacing path the other suites drive', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 0, resolveMs: 0 });
    p.adoptView(quiet(1));
    p.considerAuto(view());
    await settle(() => p.postedSeq === 1); // microtasks only; no timer is ever scheduled
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe('the wait is cancellable', () => {
  it('a decision change during the wait cancels: no post for the stale seq, the new window is paced on its own', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    p.adoptView(quiet(1));
    p.considerAuto(view());
    p.adoptView(quiet(2)); // the game moved on; the old pass is abandoned
    await vi.advanceTimersByTimeAsync(10_000);
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.postedSeq).toBeNull();

    // and the new decision is answered under the same rules, after ITS beat
    p.considerAuto(view());
    await vi.advanceTimersByTimeAsync(200);
    await settle(() => p.postedSeq === 2);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock.mock.calls[0][2].seq).toBe(2);
    // the abandoned pass was never "answered": no loop guard trips on seq 1
    expect(p.machinePaused).toBe(false);
  });

  it('a view change during the wait (step or stack moved) re-derives instead of posting stale', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    p.adoptView(quiet(1));
    p.considerAuto(view('draw', 0, 2, []));
    // the stack grew: the window is now under a resolving object, so the
    // re-derived pass waits resolveMs and the note names the object
    p.considerAuto(view('draw', 0, 2, [stackView('Lightning Bolt')]));
    await vi.advanceTimersByTimeAsync(200);
    expect(postIntentMock).not.toHaveBeenCalled(); // the old stepMs beat died
    await vi.advanceTimersByTimeAsync(200);
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.autoLog.map((n) => n.text)).toEqual(['Auto-passed: Lightning Bolt resolving']);
  });

  it('re-derives at fire: even an in-place view mutation cannot authorize the scheduled pass (r3)', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    const initial = view('main1', 0, 2, [{ ...stackView('Opponent trigger'), kind: 'trigger' }]);
    p.adoptView(live(1));
    p.considerAuto(initial); // no target: Casual may pass this trigger
    await vi.advanceTimersByTimeAsync(200);

    // Production projections replace the whole View and restart pacing. This
    // deliberately mutates the same object to hold the fire-time defence too:
    // even if a caller breaks that projection contract, the candidate verdict
    // is not trusted when its timer reaches the original deadline.
    initial.stack[0].targets = [{ player: 0, is_player: true }];
    p.considerAuto(initial);
    await vi.advanceTimersByTimeAsync(200);

    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.postedSeq).toBeNull();
    expect(autoNoteText(p.note)).toContain("opponent's object is on the stack");
    expect(p.autoLog).toEqual([]);
  });

  it('re-derives at fire when a targeted permanent changes controller to this seat (r3)', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    const trigger = { ...stackView('Opponent trigger'), kind: 'trigger', targets: [{ obj: 42, player: 0, is_player: false }] };
    const initial = view('main1', 0, 2, [trigger]);
    initial.players = [{ seat: 1, battlefield: [{ id: 42, controller: 1 }] }] as unknown as View['players'];
    p.adoptView(live(1));
    p.considerAuto(initial); // target is not controlled by this seat yet
    await vi.advanceTimersByTimeAsync(200);

    const controlledByMe = view('main1', 0, 2, [trigger]);
    controlledByMe.players = [{ seat: 0, battlefield: [{ id: 42, controller: 0 }] }] as unknown as View['players'];
    p.considerAuto(controlledByMe); // every new view cancels before reclassification
    await vi.advanceTimersByTimeAsync(200);

    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.postedSeq).toBeNull();
    expect(autoNoteText(p.note)).toContain("opponent's object is on the stack");
  });

  it('an unrelated view change cancels the old deadline and starts a full fresh wait (r3)', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    const trigger = { ...stackView('Harmless trigger'), kind: 'trigger' };
    const initial = view('main1', 0, 2, [trigger]);
    initial.players = [{ seat: 1, life: 20 }] as unknown as View['players'];
    p.adoptView(live(1));
    p.considerAuto(initial);
    await vi.advanceTimersByTimeAsync(200);

    const lifeChanged = view('main1', 0, 2, [trigger]);
    lifeChanged.players = [{ seat: 1, life: 19 }] as unknown as View['players'];
    p.considerAuto(lifeChanged);
    await vi.advanceTimersByTimeAsync(200); // the OLD 400 ms deadline
    expect(postIntentMock).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(199);
    expect(postIntentMock).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1); // a full resolveMs after the update
    await settle(() => p.postedSeq === 1);

    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock.mock.calls[0][2].choices).toEqual([1]);
    expect(p.autoLog.map((n) => n.text)).toEqual(['Auto-passed: Harmless trigger resolving']);
  });

  it('a steady stream of new views keeps cancelling the pass until the view settles', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    p.adoptView(quiet(1));
    p.considerAuto(view('draw', 0, 2));

    for (let life = 19; life >= 16; life--) {
      await vi.advanceTimersByTimeAsync(150);
      const update = view('draw', 0, 2);
      update.players = [{ seat: 1, life }] as unknown as View['players'];
      p.considerAuto(update);
      expect(postIntentMock).not.toHaveBeenCalled();
    }

    await vi.advanceTimersByTimeAsync(199);
    expect(postIntentMock).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
  });

  it("a settings change during the wait cancels it and the new 'always stop' trigger rule never posts (r3)", async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    const v = view('main1', 0, 2, [{ ...stackView('Opponent trigger'), kind: 'trigger' }]);
    p.adoptView(live(1));
    p.considerAuto(v);
    await vi.advanceTimersByTimeAsync(200);

    p.editSettings({ opponentTrigger: 'always' });
    p.considerAuto(v); // the component effect reclassifies under the edit
    await vi.advanceTimersByTimeAsync(10_000);

    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.postedSeq).toBeNull();
    expect(autoNoteText(p.note)).toContain("opponent's object is on the stack");
  });

  it('the top stack object REPLACED at the same depth cancels and re-paces: no post at the old deadline, the new spell logged (r2)', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    p.adoptView(quiet(1));
    p.considerAuto(view('main1', 0, 2, [stackView('Lightning Bolt')]));
    await vi.advanceTimersByTimeAsync(200);
    expect(postIntentMock).not.toHaveBeenCalled();
    // The top object resolves and the object beneath it is revealed at the
    // SAME depth: same turn, same step, same stack length — but a different
    // resolving object. The old wait must die (never post at its original
    // deadline, never log the old spell); the pass is re-derived against
    // the new view and paces again from scratch.
    p.considerAuto(view('main1', 0, 2, [{ ...stackView('Giant Growth'), id: 10 }]));
    await vi.advanceTimersByTimeAsync(200); // the ORIGINAL 400 ms deadline
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.autoLog).toEqual([]);
    await vi.advanceTimersByTimeAsync(200); // the NEW full beat
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.autoPassed).toBe(1); // counted once, for the pass that actually posted
    expect(p.autoLog.map((n) => n.text)).toEqual(['Auto-passed: Giant Growth resolving']);
  });

  it('clicking End Turn during an automatic wait re-classifies the pass: runPassed and the run register, not autoPassed (r2)', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    p.adoptView(quiet(1));
    p.considerAuto(view('main1', 0, 2, []));
    expect(postIntentMock).not.toHaveBeenCalled();
    p.startEndTurn(view('main1', 0, 2)); // the click takes the controls mid-beat
    expect(p.endTurn).toBe(true);
    p.considerAuto(view('main1', 0, 2)); // the effect re-runs considerAuto on oneShot
    await vi.advanceTimersByTimeAsync(200); // the re-derived wait, under the run
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.runPassed).toBe(1);
    expect(p.autoPassed).toBe(0); // the old auto wait did NOT post
    expect(p.autoLog.map((n) => n.text)).toEqual(['End turn: passed main 1']);
  });

  it('shift-click Hard Skip during a wait does the same under the skip register (r2)', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    p.adoptView(quiet(1));
    p.considerAuto(view('main1', 0, 2, [stackView('Lightning Bolt')]));
    expect(postIntentMock).not.toHaveBeenCalled();
    p.startHardSkip(view('main1', 0, 2, [stackView('Lightning Bolt')]));
    expect(p.hardSkip).toBe(true);
    p.considerAuto(view('main1', 0, 2, [stackView('Lightning Bolt')]));
    await vi.advanceTimersByTimeAsync(400); // resolveMs: a stack is resolving
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.runPassed).toBe(1);
    expect(p.autoPassed).toBe(0);
    expect(p.autoLog.map((n) => n.text)).toEqual(['Skip turn: passed main 1']);
  });

  it('Escape during the wait abandons the pass', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    p.adoptView(quiet(1));
    p.considerAuto(view());
    p.onKeydown('Escape');
    await vi.advanceTimersByTimeAsync(10_000);
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.pending?.seq).toBe(1); // the window is the player's again
  });

  it('a hand click during the wait abandons it: the click, not the machine, answers', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    p.setAuto(false); // the empty-window floor answers this one
    p.adoptView(manaLand(1));
    p.considerAuto(view('main1', 0, 2, []));
    expect(postIntentMock).not.toHaveBeenCalled(); // the floor is paced too
    p.click(1); // the land drop, by hand, mid-beat
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock.mock.calls[0][2].choices).toEqual([1]); // the LAND, not the pass
    await vi.advanceTimersByTimeAsync(10_000);
    expect(postIntentMock).toHaveBeenCalledTimes(1); // the scheduled pass never fired
    expect(p.emptySkipped).toBe(0); // the floor's pass was abandoned, not counted
    expect(p.actPassed).toBe(0);
  });

  it('the pass is never posted for a stale decision seq even if a timer somehow fires after adopt', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 100, resolveMs: 400 });
    p.adoptView(quiet(1));
    p.considerAuto(view());
    // adopt without cancelPassWait going through the normal path: drive the
    // timer past the fire point with the decision REPLACED underneath.
    p.adoptView(quiet(5));
    await vi.advanceTimersByTimeAsync(10_000);
    expect(postIntentMock).not.toHaveBeenCalled();
  });

  it('a one-shot run ended during the wait does not post (End Turn cancelled by Escape)', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    p.setAuto(false);
    p.setSkipEmpty(false);
    p.adoptView(quiet(1));
    p.startEndTurn(view('end', 0, 2));
    p.considerAuto(view('end', 0, 2));
    expect(p.endTurn).toBe(true);
    p.onKeydown('Escape');
    expect(p.endTurn).toBe(false);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.runPassed).toBe(0);
  });

  it('a rewind clears the pending decision, one-shot run and paced pass', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    p.adoptView(quiet(1));
    p.startEndTurn(view('end', 0, 2));
    p.considerAuto(view('end', 0, 2));
    expect(p.endTurn).toBe(true);

    p.rewind();
    expect(p.pending).toBeNull();
    expect(p.oneShot).toBe('none');
    expect(p.postedSeq).toBeNull();
    await vi.advanceTimersByTimeAsync(10_000);
    expect(postIntentMock).not.toHaveBeenCalled();
  });

  it('turning the machine off mid-beat (setAuto(false)) abandons its paced pass', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    p.adoptView(quiet(1));
    p.considerAuto(view());
    p.setAuto(false);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(postIntentMock).not.toHaveBeenCalled();
  });
});

describe('the log notes', () => {
  it('a pass with logAutoPasses on adds a client-local note; with it off, none', async () => {
    vi.useFakeTimers();
    const on = pacedSeat({ stepMs: 0, resolveMs: 0 }, true);
    on.adoptView(quiet(1));
    on.considerAuto(view('end', 1, 2));
    await settle(() => on.postedSeq === 1);
    expect(on.autoLog.map((n) => n.text)).toEqual(["Auto-passed: opponent's end step"]);

    const off = pacedSeat({ stepMs: 0, resolveMs: 0 }, false);
    off.adoptView(quiet(1));
    off.considerAuto(view('end', 1, 2));
    await settle(() => off.postedSeq === 1);
    expect(off.autoLog).toEqual([]);
  });

  it('the wording distinguishes the step from the resolving object, and your turn from theirs', () => {
    const seat = 0;
    expect(autoPassLogText('auto', view('end', 0), seat)).toBe('Auto-passed: your end step');
    expect(autoPassLogText('auto', view('end', 1), seat)).toBe("Auto-passed: opponent's end step");
    expect(autoPassLogText('auto', view('main2', 1), seat)).toBe("Auto-passed: opponent's main 2");
    expect(autoPassLogText('auto', view('main1', 0, 2, [stackView('Lightning Bolt')]), seat)).toBe('Auto-passed: Lightning Bolt resolving');
    // a trigger names itself the way the wire does
    expect(autoPassLogText('auto', view('upkeep', 1, 2, [{ ...stackView('Upkeep draw'), kind: 'trigger' }]), seat)).toBe('Auto-passed: Upkeep draw resolving');
  });

  it('the one-shot runs speak in their own register', () => {
    const seat = 0;
    expect(autoPassLogText('end-turn', view('main2', 0), seat)).toBe('End turn: passed main 2');
    expect(autoPassLogText('hard-skip', view('main2', 0, 2, [stackView('Giant Growth', 1)]), seat)).toBe('Skip turn: passed main 2');
  });

  it('paced wording is computed from the fresh view after that view gets its own full wait', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    p.adoptView(quiet(1));
    p.considerAuto(view('end', 1, 2));
    await vi.advanceTimersByTimeAsync(100);

    const named = view('end', 1, 2);
    named.players = [{ seat: 1, name: 'Ana' }] as unknown as View['players'];
    p.considerAuto(named);
    await vi.advanceTimersByTimeAsync(199);
    expect(postIntentMock).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    await settle(() => p.postedSeq === 1);

    expect(p.autoLog.map((n) => n.text)).toEqual(["Auto-passed: Ana's end step"]);
  });

  it('notes land only at POST time: an abandoned paced pass writes nothing', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 200, resolveMs: 400 });
    p.adoptView(quiet(1));
    p.considerAuto(view('end', 1, 2));
    p.onKeydown('Escape');
    await vi.advanceTimersByTimeAsync(10_000);
    expect(p.autoLog).toEqual([]);
  });

  it('the note list is capped at AUTO_LOG_CAP, oldest dropped', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 0, resolveMs: 0 });
    for (let i = 1; i <= AUTO_LOG_CAP + 5; i++) {
      p.adoptView(quiet(i));
      p.autoRun = 0; // this suite caps the LOG, not the pass run: keep the machine's own cap out of the way
      p.considerAuto(view('draw', 0, i));
      await settle(() => p.postedSeq === i);
    }
    expect(p.autoLog).toHaveLength(AUTO_LOG_CAP);
    expect(p.autoLog[0].turn).toBe(6); // the first five fell off
    expect(p.autoLog[AUTO_LOG_CAP - 1].turn).toBe(AUTO_LOG_CAP + 5);
  });
});

describe('the counters keep working through the pacing', () => {
  it('autoPassed, the note and the cap all advance only on real posts', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 50, resolveMs: 0 });
    for (let i = 1; i <= 3; i++) {
      p.adoptView(quiet(i));
      p.considerAuto(view());
      expect(p.autoPassed).toBe(i - 1); // not yet: the beat is running
      await vi.advanceTimersByTimeAsync(50);
      await settle(() => p.postedSeq === i);
      expect(p.autoPassed).toBe(i);
    }
    expect(autoNoteText(p.note)).toBe('Auto passed 3 priority windows.');
   });
});

describe('stepLabel — the wire step names, as words', () => {
  it('names every step the engine sends, un-hyphenated or special-cased', () => {
    expect(stepLabel('main1')).toBe('main 1');
    expect(stepLabel('main2')).toBe('main 2');
    expect(stepLabel('end')).toBe('end step');
    expect(stepLabel('draw')).toBe('draw step');
    expect(stepLabel('end-combat')).toBe('end of combat');
    expect(stepLabel('declare-attackers')).toBe('declare attackers');
    expect(stepLabel('upkeep')).toBe('upkeep');
    expect(stepLabel('cleanup')).toBe('cleanup');
  });
});

describe('casual still ships the paced defaults', () => {
  it('defaultSettings() keeps the settings table\u2019s 200/400 and logAutoPasses true', () => {
    expect(defaultSettings().pacing).toEqual({ stepMs: 200, resolveMs: 400 });
    expect(defaultSettings().logAutoPasses).toBe(true);
  });
});

// fb-20260917T004341Z: the Custom step-delay pair. paceMs reads
// settings.pacing generically, so any user-set pair drives the machine —
// this is the wiring pin that a pair set through the panel's Custom inputs
// schedules a wait (never posts synchronously, never posts early).
describe('a user-set (Custom) pacing pair drives the same wait', () => {
  it('750/750 schedules and posts only after the full beat, on a bare step and under a resolving object', async () => {
    vi.useFakeTimers();
    const p = pacedSeat({ stepMs: 750, resolveMs: 750 });
    p.adoptView(quiet(1));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled(); // nothing synchronous
    expect(vi.getTimerCount()).toBe(1); // a real wait is scheduled
    await vi.advanceTimersByTimeAsync(749);
    expect(postIntentMock).not.toHaveBeenCalled(); // still inside the beat
    await vi.advanceTimersByTimeAsync(1);
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    // Same pair under a resolving object: resolveMs (not stepMs) governs —
    // identical here, so pin that the wait restarts and waits the full beat.
    p.adoptView(quiet(2));
    p.considerAuto(view('main1', 0, 2, [stackView('Lightning Bolt')]));
    expect(postIntentMock).toHaveBeenCalledTimes(1); // not yet
    await vi.advanceTimersByTimeAsync(749);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    await settle(() => p.postedSeq === 2);
    expect(postIntentMock).toHaveBeenCalledTimes(2);
  });
});
