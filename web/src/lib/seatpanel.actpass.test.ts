import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Decision, Option, View } from '../protocol';
import { actPassKey, loadActPass } from './actpass';
import { SeatPanelState, autoNoteText } from './seatpanel.svelte';

// The "pass after acting" preference: after this seat POSTS a hand answer to
// a priority decision carrying a real action, its NEXT priority window is
// passed once. decide() is the safety oracle and is tested in
// autopilot.test.ts; what these tests hold down is the token's lifecycle —
// what arms it, what consumes it, what clears it, and that the machine can
// only ever post a pass.
const { postIntentMock, fetchPendingMock } = vi.hoisted(() => ({ postIntentMock: vi.fn(), fetchPendingMock: vi.fn() }));
vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  postIntent: postIntentMock,
  fetchPending: fetchPendingMock,
}));

const ctx = { seat: 0, token: 'tok' };

const cast = (i: number): Option => ({ index: i, kind: 'cast', label: `Cast ${i}`, player: 0 });
const ability = (i: number): Option => ({ index: i, kind: 'ability', label: `Ability ${i}`, player: 0 });
const pass = (i: number): Option => ({ index: i, kind: 'pass', label: 'Pass priority', player: 0 });
const concede = (i: number): Option => ({ index: i, kind: 'concede', label: 'Concede', player: 0 });
const target = (i: number): Option => ({ index: i, kind: 'target', label: `Target ${i}`, player: 0 });

/** quiet is a priority window with nothing to do: pass and concede only. */
const quiet = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [pass(0), concede(1)] });

/** live is a priority window where the player has a real action. */
const live = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [cast(0), pass(1), concede(2)] });

/** choose is a non-priority decision — a target ask, the shape a cast hands back. */
const targetAsk = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'target', prompt: 'Choose a target.', min: 1, max: 1, options: [target(0), target(1)] });

const mulligan = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'mulligan', prompt: 'Keep?', min: 1, max: 1, options: [pass(0), concede(1)] });

/** multi is a multi-pick priority window (max > 1), answered with submit(). */
const multi = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 0, max: 2, options: [cast(0), ability(1), pass(2), concede(3)] });

const view = (step = 'draw', active = 0, stack: { id: number; controller: number }[] = []): View =>
  ({ active, step, turn: 2, stack }) as unknown as View;

function fakeStorage(): Storage {
  const store = new Map<string, string>();
  return {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, v),
    removeItem: (k: string) => void store.delete(k),
    clear: () => void store.clear(),
    key: (i: number) => [...store.keys()][i] ?? null,
    get length() {
      return store.size;
    },
  } as unknown as Storage;
}

async function settle(predicate: () => boolean, maxTicks = 200): Promise<void> {
  for (let i = 0; i < maxTicks; i++) {
    if (predicate()) return;
    await Promise.resolve();
  }
  throw new Error(`settle: condition still false after ${maxTicks} microtask ticks`);
}

/** manual is a seat with the preference ON, auto/FFWD off and no stops set. */
function manual(storage: Storage | null = null): SeatPanelState {
  const p = new SeatPanelState('t1', 1, ctx, storage);
  p.stops = { yours: new Set(), opponents: new Set() };
  p.actPass = true;
  return p;
}

/** arm posts one cast by hand so the token is armed for what follows. */
async function arm(p: SeatPanelState) {
  p.adoptView(live(1));
  p.click(0);
  await settle(() => p.postedSeq === 1);
  postIntentMock.mockClear();
}

describe('pass after acting — the preference and its persistence', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('is OFF on a fresh seat, and a fresh browser (no saved value) loads OFF', () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    expect(p.actPass).toBe(false);
    p.mountActPass();
    expect(p.actPass).toBe(false);
    expect(p.actPassed).toBe(0);
  });

  it('setActPass persists per table AND per seat, and a second seat reads its own default', () => {
    const storage = fakeStorage();
    const p = new SeatPanelState('t1', 1, ctx, storage);
    p.setActPass(true);
    expect(p.actPass).toBe(true);
    expect(storage.getItem(actPassKey('t1', 0))).toBe('1');

    const other = new SeatPanelState('t1', 1, { seat: 1, token: 'tok' }, storage);
    other.mountActPass();
    expect(other.actPass).toBe(false); // seat 1 did not opt in

    // OFF is stored explicitly, so a deliberate "off" survives a reload.
    p.setActPass(false);
    expect(storage.getItem(actPassKey('t1', 0))).toBe('0');
    const reloaded = new SeatPanelState('t1', 1, ctx, storage);
    reloaded.mountActPass();
    expect(reloaded.actPass).toBe(false);

    // and a genuinely absent key loads false, never throws
    expect(loadActPass(null, 't1', 0)).toBe(false);
  });

  it('the preference survives a match boundary like the stops; the armed token and counter do not', () => {
    const p = manual();
    p.actPassed = 3;
    p.begin();
    expect(p.actPass).toBe(true); // persisted preference, not a per-match opt-in
    expect(p.actPassed).toBe(0);
  });
});

