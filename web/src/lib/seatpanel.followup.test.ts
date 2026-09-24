import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Decision, Option } from '../protocol';
import { SeatPanelState, followUpArm } from './seatpanel.svelte';

// fb-20260923T050205Z: the card-follow-up expectation is armed by the HAND
// post (click()'s post-on-click and submit()'s multi-pick commit), wherever
// the click came from — the board tile or the seat panel's own option
// buttons. The panel is the surface the treasure report used, and before this
// fix only the tile path armed it, so the follow-up colour ask fell back to
// the panel's plain button list instead of raising the radial mana wheel.
//
// These tests hold the arming rule down at the state-machine layer (node env,
// hand-built decisions, no DOM): a hand post whose answered options carry an
// `obj` arms followUpExpected with that option's OWN obj (R-E4-1); a machine
// post — passClick/primaryClick, auto, the empty-window floor — arms nothing.
const { postIntentMock, fetchPendingMock } = vi.hoisted(() => ({ postIntentMock: vi.fn(), fetchPendingMock: vi.fn() }));
vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  postIntent: postIntentMock,
  fetchPending: fetchPendingMock,
}));

const ctx = { seat: 0, token: 'tok' };

const pass = (i: number): Option => ({ index: i, kind: 'pass', label: 'Pass priority', player: 0 });
const concede = (i: number): Option => ({ index: i, kind: 'concede', label: 'Concede', player: 0 });

/**
 * treasureWindow is the captured priority decision (capture seq 688): two
 * identical "Activate Treasure Token for mana" options (obj 205 and 207),
 * a pass and a concede. The two activate options carry an `obj` — the
 * battlefield Treasure each would sacrifice — which is exactly the arming
 * case. Qualified as a local builder rather than inline so every test
 * asserts against the same obj pair the fix reads.
 */
const treasureWindow = (seq: number): Decision => ({
  seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1,
  options: [
    { index: 0, kind: 'activate', label: 'Activate Treasure Token for mana', obj: 205, player: 0, cost: 'T Sac<1/CARDNAME>' },
    { index: 1, kind: 'activate', label: 'Activate Treasure Token for mana', obj: 207, player: 0, cost: 'T Sac<1/CARDNAME>' },
    pass(2),
    concede(3),
  ],
});

/** multi is a multi-pick priority window (max > 1), answered with submit(). */
const multi = (seq: number): Decision => ({
  seq, player: 0, kind: 'priority', prompt: 'You have priority.', min: 0, max: 2,
  options: [
    { index: 0, kind: 'ability', label: 'Ability A', obj: 300, player: 0 },
    { index: 1, kind: 'ability', label: 'Ability B', obj: 301, player: 0 },
    pass(2),
    concede(3),
  ],
});

/**
 * arrange is a restable arrange ask (Scry/Surveil/Rearrange): its keep
 * options are the library cards the walker offered, each with an `obj`, and a
 * non-empty `rest` list. Answered by submitArrange().
 */
const arrange = (seq: number): Decision => ({
  seq, player: 0, kind: 'arrange', prompt: 'Arrange the top cards.', min: 0, max: 2, restable: true,
  options: [
    { index: 0, kind: 'card', label: 'Card A', obj: 401, player: 0 },
    { index: 1, kind: 'card', label: 'Card B', obj: 402, player: 0 },
    { index: 2, kind: 'card', label: 'Card C', obj: 403, player: 0 },
  ],
});

async function settle(predicate: () => boolean, maxTicks = 200): Promise<void> {
  for (let i = 0; i < maxTicks; i++) {
    if (predicate()) return;
    await Promise.resolve();
  }
  throw new Error(`settle: condition still false after ${maxTicks} microtask ticks`);
}

describe('the hand post arms the card-follow-up expectation', () => {
  beforeEach(() => {
    postIntentMock.mockReset();
    fetchPendingMock.mockReset();
    postIntentMock.mockResolvedValue(undefined);
  });

  it('followUpArm reads the answered option its OWN obj (R-E4-1), never the decision source', () => {
    // PRECONDITION: the two activate options differ in obj, so which one the
    // rule reads is observable — a rule that rebuilt the obj from the
    // decision source or a list position could not produce 207 here.
    const d = treasureWindow(688);
    expect(d.options[0].obj).not.toBe(d.options[1].obj);
    expect(followUpArm(d, [1])).toEqual({ seq: 688, obj: 207 });
    // The same decision answered on the OTHER treasure arms with 205.
    expect(followUpArm(d, [0])).toEqual({ seq: 688, obj: 205 });
    // An obj-less answer (pass/concede) disarms rather than keeping a stale
    // expectation.
    expect(followUpArm(d, [2])).toBeNull();
    expect(followUpArm(d, [3])).toBeNull();
  });

  it('a panel click that posts a min==max==1 activate arms followUpExpected with that obj', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView(treasureWindow(688));
    expect(p.followUpExpected).toBeNull();
    p.click(1);
    await settle(() => p.postedSeq === 688);
    // The panel click DID post (the precondition for the arm below) and the
    // arm carries the clicked option's obj, 207 — not 205, not the source.
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.followUpExpected).toEqual({ seq: 688, obj: 207 });
  });

  it('a multi-pick hand submit also arms, with the first answered option that carries an obj', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView(multi(710));
    p.click(1);
    p.submit();
    await settle(() => p.postedSeq === 710);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.followUpExpected).toEqual({ seq: 710, obj: 301 });
  });

  it('a restable arrange hand submit (submitArrange) also arms with its first kept option obj', async () => {
    // submitArrange is a hand post too (it runs handAnswer() before posting),
    // and its keep options are the library cards the arrange walker offered —
    // each carries an obj. It must follow the ONE rule rather than carve an
    // exception the next hand-post path would have to rediscover.
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView(arrange(720));
    // PRECONDITION: the two keep options carry DIFFERENT objs, so a rule that
    // rebuilt the obj from the decision source or a list position could not
    // produce 401 here.
    const d = arrange(720);
    expect(d.options[0].obj).not.toBe(d.options[1].obj);
    p.submitArrange([0, 1], [2]);
    await settle(() => p.postedSeq === 720);
    // PRECONDITION: the arrange DID post — an early-return would make the
    // assert below vacuous.
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.followUpExpected).toEqual({ seq: 720, obj: 401 });
  });

  it('a machine passClick arms nothing', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView(treasureWindow(688));
    p.passClick();
    await settle(() => p.postedSeq === 688);
    // PRECONDITION: the machine DID post the pass — a passClick that never
    // posted would make the null arm vacuous.
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.followUpExpected).toBeNull();
  });

  it('a machine primaryClick arms nothing', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView(treasureWindow(688));
    p.primaryClick();
    await settle(() => p.postedSeq === 688);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.followUpExpected).toBeNull();
  });

  it('an unmatched answer clears a previously armed expectation instead of leaving it stale', async () => {
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView(treasureWindow(688));
    p.click(1);
    await settle(() => p.postedSeq === 688);
    expect(p.followUpExpected).toEqual({ seq: 688, obj: 207 });
    // The NEXT hand post answers with an obj-less pass: the expectation must
    // be replaced by null, never carried into an unrelated window.
    p.adoptView(treasureWindow(700));
    p.click(2);
    await settle(() => p.postedSeq === 700);
    expect(p.followUpExpected).toBeNull();
  });
});
