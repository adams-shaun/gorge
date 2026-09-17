import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Decision, Option, View } from '../protocol';
import type { StopReason } from './autopilot';
import { AUTO_PASS_CAP, SeatPanelState, autoNoteText, type AutoOffReason } from './seatpanel.svelte';
import { defaultSettings } from './playsettings';

// The autopilot LOOP, not decide(). decide() is pure and tested in
// autopilot.test.ts; what these tests hold down is the thing that can lose a
// game in silence — the loop that calls it, posts for the player, and has to
// stop itself when something goes wrong. The settings model
// (playsettings.ts) is the source of truth: `auto` is settings.autoPass, the
// stops are settings.steps, actPass is settings.passAfterAct.
const { postIntentMock, fetchPendingMock } = vi.hoisted(() => ({ postIntentMock: vi.fn(), fetchPendingMock: vi.fn() }));
vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  postIntent: postIntentMock,
  fetchPending: fetchPendingMock,
}));

const ctx = { seat: 0, token: 'tok' };

const cast = (i: number): Option => ({ index: i, kind: 'cast', label: `Cast ${i}`, player: 0 });
const pass = (i: number): Option => ({ index: i, kind: 'pass', label: 'Pass priority', player: 0 });
const concede = (i: number): Option => ({ index: i, kind: 'concede', label: 'Concede', player: 0 });
const activate = (i: number): Option => ({ index: i, kind: 'activate', label: `Tap ${i}`, player: 0 });
const land = (i: number): Option => ({ index: i, kind: 'play_land', label: `Play land ${i}`, player: 0 });

/** manaLand is a main-phase window whose only real action is a land drop (plus the mana taps every window carries). */
const manaLand = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [activate(0), land(1), pass(2), concede(3)] });

/** quiet is a priority window with nothing to do: pass and concede only. */
const quiet = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [pass(0), concede(1)] });

/** live is a priority window where the player has a real action. */
const live = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [cast(0), pass(1), concede(2)] });

const mulligan = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'mulligan', prompt: 'Keep?', min: 1, max: 1, options: [pass(0), concede(1)] });

const view = (step = 'draw', active = 0, turn = 2, stack: { id: number; controller: number; kind?: string }[] = []): View =>
  ({ active, step, turn, stack }) as unknown as View;

/** manualSeat is casual with auto off, every stop off and pass-after-acting off — the player, answering everything. Pacing is zeroed so the machine passes post synchronously (the pre-prio5 path); pacing itself is tested in seatpanel.pacing.test.ts. */
function manualSeat(storage: Storage | null = null): SeatPanelState {
  const p = new SeatPanelState('t1', 1, ctx, storage);
  p.stops = { yours: new Set(), opponents: new Set() };
  p.setAuto(false);
  p.setActPass(false);
  p.settings = { ...p.settings, pacing: { stepMs: 0, resolveMs: 0 } };
  return p;
}

/** armedSeat is casual with every stop off — auto on, nothing stopping it but the guards. */
function armedSeat(storage: Storage | null = null): SeatPanelState {
  const p = manualSeat(storage);
  p.setAuto(true);
  return p;
}

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