describe('pass after acting — what arms the token', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('THE REPORTED FLOW, end to end: cast click arms → the spell’s target decision is hand-answered → the next priority window is machine-passed exactly once, with its own note', async () => {
    const p = manual();
    // 1. the cast: a hand click on a live priority window posts the cast…
    p.adoptView(live(1));
    p.considerAuto(view()); // no token yet — the window surfaces as today
    expect(postIntentMock).not.toHaveBeenCalled();
    p.click(0);
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);

    // 2. the spell's target decision: non-priority, hand-answered by the
    // player. The token must survive it — and it must not ANSWER it.
    p.adoptView(targetAsk(2));
    p.considerAuto(view());
    expect(postIntentMock).toHaveBeenCalledTimes(1); // never machine-answered
    p.click(0); // the target — a hand answer
    await settle(() => p.postedSeq === 2);
    expect(postIntentMock).toHaveBeenCalledTimes(2);

    // 3. the next priority window: machine-passed once, exactly once.
    p.adoptView(live(3));
    p.considerAuto(view());
    await settle(() => p.postedSeq === 3);
    expect(postIntentMock).toHaveBeenCalledTimes(3);
    expect(postIntentMock.mock.calls[2][2].choices).toEqual([1]); // the PASS option's wire index
    expect(p.actPassed).toBe(1);
    expect(p.autoPassed).toBe(0); // never credited to auto
    expect(p.emptySkipped).toBe(0); // nor to the empty-window floor
    expect(autoNoteText(p.note)).toBe('Passed 1 priority window after your action.');

    // 4. exactly once: the window AFTER that is the player's again.
    p.adoptView(live(4));
    p.considerAuto(view());
    expect(postIntentMock).toHaveBeenCalledTimes(3);
    expect(p.actPassed).toBe(1);
  });

  it('a multi-pick submit arms too (the priority window the submit button answers)', async () => {
    const p = manual();
    p.adoptView(multi(1));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled(); // max > 1: the click toggles, it does not post
    p.click(0); // pick the cast
    expect(p.picked).toEqual([0]);
    p.submit();
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock.mock.calls[0][2].choices).toEqual([0]);

    p.adoptView(live(2));
    p.considerAuto(view());
    await settle(() => p.postedSeq === 2);
    expect(p.actPassed).toBe(1);
  });

  it('a multi-pick submit of only passes does NOT arm', async () => {
    const p = manual();
    p.adoptView(multi(1));
    p.click(2); // the pass option
    p.submit();
    await settle(() => p.postedSeq === 1);
    p.adoptView(live(2));
    p.considerAuto(view());
    expect(postIntentMock).toHaveBeenCalledTimes(1); // the submit only; no machine pass
  });

  it.each([
    ['the dedicated pass button', (p: SeatPanelState) => p.passClick()],
    ['the primary (pass/resolve) button', (p: SeatPanelState) => p.primaryClick()],
  ])('%s posts its pass but does not arm', async (_name, act) => {
    const p = manual();
    p.adoptView(multi(1));
    act(p);
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1); // the pass itself
    postIntentMock.mockClear();
    p.adoptView(live(2));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled(); // nothing was armed
  });

  it('a toggle without submit does not arm', async () => {
    const p = manual();
    p.adoptView(multi(1));
    p.toggle(0);
    await settle(() => p.busy === false);
    p.adoptView(live(2));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled();
  });

  it('conceding does not arm', async () => {
    const p = manual();
    p.adoptView(live(1));
    p.click(2); // arms the confirmation
    expect(p.confirming).toBe(true);
    p.confirmConcede();
    await settle(() => p.postedSeq === 1);
    p.adoptView(live(2));
    p.considerAuto(view());
    expect(postIntentMock).toHaveBeenCalledTimes(1); // the concede only
  });

  it('the empty-window floor’s own pass does not arm', async () => {
    const p = manual();
    p.adoptView(quiet(1));
    p.considerAuto(view());
    await settle(() => p.postedSeq === 1);
    expect(p.emptySkipped).toBe(1);
    p.adoptView(live(2));
    p.considerAuto(view());
    expect(postIntentMock).toHaveBeenCalledTimes(1); // the floor only; the live window is the player's
  });

  it('a hand action does NOT arm while the preference is off', async () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.stops = { yours: new Set(), opponents: new Set() };
    p.adoptView(live(1));
    p.click(0);
    await settle(() => p.postedSeq === 1);
    p.adoptView(live(2));
    p.considerAuto(view());
    expect(postIntentMock).toHaveBeenCalledTimes(1); // the cast only
  });

  it('a rejected cast never arms: the next window is the player’s', async () => {
    const p = manual();
    postIntentMock.mockRejectedValueOnce(new Error('stale seq'));
    fetchPendingMock.mockResolvedValue(live(1));
    p.adoptView(live(1));
    p.click(0);
    await settle(() => p.busy === false && p.error !== null);
    p.adoptView(live(1)); // the same decision comes back
    p.considerAuto(view());
    expect(postIntentMock).toHaveBeenCalledTimes(1); // the failed cast; no machine pass
  });
});

