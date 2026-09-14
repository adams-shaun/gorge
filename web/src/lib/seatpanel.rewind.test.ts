import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Decision, Option, View } from '../protocol';
import { SeatPanelState, autoNoteText } from './seatpanel.svelte';

// The UNDO pause (fb-20260914T063523Z): pressing UNDO rewinds the board to
// the decision point of the player's last action, and the autopilot machine
// — auto-pass, the empty-window floor, pass-after-acting, the
// identical-trigger auto-order — must NOT instantly re-answer that restored
// window. rewind() sets machinePaused (the runaway brake, never the
// persisted preference); only the player's own resume paths clear it.
// The server half (host/undo.go, one intent per click) is already correct
// and untouched; these tests hold the client half.

const { postIntentMock, fetchPendingMock } = vi.hoisted(() => ({ postIntentMock: vi.fn(), fetchPendingMock: vi.fn() }));
vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  postIntent: postIntentMock,
  fetchPending: fetchPendingMock,
}));

const ctx = { seat: 0, token: 'tok' };

const pass = (i: number): Option => ({ index: i, kind: 'pass', label: 'Pass priority', player: 0 });
const concede = (i: number): Option => ({ index: i, kind: 'concede', label: 'Concede', player: 0 });
const cast = (i: number): Option => ({ index: i, kind: 'cast', label: `Cast ${i}`, player: 0 });
const trigger = (i: number, label: string, obj: number): Option => ({ index: i, kind: 'trigger', label, obj, player: 0 });

/** quiet is a priority window with nothing to do: pass and concede only. */
const quiet = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [pass(0), concede(1)] });

/** live is a priority window where the player has a real action. */
const live = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [cast(0), pass(1), concede(2)] });

/** triggerOrder is the measured wire shape of a trigger_order decision (mirrors seatpanel.prio6.test.ts). */
const triggerOrder = (seq: number, labels: [string, string]): Decision => ({
  seq,
  player: 0,
  kind: 'trigger_order',
  prompt: 'Order your simultaneous triggered abilities.',
  min: 2,
  max: 2,
  options: labels.map((label, i) => trigger(i, label, 81 + i)),
});
const IDENTICAL: [string, string] = ['Blood Artist: loses 1 life', 'Blood Artist: loses 1 life'];

const view = (step = 'draw', active = 0, turn = 2, stack: { id: number; controller: number; kind?: string }[] = []): View =>
  ({ active, step, turn, stack }) as unknown as View;

/** armedSeat is casual (auto ON — the default) with every stop off and zero pacing, so machine passes post synchronously. */
function armedSeat(): SeatPanelState {
  const p = new SeatPanelState('t1', 1, ctx, null);
  p.stops = { yours: new Set(), opponents: new Set() };
  p.setAuto(true);
  p.settings = { ...p.settings, pacing: { stepMs: 0, resolveMs: 0 } };
  return p;
}

/** manualSeat is casual with auto off — the empty-window floor is the only machine path left. */
function manualSeat(): SeatPanelState {
  const p = armedSeat();
  p.setAuto(false);
  return p;
}

async function settle(predicate: () => boolean, maxTicks = 200): Promise<void> {
  for (let i = 0; i < maxTicks; i++) {
    if (predicate()) return;
    await Promise.resolve();
  }
  throw new Error(`settle: condition still false after ${maxTicks} microtask ticks`);
}

const seqs = () => postIntentMock.mock.calls.map((c) => (c[2] as { seq: number }).seq);

beforeEach(() => {
  postIntentMock.mockReset();
  fetchPendingMock.mockReset();
  postIntentMock.mockResolvedValue(undefined);
});