describe('the settings source of truth', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('auto defaults ON with empty storage (casual), and answers an ordinary window', async () => {
    const storage = fakeStorage();
    const p = new SeatPanelState('t1', 1, ctx, storage);
    expect(p.auto).toBe(true);
    expect(p.settings).toEqual(defaultSettings());
    p.stops = { yours: new Set(), opponents: new Set() };
    p.settings = { ...p.settings, pacing: { stepMs: 0, resolveMs: 0 } }; // the pacing suite drives the wait; this one pins the defaults
    p.adoptView(quiet(1));
    p.considerAuto(view());
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.autoPassed).toBe(1);
  });

  it('auto stays ON after a hand answer — answering is not a request to stop auto-passing', async () => {
    const p = armedSeat();
    p.adoptView(live(1));
    p.click(0); // the cast, by hand
    await settle(() => p.postedSeq === 1);
    expect(p.auto).toBe(true);
    expect(p.settings.autoPass).toBe(true);
    // and Auto still answers the next ordinary window
    p.adoptView(quiet(2));
    p.considerAuto(view());
    await settle(() => p.postedSeq === 2);
    expect(postIntentMock).toHaveBeenCalledTimes(2);
  });

  it('auto stays ON across begin() — the preference is the player\u2019s, not the match\u2019s', () => {
    const storage = fakeStorage();
    const p = new SeatPanelState('t1', 1, ctx, storage);
    expect(p.auto).toBe(true);
    p.autoPassed = 4;
    p.begin();
    expect(p.auto).toBe(true);
    expect(p.settings.autoPass).toBe(true);
    expect(p.autoPassed).toBe(0); // only the session counters reset
  });

  it('a saved autoPass=false survives a reload, and toggling writes the global key', () => {
    const storage = fakeStorage();
    const p = new SeatPanelState('t1', 1, ctx, storage);
    p.setAuto(false);
    expect(JSON.parse(storage.getItem('gorge.playsettings.v1') as string).autoPass).toBe(false);

    const reloaded = new SeatPanelState('t1', 1, ctx, storage);
    expect(reloaded.auto).toBe(false);
  });

  it('Escape cancels a one-shot run but does NOT flip settings.autoPass', () => {
    const p = armedSeat();
    p.adoptView(quiet(1));
    p.startEndTurn(view());
    expect(p.endTurn).toBe(true);
    p.onKeydown('Escape');
    expect(p.endTurn).toBe(false);
    expect(p.auto).toBe(true);
    expect(p.settings.autoPass).toBe(true);
  });

  it('the stops live in settings.steps: toggleStop writes the rule and the global key', () => {
    const storage = fakeStorage();
    const p = new SeatPanelState('t1', 1, ctx, storage);
    p.stops = { yours: new Set(), opponents: new Set() };
    p.toggleStop('upkeep', 'yours');
    expect(p.settings.steps.yours.upkeep).toBe('smart');
    expect(JSON.parse(storage.getItem('gorge.playsettings.v1') as string).steps.yours.upkeep).toBe('smart');
    p.toggleStop('upkeep', 'yours');
    expect(p.settings.steps.yours.upkeep).toBe('off');
    // untap and cleanup grant no priority and are refused: no rule is written
    p.toggleStop('untap', 'yours');
    expect(p.settings.steps.yours).not.toHaveProperty('untap');
  });

  it('actPass is settings.passAfterAct, persisted in the same object', () => {
    const storage = fakeStorage();
    const p = new SeatPanelState('t1', 1, ctx, storage);
    expect(p.actPass).toBe(true); // casual
    p.setActPass(false);
    expect(p.actPass).toBe(false);
    expect(JSON.parse(storage.getItem('gorge.playsettings.v1') as string).passAfterAct).toBe(false);
  });
});

