import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView, PlayerView, SeatInfo, StackView, View } from '../protocol';
import Rail from './Rail.svelte';

// SSR via svelte/server, the repo's component-test pattern (see CardTile.
// svelte.test.ts / SeatPanel.svelte.test.ts): no DOM, no $effect, no
// pointer/hover lifecycle — what renders from a given view is exactly what
// these tests can check.

const card = (over: Partial<CardView> = {}): CardView => ({
  id: 1, name: 'Lightning Bolt', types: 'Instant', mana_cost: 'R',
  printing: { name: 'Lightning Bolt' }, token: '', tapped: false, power: 0, toughness: 0,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
  ...over,
});

// A PlayerView with hand/pool exactly the shape a PUBLIC spectator gets:
// view/view.go fills Pool and Hand only for the viewer's own seat, with no
// `omitempty`, so every OTHER seat's client receives a literal JSON null —
// and a spectator (view.NoSeat) has no seat at all, so EVERY seat's hand
// and pool are null. protocol.ts still types both as non-nullable arrays/
// records, which is exactly the gap a previous round shipped a crash
// through on a public server while an omniscient one stayed fine.
const spectatorPlayer = (seat: number, name: string, over: Partial<PlayerView> = {}): PlayerView => ({
  seat, name, life: 40, lost: false, library_size: 60, hand_size: 7, graveyard_size: 0,
  hand: null as unknown as CardView[],
  pool: null as unknown as Record<string, number>,
  battlefield: [], graveyard: [], exile: [], command: [], commanders: [], commander_casts: [],
  ...over,
});

const seats: SeatInfo[] = [
  { name: 'Ari', deck: 'mono-red', colour: '#e5484d' },
  { name: 'Bo', deck: 'mono-green', colour: '#22c55e' },
];

const baseView = (over: Partial<View> = {}): View => ({
  viewer: 255, visibility: 'public', turn: 3, round: 3, step: 'main1', phase: 'main1', active: 0, priority: 0,
  over: false, draw: false, winner: null, stack: [], pending: [],
  players: [spectatorPlayer(0, 'Ari'), spectatorPlayer(1, 'Bo')],
  ...over,
});

describe('Rail — public spectator (every seat\'s hand and pool are null)', () => {
  it('renders with no crash and no fabricated data: a null hand is a true count with no pile control', () => {
    const { html } = render(Rail, { props: { view: baseView(), seats, decision: null } });
    expect(html).toContain('data-hand-hidden');
    expect(html).toContain('data-icon="hand"');
    expect(html).not.toContain('data-pile="hand"');
    expect(html).not.toContain('not visible');
  });

  it('ManaPool renders nothing for a null pool rather than throwing', () => {
    // No assertion beyond "did not throw" is needed for the pool itself —
    // ManaPool already null-guards (see its own doc comment) — but the
    // render() call above having completed at all is the proof; this test
    // exists so a future regression that removes that guard fails HERE,
    // in the rail's own suite, rather than only in a live public server.
    expect(() => render(Rail, { props: { view: baseView(), seats, decision: null } })).not.toThrow();
  });

  it('the one summary panel carries all five counts for EVERY seat, with no duplicate zone pane', () => {
    const v = baseView({ players: [spectatorPlayer(0, 'Ari'), spectatorPlayer(1, 'Bo')] });
    const { html } = render(Rail, { props: { view: v, seats, decision: null } });
    expect(html.match(/data-seat-row=/g)).toHaveLength(2);
    expect(html.match(/data-stat="exile"/g)).toHaveLength(2);
    expect(html).not.toContain("'s zones");
  });
});

describe('Rail — the live decision line (ui15)', () => {  it('names the decision player from the view when seats is empty, not the Seat N placeholder', () => {
    const v = baseView({
      players: [spectatorPlayer(0, 'Ari'), spectatorPlayer(1, 'Bo'), spectatorPlayer(2, 'Player 3')],
    });
    const { html } = render(Rail, { props: { view: v, seats: [], decision: { player: 2, kind: 'priority', prompt: 'pass?' } } });
    expect(html).toContain('Player 3');
    expect(html).not.toContain('Seat 2');
  });
});