describe('pass after acting — where the token is spent and where it is not', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('a stop the player set on the current step/side is honoured: no post, token consumed without acting', async () => {
    const p = manual();
    p.stops = { yours: new Set(['main1']), opponents: new Set() };
    await arm(p);
    p.adoptView(live(4));
    p.considerAuto(view('main1', 0));
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(autoNoteText(p.note)).toBe('Auto is off. You answer every window that offers you something to do.');

    // consumed: the NEXT window, away from the stop, is the player's again
    p.adoptView(live(5));
    p.considerAuto(view('draw', 0));
    expect(postIntentMock).not.toHaveBeenCalled();
  });

  it('an opponent-controlled stack object is honoured: no post, token consumed without acting', async () => {
    const p = manual();
    await arm(p);
    p.adoptView(live(4));
    p.considerAuto(view('main1', 0, [{ id: 9, controller: 1 }]));
    expect(postIntentMock).not.toHaveBeenCalled();
    p.adoptView(live(5));
    p.considerAuto(view('draw', 0));
    expect(postIntentMock).not.toHaveBeenCalled(); // consumed, not deferred
  });

  it('a shape decide() does not understand is honoured: no post, token consumed', async () => {
    const p = manual();
    await arm(p);
    p.adoptView({ ...live(4), min: 0 });
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled();
    p.adoptView(live(5));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled();
  });

  it('a non-priority decision neither consumes the token nor is answered by it — the token fires at the NEXT priority window', async () => {
    const p = manual();
    await arm(p);
    p.adoptView(mulligan(4));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled();
    p.adoptView(live(5));
    p.considerAuto(view());
    await settle(() => p.postedSeq === 5);
    expect(p.actPassed).toBe(1);
  });

  it('a failed machine pass never retries: the token is spent, the window surfaces to the player', async () => {
    const p = manual();
    await arm(p);
    postIntentMock.mockRejectedValueOnce(new Error('stale seq'));
    fetchPendingMock.mockResolvedValue(live(3));
    p.adoptView(live(3));
    p.considerAuto(view());
    await settle(() => p.busy === false && p.error !== null);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.actPassed).toBe(1); // counted when posted; the rejection surfaces as an error

    p.adoptView(live(3)); // the same seq comes back
    p.considerAuto(view());
    expect(postIntentMock).toHaveBeenCalledTimes(1); // no retry
    p.adoptView(live(4));
    p.considerAuto(view());
    expect(postIntentMock).toHaveBeenCalledTimes(1); // and the next window is the player's — one-shot
  });
});