describe('autopilot — what it will and will not answer', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('auto passes a window where the seat has nothing to do, through the panel’s one post path', async () => {
    const p = armedSeat();
    p.adoptView(quiet(7));
    p.considerAuto(view());
    await settle(() => p.postedSeq === 7);
    expect(postIntentMock).toHaveBeenLastCalledWith('t1', 1, { seq: 7, player: 0, choices: [0] }, ctx);
    expect(autoNoteText(p.note)).toBe('Auto passed 1 priority window.');
  });

  it('auto NEVER posts for a non-priority decision, even one carrying a pass option', () => {
    const p = armedSeat();
    p.adoptView(mulligan(3));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.auto).toBe(true); // it declines the window, it does not stop itself
    expect(autoNoteText(p.note)).toBe('Auto is waiting: this decision needs you, not a pass.');
  });

  it('auto stops at a stop the player set, on the right turn side, and stays armed', () => {
    const p = armedSeat();
    p.stops = { yours: new Set(['main1']), opponents: new Set() };
    p.adoptView(live(4));
    p.considerAuto(view('main1', 0));
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.auto).toBe(true);
    // fb-20260916T225211Z: the note names the play the smart stop stopped for.
    expect(autoNoteText(p.note)).toBe('Auto stopped here: you set a stop on this step and you can act — Cast 0.');
  });

  it('a stop set on your side does not stop auto on the opponent’s turn', async () => {
    const p = armedSeat();
    p.stops = { yours: new Set(['main1']), opponents: new Set() };
    p.adoptView(live(4));
    p.considerAuto(view('main1', 1)); // seat 1 is active, so this is the opponents side
    await settle(() => p.postedSeq === 4);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
  });

  it('a mana tap is not an action: a smart-stop yours.main1 whose only real action is a land drop stops (the c2f4db8f regression, panel level)', () => {
    const p = armedSeat();
    p.stops = { yours: new Set(['main1']), opponents: new Set() };
    p.adoptView(manaLand(9));
    p.considerAuto(view('main1', 0));
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.auto).toBe(true);
    expect(p.active?.seq).toBe(9);
    // fb-20260916T225211Z: the land drop the smart stop caught is named too.
    expect(autoNoteText(p.note)).toBe('Auto stopped here: you set a stop on this step and you can act — Play land 1.');
  });

  it('a hand answer at the stop window keeps Auto armed, and Auto answers the next ordinary window', async () => {
    const p = armedSeat();
    p.stops = { yours: new Set(['main1']), opponents: new Set() };
    p.adoptView(live(4));
    p.considerAuto(view('main1', 0));
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.auto).toBe(true);
    p.click(0); // the answer the stop invited
    await settle(() => p.postedSeq === 4);
    expect(p.auto).toBe(true);
    p.adoptView(quiet(5));
    p.considerAuto(view('main1', 0));
    await settle(() => p.postedSeq === 5);
    expect(postIntentMock).toHaveBeenCalledTimes(2);
  });
});

describe('End Turn — one-shot to the end of the current turn', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('passes a smart main2 stop with a playable card — the step stops are exactly what END TURN ignores', async () => {
    const p = manualSeat();
    // casual's yours.main2 is 'smart': auto would stop for the playable card.
    p.adoptView(live(1));
    p.startEndTurn(view('main2', 0));
    p.considerAuto(view('main2', 0));
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock.mock.calls[0][2].choices).toEqual([1]); // the pass option's index
    expect(p.endTurn).toBe(true);
    expect(p.runPassed).toBe(1);
  });

  it('stops for an opponent instant when respondable, and the run ends', () => {
    const p = manualSeat();
    const v = view('main1', 0, 2, [{ id: 9, controller: 1 }]);
    p.adoptView(live(2));
    p.startEndTurn(v);
    p.considerAuto(v);
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.endTurn).toBe(false);
    expect(autoNoteText(p.note)).toBe("End Turn stopped: an opponent's object is on the stack and you can respond.");
    expect(p.active?.seq).toBe(2); // the window is the player's to answer
  });

  it('clears on turn change: the run does not reach the next turn', async () => {
    const p = manualSeat();
    p.setSkipEmpty(false);
    p.adoptView(quiet(1));
    p.startEndTurn(view('end', 0, 2));
    p.considerAuto(view('end', 0, 2));
    await settle(() => p.postedSeq === 1);
    expect(p.endTurn).toBe(true);

    p.adoptView(quiet(2));
    p.considerAuto(view('untap', 0, 3)); // the next turn
    expect(p.endTurn).toBe(false);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
  });

  it('clears when the step reaches cleanup', () => {
    const p = manualSeat();
    p.adoptView(quiet(1));
    p.startEndTurn(view('end', 0, 2));
    p.considerAuto(view('end', 0, 2));
    expect(p.endTurn).toBe(true);
    p.expireRun(view('cleanup', 0, 2));
    expect(p.endTurn).toBe(false);
  });

  it('clears on any non-priority decision', async () => {
    const p = manualSeat();
    p.setSkipEmpty(false);
    p.adoptView(quiet(1));
    p.startEndTurn(view('main1', 0));
    p.considerAuto(view('main1', 0));
    await settle(() => p.postedSeq === 1);
    expect(p.endTurn).toBe(true);

    p.adoptView(mulligan(2));
    p.considerAuto(view('main1', 0));
    expect(p.endTurn).toBe(false);
    expect(postIntentMock).toHaveBeenCalledTimes(1); // the pre-expiry pass only
    expect(autoNoteText(p.note)).toBe('End Turn stopped: this decision needs you, not a pass.');
  });

  it('still keeps the hard pass cap as the runaway guard', async () => {
    const p = manualSeat();
    p.startEndTurn(view('draw', 0));
    for (let i = 1; i <= AUTO_PASS_CAP; i++) {
      p.adoptView(quiet(i));
      p.considerAuto(view('draw', 0));
      await settle(() => p.postedSeq === i);
    }
    p.adoptView(quiet(AUTO_PASS_CAP + 1));
    p.considerAuto(view('draw', 0));
    expect(postIntentMock).toHaveBeenCalledTimes(AUTO_PASS_CAP);
    expect(p.endTurn).toBe(false);
    expect(autoNoteText(p.note)).toBe(`End Turn stopped: it passed ${AUTO_PASS_CAP} windows in a row.`);
  });

  it('a human option click interrupts the run before posting that click', async () => {
    const p = manualSeat();
    p.startEndTurn(view());
    p.adoptView(live(50));
    p.click(0);
    await settle(() => p.postedSeq === 50);
    expect(p.endTurn).toBe(false);
  });
});

