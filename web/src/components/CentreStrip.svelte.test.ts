import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { SeatInfo, View } from '../protocol';
import CentreStrip from './CentreStrip.svelte';

const seats: SeatInfo[] = [
  { name: 'alice', deck: 'burn', colour: '#e5484d' },
  { name: 'bob', deck: 'stompy', colour: '#30a46c' },
];

const view = (turn: number, round: number): View =>
  ({
    viewer: 1, visibility: 'seat', turn, round, step: 'main1', phase: '', active: 0, priority: 0,
    over: false, draw: false, winner: null, players: [], stack: [], pending: [],
  }) as unknown as View;

const label = (v: View) => render(CentreStrip, { props: { view: v, seats } }).html;

describe('CentreStrip', () => {
  it('numbers the turn as the game and the transcript do, not the round', () => {
    expect(label(view(5, 3))).toMatch(/Turn <b[^>]*>5<\/b>/);
  });

  it('reads turn 1 before the first turn starts', () => {
    expect(label(view(0, 1))).toMatch(/Turn <b[^>]*>1<\/b>/);
  });
});