describe('pass after acting — dormant while a machine run is live', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('while Auto runs, its own rules govern and the token stays dormant: the pass is Auto’s, not the preference’s', async () => {
    const p = manual();
    p.auto = true;
    p.actPass = true;
    // Arm the token through the one hand answer that leaves Auto armed: the
    // window Auto stopped at for the player's OWN set stop.
    p.stops = { yours: new Set(['main1']), opponents: new Set() };
    p.adoptView(live(1));
    p.considerAuto(view('main1', 0));
    expect(p.auto).toBe(true); // Auto kept the wheel
    p.click(0); // the answer the stop invited
    await settle(() => p.postedSeq === 1);
    expect(p.auto).toBe(true);
    postIntentMock.mockClear();

    // Auto's next window is answered by AUTO — counted and worded as Auto's.
    p.adoptView(quiet(2));
    p.considerAuto(view('main1', 0));
    await settle(() => p.postedSeq === 2);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.autoPassed).toBe(1);
    expect(p.actPassed).toBe(0);
    expect(autoNoteText(p.note)).toBe('Auto passed 1 priority window.');

    // Auto switched off by hand; the still-armed token now spends itself —
    // away from the stop that is still set.
    p.setAuto(false);
    p.adoptView(live(3));
    p.considerAuto(view('draw', 0));
    await settle(() => p.postedSeq === 3);
    expect(p.actPassed).toBe(1);
    expect(autoNoteText(p.note)).toBe('Passed 1 priority window after your action.');
  });

  it('while fast forward runs the token stays dormant, and spends itself after the run ends', async () => {
    const p = manual();
    p.stops = { yours: new Set(['draw']), opponents: new Set() };
    await arm(p); // armed, manual mode
    p.adoptView(quiet(2));
    p.startFastForward(); // FFWD takes the wheel before the token is spent
    p.considerAuto(view('draw', 0));
    await settle(() => p.postedSeq === 2);
    expect(p.fastPassed).toBe(1);
    expect(p.actPassed).toBe(0); // dormant, not consumed

    // The run ends at the player's stop; the token then spends itself.
    p.adoptView(live(3));
    p.considerAuto(view('draw', 0));
    expect(p.fastForward).toBe(false);
    expect(postIntentMock).toHaveBeenCalledTimes(1); // FFWD's pass only
    expect(p.actPassed).toBe(0); // not consumed by the stop either

    p.adoptView(quiet(4));
    p.considerAuto(view('draw', 0));
    await settle(() => p.postedSeq === 4);
    expect(p.actPassed).toBe(1);
    expect(autoNoteText(p.note)).toBe('Passed 1 priority window after your action.');
  });

  it('turning the preference off disarms a still-armed token', async () => {
    const p = manual();
    await arm(p);
    p.setActPass(false);
    p.adoptView(live(2));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled();
  });
});

describe('pass after acting — what clears an armed token', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('begin() clears it', async () => {
    const p = manual();
    await arm(p);
    p.begin();
    p.adoptView(live(2));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled();
  });

  it('Escape clears it, even in manual mode where auto’s own guards would skip', async () => {
    const p = manual();
    await arm(p);
    p.onKeydown('Escape');
    p.adoptView(live(2));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled();
  });

  it('conceding clears it in manual mode, where the suspend helpers are no-ops (click() path, both clicks)', async () => {
    const p = manual();
    await arm(p);
    p.adoptView(live(2));
    p.click(2); // the concede option: first click arms the confirmation…
    expect(p.confirming).toBe(true);
    p.click(2); // …the second posts it — and must clear the armed token
    await settle(() => p.postedSeq === 2);
    p.adoptView(live(3));
    p.considerAuto(view());
    expect(postIntentMock).toHaveBeenCalledTimes(1); // the concede post only; no machine pass
  });

  it('confirmConcede() clears it in manual mode too', async () => {
    const p = manual();
    await arm(p);
    p.adoptView(live(2));
    p.click(2); // arms the confirmation
    expect(p.confirming).toBe(true);
    p.confirmConcede(); // the explicit second confirmation
    await settle(() => p.postedSeq === 2);
    p.adoptView(live(3));
    p.considerAuto(view());
    expect(postIntentMock).toHaveBeenCalledTimes(1); // the concede post only; no machine pass
  });

  it('answering by hand while a fast-forward run is live clears it (the cancel fired, the pass did not re-arm)', async () => {
    const p = manual();
    await arm(p);
    p.adoptView(live(2));
    p.startFastForward(); // FFWD takes the wheel before considerAuto spends the token
    p.passClick(); // a human click during the run cancels it…
    await settle(() => p.postedSeq === 2);
    expect(p.fastForward).toBe(false);
    p.adoptView(live(3));
    p.considerAuto(view());
    expect(postIntentMock).toHaveBeenCalledTimes(1); // the passClick only; the token went with the cancel
  });

  it('a hand answer that disarms auto clears it — and the cast that caused both re-arms nothing extra', async () => {
    const storage = fakeStorage();
    const p = new SeatPanelState('t1', 1, ctx, storage);
    p.stops = { yours: new Set(), opponents: new Set() };
    p.actPass = true;
    p.auto = true;
    // the token arms only via a hand action while auto is ON — the player's
    // own stop exception is that shape; here any cast click suspends auto.
    p.adoptView(live(1));
    p.click(0);
    await settle(() => p.postedSeq === 1);
    expect(p.auto).toBe(false); // the click was a takeover
    p.adoptView(live(2));
    p.considerAuto(view());
    // auto was disarmed by the same click that armed the token; the token
    // survived the suspend (it re-arms after handAnswer) and fires here —
    // exactly one window after the player's action.
    await settle(() => p.postedSeq === 2);
    expect(p.actPassed).toBe(1);
  });
});