describe('hard skip — everything, including opponent objects (MTGO F6)', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('passes an opponent spell that a plain End Turn would have stopped for', async () => {
    const p = manualSeat();
    const v = view('main1', 0, 2, [{ id: 9, controller: 1, kind: 'spell' }]);
    p.adoptView(live(1));
    p.startHardSkip(v);
    p.considerAuto(v);
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock.mock.calls[0][2].choices).toEqual([1]);
    expect(p.hardSkip).toBe(true);
    expect(autoNoteText(p.note)).toBe('Skipping turn: passed 1 priority window.');
  });

  it('the skip-turn run still stops for a non-priority decision, like End Turn', async () => {
    const p = manualSeat();
    p.setSkipEmpty(false);
    const v = view('main1', 0, 2, [{ id: 9, controller: 1, kind: 'spell' }]);
    p.adoptView(live(1));
    p.startHardSkip(v);
    p.considerAuto(v);
    await settle(() => p.postedSeq === 1);
    expect(p.hardSkip).toBe(true);

    p.adoptView(mulligan(2));
    p.considerAuto(v);
    expect(p.hardSkip).toBe(false);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
  });

  it('Escape ends the hard skip', () => {
    const p = manualSeat();
    p.adoptView(quiet(1));
    p.startHardSkip(view());
    expect(p.hardSkip).toBe(true);
    p.onKeydown('Escape');
    expect(p.hardSkip).toBe(false);
  });

  it('shift-click vs plain press are two different runs on the same button', () => {
    const p = manualSeat();
    p.adoptView(quiet(1));
    p.startEndTurn(view());
    expect(p.endTurn).toBe(true);
    expect(p.hardSkip).toBe(false);
    p.cancelRun();
    p.startHardSkip(view());
    expect(p.hardSkip).toBe(true);
    expect(p.endTurn).toBe(false);
  });
});

