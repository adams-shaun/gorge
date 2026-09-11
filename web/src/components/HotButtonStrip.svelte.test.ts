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
