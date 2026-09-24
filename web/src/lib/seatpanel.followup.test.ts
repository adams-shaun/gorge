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
// hand-built decisions, no DOM): a hand post whose answered option is an
// `activate` or a `mana` (the only two answers the engine follows with a
// same-object mana ask -- see MANA_FOLLOW_UP_KINDS) arms followUpExpected with
// that option's OWN obj (R-E4-1), whatever the decision kind; every other
// object-bearing answer (ability, target, arrange, multi-pick sacrifice)
// disarms; a machine post -- passClick/primaryClick -- arms nothing.
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

/** multi is a multi-pick priority window (max > 1) of object-bearing `ability` options, answered with submit(). */
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
 * stageOne is the multi-ability mana source's stage-1 ability pick as
 * rules/mana_activation.go activateManaFor poses it (fb-e079def5; pinned
 * engine-side by rules TestManaProducedAnyStageOneLabel): a `choose`
 * decision whose options are Kind "mana" on the source. Answering the Any
 * ability poses askManaColor's five-colour stage-2 wheel on the SAME source
 * (Vivid Marsh: {T}: Add {B} / {T}, remove a charge counter: Add one mana of
 * any color).
 */
const stageOne = (seq: number): Decision => ({
  seq, player: 0, kind: 'choose', prompt: 'Choose a mana ability of Vivid Marsh', min: 1, max: 1, source: 512,
  options: [
    { index: 0, kind: 'mana', label: 'Add B', obj: 512, player: 0 },
    { index: 1, kind: 'mana', label: 'Add any color', obj: 512, player: 0 },
  ],
});

/**
 * castWindow is the CR 601.2g mid-cast mana window (rules/cast.go
 * manaWindowAsk): a `choose` decision with one `activate` option per
 * untapped mana source and a `done`. Answering an activate enters the same
 * activateManaFor as the priority window, so a Treasure activated here
 * poses the same colour ask.
 */
const castWindow = (seq: number): Decision => ({
  seq, player: 0, kind: 'choose', prompt: 'Activate mana abilities to pay for Lightning Bolt', min: 1, max: 1, source: 90,
  options: [
    { index: 0, kind: 'activate', label: 'Activate Treasure Token for mana', obj: 207, player: 0, cost: 'T Sac<1/CARDNAME>' },
    { index: 1, kind: 'done', label: 'Done', player: 0 },
  ],
});

/** sacrifice is a multi-pick cost choose whose options carry objs (Kind "sacrifice"). */
const sacrifice = (seq: number): Decision => ({
  seq, player: 0, kind: 'choose', prompt: 'Choose 2 permanents to sacrifice', min: 2, max: 2, source: 90,
  options: [
    { index: 0, kind: 'sacrifice', label: 'Treasure Token', obj: 205, player: 0 },
    { index: 1, kind: 'sacrifice', label: 'Treasure Token', obj: 207, player: 0 },
    { index: 2, kind: 'sacrifice', label: 'Memnite', obj: 210, player: 0 },
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

  it('a panel click on the stage-1 mana-ability pick (choose/mana) arms for the stage-2 wheel', async () => {
    // The round-2 regression: a priority-only rule returned null here, so a
    // Vivid Marsh's "Add any color" stage-2 ask fell back to plain buttons.
    const d = stageOne(400);
    // PRECONDITION: this is NOT a priority decision and the option is NOT an
    // activate -- the shapes a priority/activate-only rule would accept.
    expect(d.kind).toBe('choose');
    expect(d.options[1].kind).toBe('mana');
    expect(followUpArm(d, [1])).toEqual({ seq: 400, obj: 512 });
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView(stageOne(400));
    p.click(1);
    await settle(() => p.postedSeq === 400);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.followUpExpected).toEqual({ seq: 400, obj: 512 });
  });

  it('an activate answered in the mid-cast mana window (a choose decision) arms', async () => {
    const d = castWindow(820);
    expect(d.kind).not.toBe('priority');
    expect(followUpArm(d, [0])).toEqual({ seq: 820, obj: 207 });
    expect(followUpArm(d, [1])).toBeNull();
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView(castWindow(820));
    p.click(0);
    await settle(() => p.postedSeq === 820);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.followUpExpected).toEqual({ seq: 820, obj: 207 });
  });

  it('a panel click on an object-bearing priority ability does not arm', async () => {
    // An `ability` goes on the stack: mana abilities are offered only as
    // `activate`, so no ability answer is followed by a same-object mana ask.
    const single: Decision = { ...multi(705), min: 1, max: 1 };
    expect(single.options[1].obj).toBe(301);
    expect(followUpArm(single, [1])).toBeNull();
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView(single);
    p.click(1);
    await settle(() => p.postedSeq === 705);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.followUpExpected).toBeNull();
  });

  it('a multi-pick hand submit of object-bearing answers does not arm', async () => {
    for (const [d, picks] of [[multi(710), [0, 1]], [sacrifice(711), [0, 1]]] as const) {
      // PRECONDITION: every picked option carries an obj, so a rule that
      // armed on "any obj" would arm here.
      for (const i of picks) expect(d.options[i].obj).toBeDefined();
      postIntentMock.mockClear();
      const p = new SeatPanelState('t1', 1, ctx);
      p.adoptView(d);
      for (const i of picks) p.click(i);
      p.submit();
      await settle(() => p.postedSeq === d.seq);
      expect(postIntentMock).toHaveBeenCalledTimes(1);
      expect(p.followUpExpected).toBeNull();
    }
  });

  it('object-bearing target and arrange answers do not arm a mana follow-up', async () => {
    // The engine's target candidates are Kind "permanent" (objects) and
    // "player" (seats); neither is ever followed by a same-object mana ask.
    const target: Decision = {
      seq: 719, player: 0, kind: 'target', prompt: 'Choose a target.', min: 1, max: 1,
      options: [
        { index: 0, kind: 'permanent', label: 'Treasure Token', obj: 207, player: 0 },
        { index: 1, kind: 'player', label: 'Opponent', player: 1 },
      ],
    };
    const arranged = arrange(720);
    expect(target.options[0].obj).toBe(207);
    expect(arranged.options[0].obj).not.toBe(arranged.options[1].obj);
    expect(followUpArm(target, [0])).toBeNull();
    expect(followUpArm(arranged, [0])).toBeNull();
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView(target);
    p.click(0);
    await settle(() => p.postedSeq === 719);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.followUpExpected).toBeNull();
  });

  it('a restable arrange hand submit posts but does not arm a mana follow-up', async () => {
    // submitArrange is a hand post, and its keep options carry objs; its
    // decision kind is not an activation chain starter, so it must not arm.
    const p = new SeatPanelState('t1', 1, ctx);
    p.adoptView(arrange(720));
    const d = arrange(720);
    expect(d.options[0].obj).not.toBe(d.options[1].obj);
    p.submitArrange([0, 1], [2]);
    await settle(() => p.postedSeq === 720);
    // PRECONDITION: the arrange DID post — an early-return would make the
    // assert below vacuous.
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(p.followUpExpected).toBeNull();
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