describe('the loop guard and the cap pause the machine, never the preference', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('pauses when the same seq comes back after the machine answered it, and the preference survives', async () => {
    const p = armedSeat();
    const d = quiet(11);
    postIntentMock.mockRejectedValue(new Error('stale seq'));
    fetchPendingMock.mockResolvedValue(d);
    p.adoptView(d);
    p.considerAuto(view());
    await settle(() => p.busy === false && p.error !== null);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.machinePaused).toBe(false); // one failure is not yet a loop

    p.considerAuto(view());
    expect(p.machinePaused).toBe(true);
    expect(p.auto).toBe(true); // the PREFERENCE survives the guard
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(autoNoteText(p.note)).toContain('same decision came back');

    // re-arming via the Auto switch clears the brake
    p.setAuto(true);
    expect(p.machinePaused).toBe(false);
  });

  it(`pauses after ${AUTO_PASS_CAP} passes in a row, and posts no more`, async () => {
    const p = armedSeat();
    for (let i = 1; i <= AUTO_PASS_CAP; i++) {
      p.adoptView(quiet(i));
      p.considerAuto(view());
      await settle(() => p.postedSeq === i);
    }
    expect(postIntentMock).toHaveBeenCalledTimes(AUTO_PASS_CAP);
    expect(p.machinePaused).toBe(false);

    p.adoptView(quiet(AUTO_PASS_CAP + 1));
    p.considerAuto(view());
    expect(p.machinePaused).toBe(true);
    expect(p.auto).toBe(true);
    expect(postIntentMock).toHaveBeenCalledTimes(AUTO_PASS_CAP);
  });

  it('a stop resets the run, so a long quiet game never trips the cap', async () => {
    const p = armedSeat();
    for (let i = 1; i <= AUTO_PASS_CAP - 1; i++) {
      p.adoptView(quiet(i));
      p.considerAuto(view('main1', 0, 2, []));
      await settle(() => p.postedSeq === i);
    }
    expect(p.autoRun).toBe(AUTO_PASS_CAP - 1);
    p.stops = { yours: new Set(['draw']), opponents: new Set() };
    p.adoptView(live(500));
    p.considerAuto(view('draw', 0));
    expect(p.autoRun).toBe(0);
    expect(p.auto).toBe(true);
  });
});

describe('the stop-set note names the play (fb-20260916T225211Z)', () => {
  // The Deadly Rollick shape: the player's own main1 smart stop fired on a
  // window whose only real action was one cast option, and the note used to
  // say only "you set a stop on this step" — which read as a contradiction of
  // the "My own spells and abilities: Don't stop" knob the player was looking
  // at. The note must name WHAT the window offered.
  it('the waiting note carries the actionable option labels and the text says you can act', () => {
    const p = armedSeat();
    p.stops = { yours: new Set(['main1']), opponents: new Set() };
    const d = { seq: 1, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [
      { index: 0, kind: 'cast', label: 'Cast Deadly Rollick (alternative cost)', player: 0 },
      pass(1),
      concede(2),
      activate(3),
    ] } as unknown as Decision;
    p.adoptView(d);
    p.considerAuto(view('main1', 0));
    expect(p.note).toEqual({ kind: 'waiting', reason: 'stop-set', detail: 'Cast Deadly Rollick (alternative cost)' });
    expect(autoNoteText(p.note)).toBe(
      'Auto stopped here: you set a stop on this step and you can act — Cast Deadly Rollick (alternative cost).',
    );
  });

  it('a forced stop with nothing to do keeps the base wording (no detail, no dangling clause)', () => {
    const p = armedSeat();
    p.stops = { yours: new Set(['draw']), opponents: new Set() };
    p.settings = { ...p.settings, steps: { ...p.settings.steps, yours: { ...p.settings.steps.yours, draw: 'forced' } } };
    p.adoptView(quiet(1));
    p.considerAuto(view('draw', 0));
    expect(p.note).toEqual({ kind: 'waiting', reason: 'stop-set' });
    expect(autoNoteText(p.note)).toBe('Auto stopped here: you set a stop on this step.');
  });
});

describe('the words — no enum identifier ever reaches the screen', () => {
  it('every reason and state is spoken as plain words, all distinct', () => {
    const reasons: StopReason[] = ['disabled', 'not-priority', 'unexpected-shape', 'stop-set', 'opponent-object', 'own-object'];
    const offs: AutoOffReason[] = ['loop', 'cap'];
    const texts = [
      ...reasons.map((reason) => autoNoteText({ kind: 'waiting', reason })),
      ...offs.map((reason) => autoNoteText({ kind: 'stopped', reason })),
      autoNoteText({ kind: 'off' }),
      autoNoteText({ kind: 'armed' }),
      autoNoteText({ kind: 'passing', count: 6 }),
      autoNoteText({ kind: 'skipped', count: 2 }),
      autoNoteText({ kind: 'act-passed', count: 1 }),
      autoNoteText({ kind: 'end-turn-armed' }),
      autoNoteText({ kind: 'end-turn-passing', count: 3 }),
      ...reasons.map((reason) => autoNoteText({ kind: 'end-turn-stopped', reason })),
      autoNoteText({ kind: 'skip-turn-armed' }),
      autoNoteText({ kind: 'skip-turn-passing', count: 3 }),
      autoNoteText({ kind: 'skip-turn-stopped', reason: 'not-priority' }),
      autoNoteText({ kind: 'skip-turn-stopped', reason: 'cap' }),
    ];
    for (const t of texts) {
      expect(t.length).toBeGreaterThan(0);
      expect(t).not.toMatch(/-/); // no kebab-case enum leaked through
      expect(t[0]).toBe(t[0].toUpperCase());
    }
    expect(new Set(texts).size).toBe(texts.length); // every state says something different
  });
});

