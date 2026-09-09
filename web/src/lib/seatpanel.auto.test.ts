import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Decision, Option, View } from '../protocol';
import type { StopReason, Stops } from './autopilot';
import { defaultStops, stopsKey } from './stops';
import { AUTO_PASS_CAP, SeatPanelState, autoNoteText, type AutoOffReason } from './seatpanel.svelte';

// The autopilot LOOP, not decide(). decide() is pure and tested in
// autopilot.test.ts; what these tests hold down is the thing that can lose a
// game in silence — the loop that calls it, posts for the player, and has to
// switch itself off when something goes wrong.
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

/** quiet is a priority window with nothing to do: pass and concede only. */
const quiet = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [pass(0), concede(1)] });

/** live is a priority window where the player has a real action. */
const live = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options: [cast(0), pass(1), concede(2)] });

const mulligan = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'mulligan', prompt: 'Keep?', min: 1, max: 1, options: [pass(0), concede(1)] });

const view = (step = 'draw', active = 0): View =>
  ({ active, step, turn: 2, stack: [] }) as unknown as View;

const NONE: Stops = { yours: new Set(), opponents: new Set() };

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

/** armed is a seat with auto on and no stops set, so only the guards can stop it. */
function armed(storage: Storage | null = null): SeatPanelState {
  const p = new SeatPanelState('t1', 1, ctx, storage);
  p.stops = { yours: new Set(NONE.yours), opponents: new Set(NONE.opponents) };
  p.setAuto(true);
  return p;
}

describe('autopilot — the opt-in', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('auto is OFF on a fresh seat and answers no window that offers an action', () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    expect(p.auto).toBe(false);
    p.adoptView(live(1));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(autoNoteText(p.note)).toBe('Auto is off. You answer every window that offers you something to do.');
  });

  it('the empty-window floor is ON for a fresh seat and passes a no-action window with auto off', async () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    expect(p.skipEmpty).toBe(true);
    p.adoptView(quiet(1));
    p.considerAuto(view());
    await settle(() => p.postedSeq === 1);
    // The index posted is the PASS option's, never the concede's.
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock.mock.calls[0][2].choices).toEqual([0]);
    expect(p.auto).toBe(false); // the floor is not auto and does not turn it on
    expect(p.autoPassed).toBe(0); // and it is never credited to auto
    expect(p.emptySkipped).toBe(1);
    expect(autoNoteText(p.note)).toBe('Passed 1 window where you had nothing to do.');
  });

  it('the floor turned off restores stopping at a no-action window', () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.setSkipEmpty(false);
    p.adoptView(quiet(1));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled();
  });

  it('the floor never answers a non-priority window, even a pass/concede-shaped one', () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.adoptView(mulligan(1));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled();
  });

  it('the floor switches itself off rather than looping when its answer does not take', async () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.adoptView(quiet(7));
    p.considerAuto(view());
    await settle(() => p.postedSeq === 7);
    p.postedSeq = null; // the answer did not take; the same seq comes back
    p.adoptView(quiet(7));
    p.considerAuto(view());
    expect(p.skipEmpty).toBe(false);
    expect(autoNoteText(p.note)).toBe(
      'Auto switched itself off: the same decision came back after it answered. Empty windows are no longer skipped either.',
    );
  });

  it('a new match drops auto back to off', () => {
    const p = armed();
    p.autoPassed = 4;
    p.begin();
    expect(p.auto).toBe(false);
    expect(p.autoPassed).toBe(0);
  });
});

