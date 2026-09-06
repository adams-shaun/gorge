import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { SeatInfo, View } from '../protocol';
import type { Stops } from '../lib/autopilot';
import { STEPS } from '../lib/autopilot';
import { SeatPanelState } from '../lib/seatpanel.svelte';
import PhaseTrack from './PhaseTrack.svelte';

// SSR via svelte/server, the repo's component-test pattern. There is no DOM
// here, so an interaction is exercised the way the app actually performs it:
// the click handler calls SeatPanelState.toggleStop, and the next render is
// the component's answer to the new stop set. The toggle under test is the
// real one, not a stand-in.

const seats: SeatInfo[] = [
  { name: 'alice', deck: 'burn', colour: '#e5484d' },
  { name: 'bob', deck: 'stompy', colour: '#30a46c' },
];

const view = (active: number, step: string, turn = 4): View =>
  ({
    viewer: 1, visibility: 'seat', turn, step, phase: '', active, priority: active,
    over: false, draw: false, winner: null, players: [], stack: [], pending: [],
  }) as unknown as View;

const stops = (yours: string[], opponents: string[]): Stops =>
  ({ yours: new Set(yours), opponents: new Set(opponents) });

/** cell returns the opening tag of the track cell for one wire step. */
function cell(html: string, step: string): string {
  const m = new RegExp(`<(button|span)[^>]*data-step="${step}"[^>]*>`).exec(html);
  if (m === null) throw new Error(`no cell for step ${step}`);
  return m[0];
}

type TrackProps = {
  view: View;
  seats: SeatInfo[];
  seat?: number | null;
  stops?: Stops | null;
  onToggle?: ((step: string, side: 'yours' | 'opponents') => void) | null;
};
const track = (props: TrackProps) => render(PhaseTrack, { props }).html;

const seated = (v: View, st: Stops) =>
  track({ view: v, seats, seat: 1, stops: st, onToggle: () => {} });

describe('PhaseTrack — the clock', () => {
  it('draws all twelve wire steps, in engine order, grouped into the five phases', () => {
    const html = seated(view(1, 'main1'), stops([], []));
    const order = [...html.matchAll(/data-step="([^"]+)"/g)].map((m) => m[1]);
    expect(order).toEqual([...STEPS]);
    const groups = [...html.matchAll(/data-group="([^"]+)"/g)].map((m) => m[1]);
    expect(groups).toEqual(['beginning', 'main1', 'combat', 'main2', 'ending']);
  });

  it('marks the current step, and only the current step', () => {
    const html = seated(view(1, 'declare-blockers'), stops([], []));
    expect(cell(html, 'declare-blockers')).toContain('aria-current="step"');
    expect([...html.matchAll(/aria-current="step"/g)]).toHaveLength(1);
    for (const s of STEPS) {
      if (s !== 'declare-blockers') expect(cell(html, s)).not.toContain('aria-current');
    }
  });

  it('names whose turn it is in that seat’s identity colour, and says "Your turn" for the viewer', () => {
    const mine = seated(view(1, 'upkeep'), stops([], []));
    expect(mine).toContain('Your turn');
    expect(mine).toContain('--seat:#30a46c'); // seat 1 is the viewer and is active
    const theirs = seated(view(0, 'upkeep'), stops([], []));
    expect(theirs).toContain('alice’s turn');
    expect(theirs).not.toContain('Your turn');
    expect(theirs).toContain('--seat:#e5484d');
  });

  it('states the modifier rather than hiding it', () => {
    const html = seated(view(1, 'upkeep'), stops([], []));
    expect(html).toContain('data-hint');
    expect(html).toMatch(/Shift-click/);
  });
});

describe('PhaseTrack — stops', () => {
  it('untap and cleanup are inert: not buttons, not focusable, and carry no stop state', () => {
    const html = seated(view(1, 'upkeep'), stops([], []));
    for (const step of ['untap', 'cleanup']) {
      const tag = cell(html, step);
      expect(tag.startsWith('<span')).toBe(true);
      expect(tag).toContain('data-inert');
      expect(tag).not.toContain('aria-pressed');
      expect(tag).not.toContain('tabindex');
    }
    // and every other step IS a button, so the inertness is specific
    for (const step of STEPS.filter((s) => s !== 'untap' && s !== 'cleanup')) {
      expect(cell(html, step).startsWith('<button')).toBe(true);
    }
  });

  it('a settable cell carries aria-pressed, and toggling one through the seat state flips it', () => {
    const p = new SeatPanelState('t1', 1, { seat: 1, token: 'tok' }, null);
    p.stops = stops([], []);
    const v = view(1, 'upkeep');

    const before = track({ view: v, seats, seat: 1, stops: p.stops, onToggle: (s: string, side: 'yours' | 'opponents') => p.toggleStop(s, side) });
    expect(cell(before, 'main1')).toContain('aria-pressed="false"');

    // exactly what the click handler does: the cell's step, on the current
    // turn side (it is the viewer's turn here)
    p.toggleStop('main1', 'yours');
    const after = track({ view: v, seats, seat: 1, stops: p.stops, onToggle: () => {} });
    expect(cell(after, 'main1')).toContain('aria-pressed="true"');

    p.toggleStop('main1', 'yours');
    const off = track({ view: v, seats, seat: 1, stops: p.stops, onToggle: () => {} });
    expect(cell(off, 'main1')).toContain('aria-pressed="false"');
  });

  it('untap can never take a stop: toggleStop refuses it and the cell stays inert', () => {
    const p = new SeatPanelState('t1', 1, { seat: 1, token: 'tok' }, null);
    p.stops = stops([], []);
    p.toggleStop('untap', 'yours');
    expect(p.stops.yours.has('untap')).toBe(false);
    const html = track({ view: view(1, 'upkeep'), seats, seat: 1, stops: p.stops, onToggle: () => {} });
    expect(cell(html, 'untap')).not.toContain('aria-pressed');
  });

  it('a stop set on the opponent side does not mark the same cell on your turn', () => {
    const st = stops([], ['declare-blockers', 'end']);
    // your turn: the track shows YOUR set, which is empty
    const mine = seated(view(1, 'upkeep'), st);
    expect(cell(mine, 'declare-blockers')).toContain('aria-pressed="false"');
    expect(cell(mine, 'end')).toContain('aria-pressed="false"');
    // their turn: the same two cells are the ones that are marked
    const theirs = seated(view(0, 'upkeep'), st);
    expect(cell(theirs, 'declare-blockers')).toContain('aria-pressed="true"');
    expect(cell(theirs, 'end')).toContain('aria-pressed="true"');
  });

  it('a stop set on your side does not mark the same cell on the opponent’s turn', () => {
    const st = stops(['main1', 'main2'], []);
    const mine = seated(view(1, 'upkeep'), st);
    expect(cell(mine, 'main1')).toContain('aria-pressed="true"');
    const theirs = seated(view(0, 'upkeep'), st);
    expect(cell(theirs, 'main1')).toContain('aria-pressed="false"');
    expect(cell(theirs, 'main2')).toContain('aria-pressed="false"');
  });

  it('a spectator gets the same clock with nothing focusable and no modifier hint', () => {
    const html = track({ view: view(0, 'main1'), seats });
    expect(html).toContain('data-phase-track');
    expect(html).not.toContain('<button');
    expect(html).not.toContain('aria-pressed');
    expect(html).not.toContain('data-hint');
    // the step is still marked: the clock is the point, the stops are extra
    expect(cell(html, 'main1')).toContain('aria-current="step"');
    expect(html).toContain('alice’s turn');
  });
});
