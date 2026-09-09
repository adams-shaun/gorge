import { describe, expect, it, vi } from 'vitest';
import type { Decision, Option } from '../protocol';
import { SeatPanelState } from './seatpanel.svelte';

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

const attack = (index: number, obj: number): Option => ({
  index,
  kind: 'attacker',
  label: `Attack with creature ${obj}`,
  obj,
  player: 1,
});

/**
 * attackers builds a declare-attackers decision the way rules.askAttackers
 * does: Min 0 (declining to attack is always legal, CR 508.1a), Max the
 * number of (attacker, defender) pairs offered, and EVERY option of kind
 * "attacker" — there is no `pass` and no `resolve` on the wire, so the
 * panel's primary-by-kind button (R-E4-1) never appears for one.
 */
const attackers = (n: number): Decision => ({
  seq: 7,
  player: 0,
  kind: 'attackers',
  prompt: 'turn 3 — declare attackers',
  min: 0,
  max: n,
  options: Array.from({ length: n }, (_, i) => attack(i, 100 + i)),
});

describe('showSubmit — a decision a click cannot answer must offer a way to commit', () => {
  // The deadlock this file exists for. A 1v1 board with exactly one creature
  // able to attack produces Min 0 / Max 1: clicking the creature does NOT
  // post (post-on-click is reserved for min==max==1, where the click IS the
  // answer), so the pick sits in `picked` waiting for a submit button that
  // the old `d.max > 1` gate never rendered. The seat could select its
  // attacker and then had no way to declare it OR to decline and move on —
  // the game simply stopped for that player. Attacking with two creatures
  // worked, which is why this survived: the bug is invisible at max >= 2.
  it('renders submit for a single-option Min 0 decision (the one-attacker deadlock)', () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.adoptView(attackers(1));

    expect(p.primary()).toBeNull(); // no pass/resolve on the wire to fall back on
    p.click(0);
    expect(p.picked).toEqual([0]); // the click selected rather than posted
    expect(postIntentMock).not.toHaveBeenCalled();

    expect(p.showSubmit).toBe(true);
    expect(p.canSubmit).toBe(true);
  });

  // The same decision with nothing picked: declining to attack is a legal
  // answer (Min 0), so the button must be there and enabled BEFORE any
  // click, or a seat that wants to skip combat is just as stuck.
  it('offers submit before any pick, so declining is reachable', () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.adoptView(attackers(1));

    expect(p.showSubmit).toBe(true);
    expect(p.canSubmit).toBe(true);
    expect(p.picked).toEqual([]);
  });

  // Unchanged behaviour, asserted so the fix cannot widen into the priority
  // window: min==max==1 is the shape where the click IS the answer, and a
  // submit button there would be a second, redundant control.
  it('does not render submit for a min==max==1 decision', () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.adoptView({
      seq: 8,
      player: 0,
      kind: 'priority',
      prompt: 'You have priority.',
      min: 1,
      max: 1,
      options: [{ index: 0, kind: 'pass', label: 'Pass priority', obj: undefined, player: 0 }],
    });
    expect(p.showSubmit).toBe(false);
  });

  // The original multi-pick case still renders it.
  it('still renders submit for a multi-option decision', () => {
    const p = new SeatPanelState('t1', 1, ctx, null);
    p.adoptView(attackers(3));
    expect(p.showSubmit).toBe(true);
  });
});