describe('autopilot — what it will and will not answer', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('auto passes a window where the seat has nothing to do, through the panel’s one post path', async () => {
    const p = armed();
    p.adoptView(quiet(7));
    p.considerAuto(view());
    await settle(() => p.postedSeq === 7);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock).toHaveBeenLastCalledWith('t1', 1, { seq: 7, player: 0, choices: [0] }, ctx);
    expect(p.autoPassed).toBe(1);
    expect(autoNoteText(p.note)).toBe('Auto passed 1 priority window.');
  });

  it('auto NEVER posts for a non-priority decision, even one carrying a pass option', () => {
    const p = armed();
    p.adoptView(mulligan(3));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.auto).toBe(true); // it declines the window, it does not switch off
    expect(autoNoteText(p.note)).toBe('Auto is waiting: this decision needs you, not a pass.');
  });

  it('auto stops at a stop the player set, on the right turn side, and stays armed', () => {
    const p = armed();
    p.toggleStop('main1', 'yours');
    p.adoptView(live(4));
    p.considerAuto(view('main1', 0));
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.auto).toBe(true);
    expect(autoNoteText(p.note)).toBe('Auto stopped here: you set a stop on this step.');
  });

  it('a stop set on your side does not stop auto on the opponent’s turn', async () => {
    const p = armed();
    p.toggleStop('main1', 'yours');
    p.adoptView(live(4));
    p.considerAuto(view('main1', 1)); // seat 1 is active, so this is the opponents side
    await settle(() => p.postedSeq === 4);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
  });
});

describe('fast forward — one shot to the next pause point', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('passes repeatedly, then turns itself off on decide’s stop verdict and says why', async () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    // Empty windows ignore stops because there is nothing to do; the next
    // actionable window must honour this dot and end the one-shot run.
    p.stops = { yours: new Set(['draw']), opponents: new Set() };
    p.adoptView(quiet(40));
    p.startFastForward();
    p.considerAuto(view('draw', 0));
    await settle(() => p.postedSeq === 40);
    expect(postIntentMock.mock.calls[0][2].choices).toEqual([0]);
    expect(p.fastForward).toBe(true);

    p.adoptView(live(41));
    p.considerAuto(view('draw', 0));
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.fastForward).toBe(false);
    expect(autoNoteText(p.note)).toBe('Fast forward stopped here: you set a stop on this step.');
  });

  it('restarting at the set stop it just reached acknowledges that one window, then stops at the next stop', async () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.stops = { yours: new Set(['draw']), opponents: new Set() };
    p.adoptView(live(60));

    p.startFastForward();
    p.considerAuto(view('draw', 0));
    expect(p.fastForward).toBe(false);
    expect(postIntentMock).not.toHaveBeenCalled();

    // Starting again on the exact window fast forward handed back means
    // "seen it; continue". It posts that window's own pass index.
    p.startFastForward();
    p.considerAuto(view('draw', 0));
    await settle(() => p.postedSeq === 60);
    expect(postIntentMock.mock.calls[0][2].choices).toEqual([1]);
    expect(p.fastForward).toBe(true);

    // The acknowledgement was consumed by seq 60. The next stopped window
    // is not skipped even though it is the same named step.
    p.adoptView(live(61));
    p.considerAuto(view('draw', 0));
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.fastForward).toBe(false);
    expect(autoNoteText(p.note)).toBe('Fast forward stopped here: you set a stop on this step.');
  });

  it.each([
    ['not-priority', mulligan(70), view('draw', 0)],
    ['unexpected-shape', { ...live(71), min: 0 }, view('draw', 0)],
    ['has-action-and-stack', live(72), ({ ...view('main1', 0), stack: [{ id: 9, controller: 1 }] }) as View],
  ] as const)('never acknowledges a %s safety stop on restart', (reason, decision, currentView) => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.stops = { yours: new Set(), opponents: new Set() };
    p.adoptView(decision);

    p.startFastForward();
    p.considerAuto(currentView);
    expect(p.fastForward).toBe(false);
    expect(autoNoteText(p.note)).toContain(reason === 'not-priority' ? 'needs you' : reason === 'unexpected-shape' ? 'recognise' : 'stack');

    p.startFastForward();
    p.considerAuto(currentView);
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.fastForward).toBe(false);
  });

  it('is bounded by the same hard pass cap as Auto', async () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.stops = { yours: new Set(), opponents: new Set() };
    p.startFastForward();
    for (let i = 1; i <= AUTO_PASS_CAP; i++) {
      p.adoptView(quiet(i));
      p.considerAuto(view());
      await settle(() => p.postedSeq === i);
    }
    p.adoptView(quiet(AUTO_PASS_CAP + 1));
    p.considerAuto(view());
    expect(postIntentMock).toHaveBeenCalledTimes(AUTO_PASS_CAP);
    expect(p.fastForward).toBe(false);
    expect(autoNoteText(p.note)).toBe(`Fast forward switched itself off after ${AUTO_PASS_CAP} passes in a row.`);
  });

  it('a human option click interrupts the run before posting that click', async () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.startFastForward();
    p.adoptView(live(50));
    p.click(0);
    await settle(() => p.postedSeq === 50);
    expect(p.fastForward).toBe(false);
    expect(autoNoteText(p.note)).toBe('Fast forward cancelled: you took the controls.');
  });
});