// fb-20260914T014141Z: the post-land window. After a land drop the engine's
// float-then-cast payment model leaves the window nothing but a tap-for-mana
// "activate" option, so the old shape test read the window as empty and BOTH
// auto-pass paths (casual's smart stop, the manual empty-window floor) sailed
// past the spell the player was holding. lib/castable's castableAfterTap
// extended actionable(); these tests hold the panel-level wiring on both
// paths: the floor must not swallow the window in manual mode, and auto must
// stop there under casual.
describe('the post-land window — castable after tapping (fb-20260914T014141Z)', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  /** tapOnly is the window after the land drop: one tap-for-mana activate, a pass, a concede. */
  const tapOnly = (seq: number): Decision =>
    ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [activate(0), pass(1), concede(2)] });

  /** postLandView carries seat 0's own hand, pool and availability: one untapped Mountain (Available {R}) and a Lava Spike ({R}). */
  const postLandView = (hand: { mana_cost: string }[] = [{ mana_cost: 'R' }]): View =>
    ({
      active: 0,
      step: 'main1',
      turn: 1,
      stack: [],
      players: [
        {
          seat: 0,
          life: 20,
          hand: hand.map((c, i) => ({ id: 5 + i, name: 'Card', types: 'Instant', controller: 0, owner: 0, ...c })),
          pool: {},
          available: { R: 1 },
        },
      ],
    }) as unknown as View;

  it('manual mode (skipEmpty on): the floor does NOT swallow a tap-only window whose hand is castable after tapping', async () => {
    const p = manualSeat(); // auto off; skipEmpty defaults ON — the manual floor is live
    p.adoptView(tapOnly(3));
    p.considerAuto(postLandView());
    await settle(() => p.busy === false);
    expect(postIntentMock).not.toHaveBeenCalled(); // the window is the player's to answer
    expect(p.emptySkipped).toBe(0); // the floor declined: this window is not "empty"
    expect(p.active?.seq).toBe(3);
  });

  it('manual mode (skipEmpty on): the floor still swallows the same window when the hand is dead mana-wise', async () => {
    const p = manualSeat();
    p.adoptView(tapOnly(4));
    p.considerAuto(postLandView([{ mana_cost: '4 U' }])); // unpayable from one Mountain
    await settle(() => p.postedSeq === 4);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock.mock.calls[0][2].choices).toEqual([1]); // the pass option's index
    expect(p.emptySkipped).toBe(1);
  });

  it('auto (casual): a smart yours.main1 stop catches the post-land window and the note surfaces', async () => {
    const p = armedSeat();
    // The harness's armed seat has every step rule off (the stops setter above
    // wrote them); set yours.main1 to smart — the same rule decide()'s smart
    // branch consumes — exactly like the 'auto stops at a stop' tests do.
    p.stops = { yours: new Set(['main1']), opponents: new Set() };
    p.adoptView(tapOnly(9));
    p.considerAuto(postLandView());
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.auto).toBe(true); // Auto stays armed; the window is the player's
    expect(p.active?.seq).toBe(9);
    // fb-20260916T225211Z: the cast the post-land window's stop caught is
    // named (the float-then-cast shape — the label says "after tapping").
    expect(autoNoteText(p.note)).toBe('Auto stopped here: you set a stop on this step and you can act — Cast Card (after tapping).');
  });
});
