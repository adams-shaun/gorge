import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Decision, Intent, Option } from '../protocol';
import { ApiError } from './api';
import { SeatPanelState, primaryOf, isConcede } from './seatpanel.svelte';

// seatpanel.svelte.ts posts via ./api (postIntent) and refreshes the
// pending decision via ./api (fetchPending); stub those two so the state
// machine is hermetic — the real ApiError class comes through so a 409
// rejection is a real one.
const { postIntentMock, fetchPendingMock } = vi.hoisted(() => ({ postIntentMock: vi.fn(), fetchPendingMock: vi.fn() }));
vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  postIntent: postIntentMock,
  fetchPending: fetchPendingMock,
}));

const ctx = { seat: 0, token: 'tok' };

const cast = (i: number): Option => ({ index: i, kind: 'cast', label: `Cast Spell ${i}`, obj: undefined, player: 0 });
const pass = (i: number): Option => ({ index: i, kind: 'pass', label: 'Pass priority', obj: undefined, player: 0 });
const concede = (i: number): Option => ({ index: i, kind: 'concede', label: 'Concede', obj: undefined, player: 0 });

const priority = (seq: number, options: Option[]): Decision =>
  ({ seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1, options });

async function settle(predicate: () => boolean, maxTicks = 200): Promise<void> {  for (let i = 0; i < maxTicks; i++) {
    if (predicate()) return;
    await Promise.resolve();
  }
  throw new Error(`settle: condition still false after ${maxTicks} microtask ticks`);
}

async function drain(ticks = 40): Promise<void> {
  for (let i = 0; i < ticks; i++) await Promise.resolve();
}