describe('autopilot — the loop guard', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('disables auto when the same seq comes back after auto answered it', async () => {
    const p = armed();
    const d = quiet(11);
    // the intent is rejected, so postedSeq never advances and the very same
    // decision is still pending: left alone, auto would post it forever
    postIntentMock.mockRejectedValue(new Error('stale seq'));
    fetchPendingMock.mockResolvedValue(d);
    p.adoptView(d);
    p.considerAuto(view());
    await settle(() => p.busy === false && p.error !== null);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.auto).toBe(true); // one failure is not yet a loop
    expect(p.pending?.seq).toBe(11);

    // second look at the same seq: that is the loop, and auto stops itself
    p.considerAuto(view());
    expect(p.auto).toBe(false);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(autoNoteText(p.note)).toBe('Auto switched itself off: the same decision came back after it answered.');
  });
});

describe('autopilot — the consecutive-pass cap', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it(`disables auto after ${AUTO_PASS_CAP} passes in a row, and posts no more`, async () => {
    const p = armed();
    for (let i = 1; i <= AUTO_PASS_CAP; i++) {
      p.adoptView(quiet(i));
      p.considerAuto(view());
      await settle(() => p.postedSeq === i);
    }
    expect(postIntentMock).toHaveBeenCalledTimes(AUTO_PASS_CAP);
    expect(p.auto).toBe(true);

    p.adoptView(quiet(AUTO_PASS_CAP + 1));
    p.considerAuto(view());
    expect(p.auto).toBe(false);
    expect(postIntentMock).toHaveBeenCalledTimes(AUTO_PASS_CAP);
    expect(autoNoteText(p.note)).toBe(`Auto switched itself off after ${AUTO_PASS_CAP} passes in a row.`);
  });

  it('a stop resets the run, so a long quiet game never trips the cap', async () => {
    const p = armed();
    for (let i = 1; i <= AUTO_PASS_CAP - 1; i++) {
      p.adoptView(quiet(i));
      p.considerAuto(view());
      await settle(() => p.postedSeq === i);
    }
    expect(p.autoRun).toBe(AUTO_PASS_CAP - 1);
    p.toggleStop('draw', 'yours');
    p.adoptView(live(500));
    p.considerAuto(view('draw', 0));
    expect(p.autoRun).toBe(0);
    expect(p.auto).toBe(true);
  });
});

