import { describe, expect, it, vi } from 'vitest';
import { render } from 'svelte/server';
import type { Decision, PlayerView, SeatInfo, View } from '../protocol';
import { SeatPanelState } from '../lib/seatpanel.svelte';
import HotButtonStrip from './HotButtonStrip.svelte';

vi.mock('../lib/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/api')>()),
  postIntent: vi.fn(),
  fetchPending: vi.fn(),
}));
vi.mock('../lib/images', () => ({ images: { url: () => new Promise<string | null>(() => {}), offline: () => false } }));

const ctx = { seat: 0, token: 'tok' };
const seats: SeatInfo[] = [{ name: 'Ari', deck: 'deck', colour: '#e5484d' }];
const player: PlayerView = {
  seat: 0, name: 'Ari', life: 20, lost: false, library_size: 53, hand_size: 0,
  graveyard_size: 0, hand: [], battlefield: [], graveyard: [], exile: [], pool: {},
  command: [], commanders: [], commander_casts: [],
};
const baseView: View = {
  viewer: 0, visibility: 'seat', turn: 1, round: 1, step: 'main1', phase: 'main1',
  active: 0, priority: 0, over: false, draw: false, winner: null,
  players: [player], stack: [], pending: [],
};

function strip(decision: Decision): string {
  const state = new SeatPanelState('t1', 1, ctx, null);
  state.skipEmpty = false;
  state.adoptView(decision);
  return render(HotButtonStrip, {
    props: { view: { ...baseView, decision }, seats, state, ctx, table: 't1', match: 1 },
  }).html;
}

/** stripState renders with a caller-prepared state, for cases strip() cannot express (no decision at all, or an answer already posted). */
function stripState(state: SeatPanelState, decision: Decision | null): string {
  return render(HotButtonStrip, {
    props: { view: { ...baseView, decision }, seats, state, ctx, table: 't1', match: 1 },
  }).html;
}

const option = (index: number, kind: string, label: string) => ({ index, kind, label, player: 0 });