describe('SeatPanelState', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  // Regression: the panel used to learn about a new decision ONLY from
  // view.decision, refreshed by the SSE 'decision' frame, so the stream was a
  // single point of failure — miss one frame and the panel sat empty forever
  // while the server held a decision for this seat. Seen in the first real
  // game played through this client. refreshPending must recover on its own,
  // with adoptView never called again.
  it('recovers a decision the stream never delivered: refreshPending alone picks up the next ask', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView(priority(5, [cast(0), pass(1), concede(2)]));
    p.click(1);
    await settle(() => p.postedSeq === 5);
    expect(p.pending).toBeNull();

    // The stream is dead from here on: no further adoptView. The server has
    // moved on and only /pending knows it.
    fetchPendingMock.mockResolvedValue(priority(9, [cast(0), pass(1), concede(2)]));
    await p.refreshPending();
    await settle(() => p.pending?.seq === 9);
    expect(p.pending?.seq).toBe(9);

    // and it is answerable, not merely displayed
    p.click(1);
    await settle(() => p.postedSeq === 9);
    expect(postIntentMock).toHaveBeenLastCalledWith('t1', 1, { seq: 9, player: 0, choices: [1] }, ctx);
  });

  // The poll must not re-open a decision this seat already answered: while the
  // server still reports the answered ask, /pending returns it again.
  it('a poll that returns the decision just answered does not re-open it', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    const d = priority(5, [cast(0), pass(1), concede(2)]);
    p.adoptView(d);
    p.click(1);
    await settle(() => p.postedSeq === 5);

    fetchPendingMock.mockResolvedValue(d);
    await p.refreshPending();
    await drain();
    expect(p.pending).toBeNull();
  });

  it('test 1 — the panel posts the option the user picked: its index and the decision seq', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    const d = priority(5, [cast(0), pass(1), concede(2)]);
    p.adoptView(d);
    expect(p.pending).toEqual(d);

    p.click(0); // the picked option
    await settle(() => p.postedSeq === 5);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock).toHaveBeenCalledWith('t1', 1, { seq: 5, player: 0, choices: [0] }, ctx);
  });

  it('test 2 — R-E4-1: the primary button resolves the pass option by kind, never the last option, never concede', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    const d = priority(5, [cast(0), pass(1), concede(2)]);
    expect(primaryOf(d)).not.toBeNull();
    p.adoptView(d);

    // the decision's LAST option is concede: a primary resolved by position
    // would be concede. It must be the pass option, found by kind.
    expect(p.primary()).toEqual(pass(1));
    expect(p.primary()?.kind).toBe('pass');
    expect(p.primary()?.index).not.toBe(d.options[d.options.length - 1].index);

    p.primaryClick();
    await settle(() => p.postedSeq === 5);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock).toHaveBeenCalledWith('t1', 1, { seq: 5, player: 0, choices: [1] }, ctx);
    // no intent ever carries the concede index without the confirmation step
    for (const call of postIntentMock.mock.calls) {
      expect((call[2] as Intent).choices).not.toContain(2);
    }
  });

  it('test 2 — a decision without a pass/resolve option has no primary action at all', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView({ seq: 5, player: 0, kind: 'choose', prompt: 'Choose a card', min: 1, max: 1,
      options: [
        { index: 0, kind: 'exile', label: 'Exile A', obj: undefined, player: 0 },
        { index: 1, kind: 'exile', label: 'Exile B', obj: undefined, player: 0 },
      ] });
    expect(p.primary()).toBeNull();
    p.primaryClick();
    await drain();
    expect(postIntentMock).not.toHaveBeenCalled();
  });

  it('test 3 — R-E4-1: concede needs a second, explicit confirmation before its intent is posted', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView(priority(5, [cast(0), pass(1), concede(2)]));

    p.click(2); // first click arms the confirmation, posts nothing
    await drain();
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.confirming).toBe(true);

    p.click(2); // the second, explicit click posts
    await settle(() => p.postedSeq === 5);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock).toHaveBeenCalledWith('t1', 1, { seq: 5, player: 0, choices: [2] }, ctx);
  });

  it('test 3 — the confirm button path posts; arming then picking another option cancels it', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView(priority(5, [cast(0), pass(1), concede(2)]));

    p.click(2); // arm
    p.click(0); // changed mind — picks a cast instead: confirmation gone, and the cast posts (min==max==1)
    await settle(() => p.postedSeq === 5);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock).toHaveBeenCalledWith('t1', 1, { seq: 5, player: 0, choices: [0] }, ctx);
    expect(p.confirming).toBe(false);

    const q = new SeatPanelState('t1', 1, ctx);
    q.adoptView(priority(6, [cast(0), pass(1), concede(2)]));
    q.click(2);
    q.confirmConcede();
    await settle(() => q.postedSeq === 6);
    expect(postIntentMock).toHaveBeenLastCalledWith('t1', 1, { seq: 6, player: 0, choices: [2] }, ctx);
  });

  it('test 4 — a stale seq is surfaced, not swallowed, and the panel recovers onto the current decision', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    const d5 = priority(5, [cast(0), pass(1), concede(2)]);
    const d7 = priority(7, [cast(0), pass(1), concede(2)]);
    p.adoptView(d5);

    postIntentMock.mockRejectedValueOnce(new ApiError(409, 'conflict', 'intent seq 5, pending decision seq 7'));
    fetchPendingMock.mockResolvedValue(d7);
    p.click(0);
    await settle(() => p.error !== null);
    expect(p.error).toBe('intent seq 5, pending decision seq 7'); // surfaced, with the server's own reason

    await settle(() => p.pending?.seq === 7); // recovered: the current decision replaced the stale one
    expect(fetchPendingMock).toHaveBeenCalledWith('t1', 1, ctx);
    expect(p.postedSeq).toBeNull();

    postIntentMock.mockResolvedValue(undefined); // the wedged seat answers again
    p.adoptView(d7); // the refreshed view carries the same seq — adoption is a no-op, nothing is re-answered
    p.click(0);
    await settle(() => p.postedSeq === 7);
    expect(postIntentMock).toHaveBeenLastCalledWith('t1', 1, { seq: 7, player: 0, choices: [0] }, ctx);
  });

  it('test 6 — min/max gate the submit button, and a server rejection is still handled', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    const d: Decision = { seq: 9, player: 0, kind: 'choose', prompt: 'Bottom 2 cards', min: 2, max: 2,
      options: [
        { index: 0, kind: 'bottom', label: 'Card A', obj: undefined, player: 0 },
        { index: 1, kind: 'bottom', label: 'Card B', obj: undefined, player: 0 },
        { index: 2, kind: 'bottom', label: 'Card C', obj: undefined, player: 0 },
      ] };
    p.adoptView(d);

    expect(p.canSubmit).toBe(false); // zero of min 2
    p.submit();
    await drain();
    expect(postIntentMock).not.toHaveBeenCalled();

    p.click(0); // one of two: still gated
    expect(p.picked).toEqual([0]);
    expect(p.canSubmit).toBe(false);
    p.submit();
    await drain();
    expect(postIntentMock).not.toHaveBeenCalled();

    p.click(1); // two: now submittable, click order preserved in the intent
    expect(p.canSubmit).toBe(true);
    p.submit();
    await settle(() => p.postedSeq === 9);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock).toHaveBeenCalledWith('t1', 1, { seq: 9, player: 0, choices: [0, 1] }, ctx);

    // an over-full selection cannot be submitted either
    const q = new SeatPanelState('t1', 1, ctx);
    q.adoptView(d);
    q.click(0); q.click(1); q.click(2);
    expect(q.picked).toEqual([0, 1, 2]);
    expect(q.canSubmit).toBe(false);
    q.click(1); // deselect one
    expect(q.canSubmit).toBe(true);

    // a rejection on a multi-pick decision is surfaced and recovered
    postIntentMock.mockRejectedValueOnce(new ApiError(409, 'conflict', 'duplicate choice 1'));
    fetchPendingMock.mockResolvedValue({ ...d, seq: 10 });
    const r = new SeatPanelState('t1', 1, ctx);
    r.adoptView(d);
    r.click(0); r.click(1);
    r.submit();
    await settle(() => r.error !== null);
    expect(r.error).toBe('duplicate choice 1');
    await settle(() => r.pending?.seq === 10);
    expect(r.canSubmit).toBe(false); // fresh slate for the current decision
  });

  it('a 409 from refreshPending with nothing pending is waiting, not an error', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    fetchPendingMock.mockRejectedValueOnce(new ApiError(409, 'conflict', 'no decision pending for this seat'));
    await p.refreshPending();
    expect(p.pending).toBeNull();
    expect(p.error).toBeNull();
  });

  it('adoptView ignores a re-adopted decision that was already answered (stale view must not reopen options)', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    const d = priority(5, [cast(0), pass(1), concede(2)]);
    p.adoptView(d);
    p.click(0);
    await settle(() => p.postedSeq === 5);
    p.adoptView(d); // a stale refresh still carrying the answered seq
    expect(p.pending).toBeNull();
    expect(p.postedSeq).toBe(5);
  });

  it('isConcede names the kind only', () => {
    expect(isConcede(concede(2))).toBe(true);
    expect(isConcede(pass(1))).toBe(false);
  });
});
