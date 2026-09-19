import { describe, expect, it, vi } from 'vitest';
import type { Decision, Option } from '../protocol';
import { SeatPanelState } from './seatpanel.svelte';

// The discard-pick modal's state-machine half (fb-20260918T201739Z): the
// modal is PRESENTATIONAL — its picks go through the panel's EXISTING
// click/submit logic, so the wire intent is byte-identical to the inline
// strip's by construction. These tests pin exactly that: the open flag's
// reset discipline (twin of arrangeOpen's) and the posting path both shapes
// of ask take through the modal.
//
// The seat panel posts through ./api; stub it so this file is hermetic and
// asserts only on the state machine's own gating.
const { postIntentMock, fetchPendingMock } = vi.hoisted(() => ({
  postIntentMock: vi.fn(),
  fetchPendingMock: vi.fn(),
}));
vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  postIntent: postIntentMock,
  fetchPending: fetchPendingMock,
}));

const ctx = { seat: 0, token: 'tok' };
const pick = (index: number, name: string, obj: number): Option =>
  ({ index, kind: 'discard', label: `Discard ${name}`, obj, player: 0 });

/** A Thoughtseize-shaped ask: KModes, Min == Max == 1 (effects/cardflow.go RevealYouChoose). */
const thoughtseize: Decision = {
  seq: 7, player: 0, kind: 'modes', min: 1, max: 1, prompt: 'Choose 1 card(s) to discard',
  options: [
    pick(0, 'Mother of Runes', 21),
    pick(1, 'Brainstorm', 22),
    pick(2, 'Force of Will', 23),
  ],
};

/** A cleanup-step-shaped ask: KChoose, a range over more cards than the count (rules/combat.go cleanupStep). */
const cleanup: Decision = {
  seq: 9, player: 0, kind: 'choose', min: 2, max: 2, prompt: 'discard 2 card(s) down to the hand-size limit',
  options: [
    pick(0, 'Brazen Borrower', 11),
    pick(1, 'Fabled Pass', 12),
    pick(2, 'Gitaxian Probe', 13),
  ],
};

describe('discardOpen — the discard-pick modal’s open flag', () => {
  it('resets across the match boundary with the rest of the seat state (the arrangeOpen contract)', () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.adoptView(thoughtseize);
    p.discardOpen = true;
    p.begin();
    expect(p.discardOpen).toBe(false);
    expect(p.picked).toEqual([]);
  });

  it('closes when a NEW decision is adopted — the modal belongs to one ask, never to the next', () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.adoptView(thoughtseize);
    p.discardOpen = true;

    const next: Decision = { ...thoughtseize, seq: 8 };
    p.adoptView(next);
    expect(p.discardOpen).toBe(false);
    expect(p.picked).toEqual([]);
  });

  it('stays open across a view adoption that carries the SAME decision (a poll/refetch is not a new ask)', () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.adoptView(thoughtseize);
    p.discardOpen = true;
    p.adoptView(thoughtseize);
    expect(p.discardOpen).toBe(true);
  });
});

describe('the modal’s posting path — byte-identical to the inline strip’s', () => {
  it('Min==Max==1 (Thoughtseize): a modal face click posts straight through — the click IS the answer', async () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.adoptView(thoughtseize);
    p.discardOpen = true;
    postIntentMock.mockClear();

    p.click(2); // the modal's onPick wiring: logic.click(option.index)
    await vi.waitFor(() => expect(postIntentMock).toHaveBeenCalledTimes(1));
    const [, , intent] = postIntentMock.mock.calls[0];
    // The exact intent the inline strip's onclick posts for the same face.
    expect(intent).toEqual({ seq: 7, player: 0, choices: [2] });
    // The ask is answered: the pending decision is dropped (the modal's
    // render gate — discard !== null — unmounts it), and the answered seq is
    // recorded so a poll cannot re-adopt it.
    expect(p.pending).toBe(null);
    expect(p.postedSeq).toBe(7);
  });

  it('a range ask (cleanup): modal clicks toggle the shared picked array; submit posts the click order', async () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.adoptView(cleanup);
    p.discardOpen = true;
    postIntentMock.mockClear();

    p.click(2);
    p.click(0);
    expect(p.picked).toEqual([2, 0]); // click order, shared with the strip
    expect(p.canSubmit).toBe(true);

    p.submit(); // the modal's onSubmit wiring: the ordinary submit
    await vi.waitFor(() => expect(postIntentMock).toHaveBeenCalledTimes(1));
    const [, , intent] = postIntentMock.mock.calls[0];
    expect(intent).toEqual({ seq: 9, player: 0, choices: [2, 0] });
  });

  it('the modal’s submit stays gated on the decision’s own min/max — nothing posts short of the ask', async () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.adoptView(cleanup);
    postIntentMock.mockClear();

    p.click(1);
    p.submit(); // one picked, min 2 — refused
    await vi.waitFor(() => expect(postIntentMock).not.toHaveBeenCalled());
    expect(p.canSubmit).toBe(false);
    expect(p.picked).toEqual([1]);
  });
});