describe('HotButtonStrip — server options regrouped into one instrument', () => {
  it('renders exactly five slots, with PASS enabled only by a pass option', () => {
    const priority: Decision = {
      seq: 1, player: 0, kind: 'priority', prompt: 'Priority', min: 1, max: 1,
      options: [option(7, 'cast', 'Cast spell'), option(42, 'pass', 'Pass priority')],
    };
    const html = strip(priority);
    expect([...html.matchAll(/data-hot-tab="/g)]).toHaveLength(5);
    expect(html).toMatch(/data-hot-tab="pass"[^>]*aria-disabled="false"/);
    expect(html).toContain('data-pass-action');

    const choose: Decision = {
      seq: 2, player: 0, kind: 'choose', prompt: 'Choose', min: 0, max: 2,
      options: [option(7, 'choose', 'First'), option(19, 'choose', 'Second')],
    };
    const unavailable = strip(choose);
    expect(unavailable).toMatch(/data-hot-tab="pass"[^>]*aria-disabled="true"/);
    // The button is always present now -- it is a transport control, not a
    // menu that appears only when it has something in it. Unavailable means
    // DISABLED, which is what keeps R-E4-2 true: the strip never offers an
    // action the wire did not send.
    expect(unavailable).toMatch(/data-pass-action[^>]*disabled/);

    // SeatPanel owns polling and Skip Empty. It must remain mounted when the
    // wire offers only pass, even though ACTIONS itself is unavailable.
    const emptyPriority: Decision = {
      ...priority, seq: 3, options: [option(42, 'pass', 'Pass priority')],
    };
    const empty = strip(emptyPriority);
    expect(empty).toMatch(/data-hot-tab="actions"[^>]*aria-disabled="true"/);
    expect(empty).toContain('data-seat-panel');
  });

  it('uses one contextual slot: choose says Done picking and attackers says Done selecting attackers', () => {
    const choose: Decision = {
      seq: 2, player: 0, kind: 'choose', prompt: 'Choose', min: 0, max: 2,
      options: [option(7, 'choose', 'First'), option(19, 'choose', 'Second')],
    };
    const picking = strip(choose);
    expect([...picking.matchAll(/data-hot-tab="done"/g)]).toHaveLength(1);
    expect(picking).toContain('aria-label="Done picking"');
    expect(picking).toContain('DONE_SELECT');

    const attackers: Decision = {
      ...choose, seq: 3, kind: 'attackers', prompt: 'Declare attackers',
      options: [option(7, 'attacker', 'Attack Ari')],
    };
    const attacking = strip(attackers);
    expect([...attacking.matchAll(/data-hot-tab="done"/g)]).toHaveLength(1);
    expect(attacking).toContain('aria-label="Done selecting attackers"');
    expect(attacking).toContain('ATTACK');
  });

  it('does not expose a separate Done submit for a one-pick decision', () => {
    const target: Decision = {
      seq: 4, player: 0, kind: 'target', prompt: 'Choose a target', min: 1, max: 1,
      options: [option(17, 'target', 'Target Ari')],
    };
    const html = strip(target);
    expect(html).toMatch(/data-hot-tab="done"[^>]*aria-disabled="true"/);
    expect(html).toMatch(/data-done-action[^>]*disabled/);
  });

  it('the GAME OPTIONS drop mounts the play-settings editor, which reflects pass-after-acting', () => {
    const priority: Decision = {
      seq: 1, player: 0, kind: 'priority', prompt: 'Priority', min: 1, max: 1,
      options: [option(7, 'cast', 'Cast spell'), option(42, 'pass', 'Pass priority')],
    };
    // casual defaults: passAfterAct is ON, the editor is bound to the panel's
    // settings object, and the old Auto/Skip-empty/stop-grid controls are gone.
    const on = strip(priority);
    expect(on).toContain('data-settings-panel');
    expect(on).toContain('data-actpass-toggle');
    expect(on).toMatch(/aria-checked="true"[^>]*data-actpass-toggle/);
    expect(on).not.toContain('Skip empty windows');
    expect(on).not.toContain('data-stop-grid');

    const state = new SeatPanelState('t1', 1, ctx, null);
    state.skipEmpty = false;
    state.setActPass(false);
    state.adoptView(priority);
    const off = render(HotButtonStrip, {
      props: { view: { ...baseView, decision: priority }, seats, state, ctx, table: 't1', match: 1 },
    }).html;
    expect(off).toMatch(/aria-checked="false"[^>]*data-actpass-toggle/);
  });
});

describe('HotButtonStrip — the ACTIONS tab projects the seat tone', () => {
  // toneOf resolves from option KINDS only (R-E4-1): initiative is a decision
  // with no pass option — the game is blocked on this seat; offered is a
  // window this seat may decline. The tab must carry that state machine-
  // readably in data-awaiting, with the dropdown closed.
  const target: Decision = {
    seq: 5, player: 0, kind: 'target', prompt: 'Choose a target', min: 1, max: 1,
    options: [option(17, 'target', 'Target Ari')],
  };
  const priority: Decision = {
    seq: 6, player: 0, kind: 'priority', prompt: 'Priority', min: 1, max: 1,
    options: [option(7, 'cast', 'Cast spell'), option(42, 'pass', 'Pass priority')],
  };

  it('awaits initiative when the game is blocked on this seat (no pass option)', () => {
    expect(strip(target)).toMatch(/data-awaiting="initiative"/);
  });

  it('awaits offered on a window this seat may act in or pass', () => {
    expect(strip(priority)).toMatch(/data-awaiting="offered"/);
  });

  it('is idle with no pending decision', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    state.skipEmpty = false;
    const html = stripState(state, null);
    expect(html).toMatch(/data-awaiting="idle"/);
    expect(html).not.toMatch(/data-awaiting="initiative"/);
    expect(html).not.toMatch(/data-awaiting="offered"/);
  });

  it('drops the affordance once the answer is posted (answered or in flight)', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    state.skipEmpty = false;
    state.adoptView(target);
    state.postedSeq = target.seq; // logic.active is null while the posted seq matches
    const html = stripState(state, target);
    expect(html).toMatch(/data-awaiting="idle"/);
    expect(html).not.toMatch(/data-awaiting="initiative"/);
  });
});

describe('HotButtonStrip — the status chip', () => {
  const priority: Decision = {
    seq: 1, player: 0, kind: 'priority', prompt: 'Priority', min: 1, max: 1,
    options: [option(7, 'cast', 'Cast spell'), option(42, 'pass', 'Pass priority')],
  };

  it('shows the settings preset by default, machine-readably in data-play-mode', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    const html = stripState(state, priority);
    expect(html).toMatch(/data-play-mode="casual"/);
    expect(html).toContain('Casual');
  });

  it('END TURN names the live run, and the hard skip names its warning', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    state.setActPass(false);
    state.adoptView(priority);
    state.startEndTurn({ ...baseView, decision: priority } as unknown as View);
    const run = stripState(state, priority);
    expect(run).toMatch(/data-play-mode="end-turn"/);
    expect(run).toContain('END TURN');

    state.startHardSkip({ ...baseView, decision: priority } as unknown as View);
    const skip = stripState(state, priority);
    expect(skip).toMatch(/data-play-mode="skip-turn"/);
    expect(skip).toContain('Skipping turn — Esc to stop');
  });

  it('shows Custom once the settings no longer match a named preset', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    state.setActPass(false);
    state.setAuto(false); // casual with autoPass off is no preset
    const html = stripState(state, priority);
    expect(html).toMatch(/data-play-mode="custom"/);
    expect(html).toContain('Custom');
  });
});