describe('the undo pause', () => {
  it('rewind pauses the machine, surfaces the paused note, and leaves the persisted autoPass untouched', () => {
    const p = armedSeat();
    p.adoptView(quiet(1));
    p.considerAuto(view());
    p.rewind();
    expect(p.machinePaused).toBe(true);
    expect(p.note.kind).toBe('paused');
    expect(autoNoteText(p.note)).toContain('Undo paused automatic passing');
    // The preference is the player's, not the match's: a reload comes back
    // with auto exactly as it was.
    expect(p.auto).toBe(true);
    expect(p.settings.autoPass).toBe(true);
  });

  it('with auto ON, a rewind leaves the restored decision PENDING until the player answers by hand', async () => {
    const p = armedSeat();
    // The pre-undo game: the player casts by hand (seq 1), then auto passes
    // a quiet window (seq 2).
    p.adoptView(live(1));
    p.click(0);
    await settle(() => p.postedSeq === 1);
    p.adoptView(quiet(2));
    p.considerAuto(view());
    await settle(() => p.postedSeq === 2);
    expect(seqs()).toEqual([1, 2]);

    // UNDO: the board rewinds to the seq-2 decision point (same seq, a fresh
    // object from the rewind snapshot — as the real adopt path delivers it).
    p.rewind();
    p.adoptView(quiet(2));
    p.considerAuto(view());
    await settle(() => postIntentMock.mock.calls.length === 2); // nothing new posts
    expect(p.machinePaused).toBe(true);
    expect(p.pending?.seq).toBe(2);
    expect(p.active?.seq).toBe(2); // the window is live for the player

    // The player answers it by hand — the only post that happens.
    p.click(0); // pass
    await settle(() => postIntentMock.mock.calls.length === 3);
    expect(seqs()[2]).toBe(2);
    // A hand answer while paused does NOT clear the brake (only the Auto
    // switch or a preset resumes the machine).
    expect(p.machinePaused).toBe(true);
  });

  it('the empty-window floor does not pass a restored empty window while paused; the floor resumes when the pause lifts', async () => {
    const p = manualSeat(); // auto off, skipEmpty on (the default) — the floor is the machine path that fires even with auto off
    p.adoptView(quiet(5));
    p.rewind();
    p.adoptView(quiet(5));
    p.considerAuto(view());
    await settle(() => postIntentMock.mock.calls.length === 0);
    expect(p.pending?.seq).toBe(5);
    expect(p.emptySkipped).toBe(0);

    // Resume: the Auto switch clears the brake, and the machine paths run again.
    p.setAuto(true);
    expect(p.machinePaused).toBe(false);
    p.considerAuto(view());
    await settle(() => p.postedSeq === 5);
    expect(seqs()).toEqual([5]);
  });

  it('maybeAutoOrderTriggers does not auto-submit a restored identical trigger-order ask while paused; it resumes after the brake clears', async () => {
    const p = armedSeat();
    p.adoptView(triggerOrder(7, IDENTICAL)); // pre-undo: the machine ordered it
    await settle(() => p.postedSeq === 7);
    expect(seqs()).toEqual([7]);

    p.rewind();
    p.adoptView(triggerOrder(7, IDENTICAL));
    await settle(() => postIntentMock.mock.calls.length === 1); // no auto-submit, at adopt or after
    p.considerAuto(view());
    await settle(() => postIntentMock.mock.calls.length === 1);
    expect(p.pending?.seq).toBe(7);
    expect(p.autoLog.some((n) => n.text.includes('identical triggers'))).toBe(false);

    // Resume by applying a named preset: the ask is then auto-ordered as before.
    p.applyNamedPreset('casual');
    expect(p.machinePaused).toBe(false);
    p.considerAuto(view());
    await settle(() => p.postedSeq === 7);
    expect(seqs()).toEqual([7, 7]);
  });

  it('an in-flight hand answer whose response lands after a rewind cannot mark the restored same-seq decision as answered', async () => {
    const p = armedSeat();
    // A controlled post: click the cast, hold the response in flight.
    let release!: () => void;
    postIntentMock.mockReturnValueOnce(new Promise<void>((res) => (release = res)));
    p.adoptView(live(1));
    p.click(0);
    // The rewind frame lands while the post is still awaiting the server.
    p.rewind();
    p.adoptView(live(1)); // the restored decision: SAME seq, fresh object
    release();
    await settle(() => !p.busy);
    // The response's bookkeeping must not have marked the restored decision
    // answered, and no error may surface for a post the undo superseded.
    expect(p.postedSeq).toBeNull();
    expect(p.active?.seq).toBe(1);
    expect(p.pending?.seq).toBe(1);
    expect(p.error).toBeNull();
    // And the machine stays paused: the restored window is the player's.
    p.considerAuto(view());
    await settle(() => postIntentMock.mock.calls.length === 1);
    expect(p.machinePaused).toBe(true);
  });

  it('two consecutive undos each walk back one of the player\'s own intents, and the machine posts nothing in between', async () => {
    const p = armedSeat();
    // Player's intent #1: the seq-1 cast, by hand. Machine's: the seq-2 pass.
    p.adoptView(live(1));
    p.click(0);
    await settle(() => p.postedSeq === 1);
    p.adoptView(quiet(2));
    p.considerAuto(view());
    await settle(() => p.postedSeq === 2);
    expect(seqs()).toEqual([1, 2]);

    // UNDO #1 rewinds to the seq-2 decision point; the machine stays silent;
    // the player answers by hand (intent #2).
    p.rewind();
    p.adoptView(quiet(2));
    p.considerAuto(view());
    await settle(() => postIntentMock.mock.calls.length === 2);
    p.click(0);
    await settle(() => postIntentMock.mock.calls.length === 3);
    expect(seqs()).toEqual([1, 2, 2]);

    // UNDO #2 rewinds to the seq-1 decision point; again the machine stays
    // silent and the window waits for the player.
    p.rewind();
    p.adoptView(live(1));
    p.considerAuto(view());
    await settle(() => postIntentMock.mock.calls.length === 3);
    expect(p.pending?.seq).toBe(1);
    expect(p.active?.seq).toBe(1);
    p.click(0);
    await settle(() => postIntentMock.mock.calls.length === 4);
    expect(seqs()).toEqual([1, 2, 2, 1]);
  });

  it('the pause-aware Auto switch resumes without touching the enabled preference (the real r2 break)', () => {
    // r2 finding: the paused note told the default auto-ON player to press
    // the Auto switch, but the switch's onclick was setAuto(!autoPass) —
    // with the persisted default that DISABLED auto, cleared the brake and
    // the empty-window floor instantly passed the restored window. The
    // pause-aware switch (pressAuto) must instead call setAuto(true): the
    // brake lifts and the enabled preference survives untouched.
    const p = armedSeat(); // autoPass true — the Casual default
    p.rewind();
    expect(p.machinePaused).toBe(true);
    p.pressAuto();
    expect(p.machinePaused).toBe(false);
    expect(p.settings.autoPass).toBe(true);
    expect(p.auto).toBe(true);
    expect(p.note.kind).toBe('armed');
  });

  it('the pause-aware Auto switch on a paused Manual seat turns auto on, and the machine runs again', async () => {
    const p = manualSeat();
    p.adoptView(quiet(9));
    p.rewind();
    p.adoptView(quiet(9));
    p.considerAuto(view());
    await settle(() => postIntentMock.mock.calls.length === 0); // paused: nothing posts
    p.pressAuto(); // the switch reads Paused; pressing it starts the machine
    expect(p.machinePaused).toBe(false);
    expect(p.settings.autoPass).toBe(true);
    p.considerAuto(view());
    await settle(() => p.postedSeq === 9);
    expect(seqs()).toEqual([9]);
  });

  it('unpaused, pressAuto is the ordinary toggle the switches always were', () => {
    const p = armedSeat();
    p.pressAuto();
    expect(p.settings.autoPass).toBe(false);
    p.pressAuto();
    expect(p.settings.autoPass).toBe(true);
  });

  it('both resume paths clear the pause: the Auto switch and applying a named preset', async () => {
    const viaSwitch = armedSeat();
    viaSwitch.rewind();
    expect(viaSwitch.machinePaused).toBe(true);
    viaSwitch.setAuto(true);
    expect(viaSwitch.machinePaused).toBe(false);
    expect(viaSwitch.note.kind).toBe('armed');

    const viaPreset = armedSeat();
    viaPreset.rewind();
    viaPreset.applyNamedPreset('casual');
    expect(viaPreset.machinePaused).toBe(false);
    expect(viaPreset.note.kind).toBe('armed');
  });

  it('while the pause holds a one-shot run cannot arm: the machine answers nothing on the restored window (r2 finding)', async () => {
    // r2 finding: startRun used to lift the pause — an unauthorized resume
    // path (End Turn / Hard Skip / Resolve All could re-enable machine
    // posting on the just-restored window). Now the arm is REFUSED while
    // the pause holds: the run never arms, the paused note stays on screen
    // (an armed run chip would overwrite it with a note claiming the
    // machine is passing), and nothing posts. After the player resumes,
    // the buttons arm as usual.
    const p = armedSeat();
    p.adoptView(quiet(3));
    p.rewind();
    p.adoptView(quiet(3));
    p.startEndTurn(view('main1', 0, 2));
    expect(p.oneShot).toBe('none');
    expect(p.machinePaused).toBe(true);
    expect(p.note.kind).toBe('paused');
    p.considerAuto(view('main1', 0, 2));
    await settle(() => postIntentMock.mock.calls.length === 0);
    expect(p.pending?.seq).toBe(3);

    // Resume, and the run arms and passes the restored window as before.
    p.pressAuto();
    p.startEndTurn(view('main1', 0, 2));
    expect(p.oneShot).toBe('end-turn');
    p.considerAuto(view('main1', 0, 2));
    await settle(() => p.postedSeq === 3);
    expect(p.endTurn).toBe(true);
  });

  it('a paced pass in flight when the rewind frame lands never fires into the new seq space', async () => {
    vi.useFakeTimers();
    try {
      const p = armedSeat();
      p.settings = { ...p.settings, pacing: { stepMs: 200, resolveMs: 400 } };
      p.adoptView(quiet(1));
      p.considerAuto(view());
      await vi.advanceTimersByTimeAsync(199); // the paced pass is mid-beat, not yet posted
      expect(postIntentMock).not.toHaveBeenCalled();
      p.rewind();
      p.adoptView(quiet(1)); // the restored window, same seq
      await vi.advanceTimersByTimeAsync(10_000);
      expect(postIntentMock).not.toHaveBeenCalled();
      expect(p.pending?.seq).toBe(1); // still the player's to answer
      expect(p.machinePaused).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });
});