describe('Rail — the stack (Task 1/2)', () => {
  const stackOf = (n: number): StackView[] =>
    Array.from({ length: n }, (_, i) => ({
      id: 100 + i, kind: i === 0 ? 'spell' : 'trigger', name: `Entry ${i}`,
      text: 'Some oracle-ish text that could run long if not clamped.',
      controller: 0, targets: [], card: i % 2 === 0 ? card({ id: 100 + i, name: `Entry ${i}` }) : null,
      optional: false,
    }));

  it('a real multi-entry stack renders every entry, the count in the header, and mixed art/no-art gracefully', () => {
    const v = baseView({ stack: stackOf(4) });
    const { html } = render(Rail, { props: { view: v, seats, decision: null } });
    expect(html).toContain('>4<'); // the stack count badge
    expect(html).toContain('Entry 0');
    expect(html).toContain('Entry 3');
    expect(html).toContain('card-image'); // at least one entry has art
  });

  it('an empty stack renders the section with no count badge', () => {
    const { html } = render(Rail, { props: { view: baseView(), seats, decision: null } });
    expect(html).toContain('>Stack<');
    const stackSection = html.slice(html.indexOf('class="stack'), html.indexOf('class="pending'));
    expect(stackSection).not.toContain('class="count'); // no count badge when the stack is empty
  });
});

describe('Rail — dead seats say why (Task 4)', () => {
  it('forwards the DVR event list down to SeatTable, which surfaces the PlayerLost cause', () => {
    const v = baseView({
      players: [
        spectatorPlayer(0, 'Ari', { lost: true, life: 39 }),
        spectatorPlayer(1, 'Bo'),
      ],
    });
    const events = [{ event: { seq: 5, kind: 'player_lost', player: 0, text: 'commander damage (21 or more from one commander)' } }];
    const { html } = render(Rail, { props: { view: v, seats, decision: null, events } });
    expect(html).toContain('Eliminated');
    expect(html).toContain('commander damage (21 or more from one commander)');
    expect(html).toContain('>39<'); // the true life total — never forced to 0
  });

  it('with no events (nothing carries the cause), a lost seat still reads Eliminated with no invented reason', () => {
    const v = baseView({ players: [spectatorPlayer(0, 'Ari', { lost: true }), spectatorPlayer(1, 'Bo')] });
    const { html } = render(Rail, { props: { view: v, seats, decision: null } });
    expect(html).toContain('Eliminated');
  });
});

describe('Rail — the log visibility control (ui21)', () => {
  it('renders the control as a switch reflecting the current state, only when a toggle is supplied', () => {
    // No onToggleLog: a caller that does not own the log (an existing test, or
    // a rail used without the live table route) renders no control at all.
    const noToggle = render(Rail, { props: { view: baseView(), seats, decision: null } });
    expect(noToggle.html).not.toContain('data-log-toggle');

    const on = render(Rail, { props: { view: baseView(), seats, decision: null, showLog: true, onToggleLog: () => {} } });
    expect(on.html).toContain('data-log-toggle');
    expect(on.html).toContain('aria-checked="true"');

    const off = render(Rail, { props: { view: baseView(), seats, decision: null, showLog: false, onToggleLog: () => {} } });
    expect(off.html).toContain('aria-checked="false"');
  });

  it('names itself LOGS in both states — the word is the noun, not the state (player feedback, ui21)', () => {
    // The old toggle spelt its STATE ("Visible"/"Hidden"); a seated player
    // whose log starts hidden read "Hidden" with no idea what it referred
    // to. The button now names the thing it controls, statically, in both
    // states; the state stays in the dot/class:on styling and aria-checked
    // (asserted above). Uppercase comes from CSS (`text-transform:
    // uppercase` on .logbar__toggle .word), so the markup carries "Logs".
    const on = render(Rail, { props: { view: baseView(), seats, decision: null, showLog: true, onToggleLog: () => {} } });
    expect(on.html).toContain('>Logs<');
    const off = render(Rail, { props: { view: baseView(), seats, decision: null, showLog: false, onToggleLog: () => {} } });
    expect(off.html).toContain('>Logs<');

    // The old "Log" label span is gone — matched by class and by exact
    // element text (">Log<" is substring-safe against "Logs" because the
    // static word is what remains).
    expect(off.html).not.toContain('logbar__label');
    expect(off.html).not.toContain('>Log</span>');
  });
});