// Prio6: the Resolve All transport control — visible only while the stack is
// non-empty AND a priority decision is pending (the two facts that make
// resolving through the stack possible), disabled without a pass option.
describe('HotButtonStrip — Resolve All (prio6)', () => {
  const option = (index: number, kind: string, label: string) => ({ index, kind, label, player: 0 });
  const priorityOnStack: Decision = {
    seq: 1, player: 0, kind: 'priority', prompt: 'Priority', min: 1, max: 1,
    options: [option(42, 'pass', 'Pass priority'), option(43, 'concede', 'Concede')],
  };

  it('is visible with a non-empty stack and a pending priority decision, enabled by a pass option', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    state.skipEmpty = false;
    state.adoptView(priorityOnStack);
    const html = render(HotButtonStrip, {
      props: {
        view: { ...baseView, decision: priorityOnStack, stack: [{ id: 9, controller: 1, kind: 'spell', name: 'Bolt', text: '', targets: [], optional: false }] },
        seats, state, ctx, table: 't1', match: 1,
      },
    }).html;
    expect(html).toContain('data-resolve-all');
    expect(html).toMatch(/data-resolve-all[^>]*aria-disabled="false"/);
  });

  it('is invisible on an empty stack, and on a non-priority decision even with a stack', () => {
    const state = new SeatPanelState('t1', 1, ctx, null);
    state.skipEmpty = false;
    state.adoptView(priorityOnStack);
    const emptyStack = render(HotButtonStrip, {
      props: { view: { ...baseView, decision: priorityOnStack, stack: [] }, seats, state, ctx, table: 't1', match: 1 },
    }).html;
    expect(emptyStack).not.toContain('data-resolve-all');

    const target: Decision = { seq: 2, player: 0, kind: 'target', prompt: 'T', min: 1, max: 1, options: [option(7, 'target', 'T Ari')] };
    state.adoptView(target);
    const nonPriority = render(HotButtonStrip, {
      props: {
        view: { ...baseView, decision: target, stack: [{ id: 9, controller: 1, kind: 'spell', name: 'Bolt', text: '', targets: [], optional: false }] },
        seats, state, ctx, table: 't1', match: 1,
      },
    }).html;
    expect(nonPriority).not.toContain('data-resolve-all');
  });
});