describe('autopilot — a human always wins', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('a click on an option suspends auto immediately', async () => {
    const p = armed();
    p.adoptView(live(21));
    p.click(0);
    await settle(() => p.postedSeq === 21);
    expect(p.auto).toBe(false);
    expect(autoNoteText(p.note)).toBe('Auto switched off: you took the decision yourself.');
  });

  it('the primary button, a multi-pick toggle, a submit and a concede confirmation all suspend auto', async () => {
    const byPrimary = armed();
    byPrimary.adoptView(live(22));
    byPrimary.primaryClick();
    await settle(() => byPrimary.postedSeq === 22);
    expect(byPrimary.auto).toBe(false);

    const byToggle = armed();
    byToggle.adoptView({ seq: 23, player: 0, kind: 'choose', prompt: 'pick', min: 1, max: 2, options: [cast(0), cast(1)] });
    byToggle.toggle(0);
    expect(byToggle.auto).toBe(false);

    const bySubmit = armed();
    bySubmit.adoptView({ seq: 24, player: 0, kind: 'choose', prompt: 'pick', min: 0, max: 2, options: [cast(0), cast(1)] });
    bySubmit.submit();
    await settle(() => bySubmit.postedSeq === 24);
    expect(bySubmit.auto).toBe(false);

    const byConcede = armed();
    byConcede.adoptView(live(25));
    byConcede.click(2); // arms the confirmation
    expect(byConcede.auto).toBe(false);
  });

  it('Escape suspends auto, and no other key does', () => {
    const p = armed();
    p.onKeydown('a');
    p.onKeydown('Enter');
    p.onKeydown(' ');
    expect(p.auto).toBe(true);
    p.onKeydown('Escape');
    expect(p.auto).toBe(false);
    expect(autoNoteText(p.note)).toBe('Auto switched off: you pressed Escape.');
  });

  it('a suspended auto answers nothing further where the player had an action', () => {
    const p = armed();
    p.onKeydown('Escape');
    p.adoptView(live(30));
    p.considerAuto(view());
    expect(postIntentMock).not.toHaveBeenCalled();
  });

  // Escape takes back the DECISIONS, not the courtesy: a suspended auto still
  // leaves the empty-window floor running, because the floor never decided
  // anything for the player in the first place. Turning it off is its own
  // switch.
  it('a suspended auto still skips a window with nothing in it', async () => {
    const p = armed();
    p.onKeydown('Escape');
    expect(p.auto).toBe(false);
    p.adoptView(quiet(30));
    p.considerAuto(view());
    await settle(() => p.postedSeq === 30);
    expect(p.emptySkipped).toBe(1);
    expect(p.autoPassed).toBe(0);
  });
});

describe('autopilot — stops persistence and words', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('mountStops loads this table+seat’s saved set and toggleStop saves it back', () => {
    const storage = fakeStorage();
    const p = new SeatPanelState('t1', 1, ctx, storage);
    p.mountStops();
    expect([...p.stops.yours].sort()).toEqual([...defaultStops().yours].sort());

    p.toggleStop('upkeep', 'opponents');
    const raw = storage.getItem(stopsKey('t1', 0));
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw as string).opponents).toContain('upkeep');

    // a second seat at the same table does not read the first seat's set
    const other = new SeatPanelState('t1', 1, { seat: 1, token: 'tok' }, storage);
    other.mountStops();
    expect(other.stops.opponents.has('upkeep')).toBe(false);
  });

  it('toggleStop refuses untap and cleanup — the two steps that grant no priority', () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.stops = { yours: new Set(), opponents: new Set() };
    p.toggleStop('untap', 'yours');
    p.toggleStop('cleanup', 'opponents');
    expect(p.stops.yours.size).toBe(0);
    expect(p.stops.opponents.size).toBe(0);
  });

  it('every reason is spoken as plain words — no enum identifier ever reaches the screen', () => {
    const reasons: StopReason[] = ['disabled', 'not-priority', 'unexpected-shape', 'stop-set', 'has-action-and-stack'];
    const offs: AutoOffReason[] = ['loop', 'cap', 'human', 'escape'];
    const texts = [
      ...reasons.map((reason) => autoNoteText({ kind: 'waiting', reason })),
      ...offs.map((reason) => autoNoteText({ kind: 'stopped', reason })),
      autoNoteText({ kind: 'off' }),
      autoNoteText({ kind: 'armed' }),
      autoNoteText({ kind: 'passing', count: 6 }),
    ];
    for (const t of texts) {
      expect(t.length).toBeGreaterThan(0);
      expect(t).not.toMatch(/-/); // no kebab-case enum leaked through
      expect(t[0]).toBe(t[0].toUpperCase());
    }
    expect(autoNoteText({ kind: 'passing', count: 6 })).toBe('Auto passed 6 priority windows.');
    expect(new Set(texts).size).toBe(texts.length); // every state says something different
  });
});
