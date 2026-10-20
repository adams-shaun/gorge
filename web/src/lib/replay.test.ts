import { describe, expect, it } from 'vitest';
import { initialDvr, type DvrState } from './dvr';
import type { EventBody } from '../protocol';
import { bandAt, intervalMs, nextStop, parseReplayQuery, replaySearch, resolveStart, stepTarget, turnBands } from './replay';

const ev = (seq: number, kind: string, player = 0, amount?: number): EventBody => ({
  event: { seq, kind, player, ...(amount !== undefined ? { amount } : {}) },
  line: `${kind} ${seq}`,
});

/**
 * A two-seat game of 40 events: turns 1-4 start at 2, 12, 22, 32 (seats
 * 0,1,0,1); a decision is asked at 5, 15, 25 and 35 and answered one event
 * later.
 */
function game(cursor = 0): DvrState {
  const events: EventBody[] = [];
  const turns: Record<number, [number, number]> = { 2: [0, 1], 12: [1, 2], 22: [0, 3], 32: [1, 4] };
  for (let s = 0; s < 40; s++) {
    if (turns[s]) events.push(ev(s, 'turn', turns[s][0], turns[s][1]));
    else if ([5, 15, 25, 35].includes(s)) events.push(ev(s, 'decision_ask'));
    else if ([6, 16, 26, 36].includes(s)) events.push(ev(s, 'decision_made'));
    else events.push(ev(s, 'priority'));
  }
  return { ...initialDvr, match: 't/1', head: 39, cursor, live: false, events, turnStarts: [2, 12, 22, 32] };
}

describe('replay steps', () => {
  it('nextStop finds the strict neighbour either way', () => {
    expect(nextStop([2, 12, 22], 12, 1)).toBe(22);
    expect(nextStop([2, 12, 22], 12, -1)).toBe(2);
    expect(nextStop([2, 12, 22], 22, 1)).toBeNull();
    expect(nextStop([2, 12, 22], 2, -1)).toBeNull();
  });

  it('steps by event, decision and turn, landing on head or 0 past the last stop', () => {
    expect(stepTarget(game(10), 'event', 1)).toBe(11);
    expect(stepTarget(game(0), 'event', -1)).toBe(0);
    expect(stepTarget(game(10), 'decision', 1)).toBe(15);
    expect(stepTarget(game(15), 'decision', -1)).toBe(5);
    expect(stepTarget(game(35), 'decision', 1)).toBe(39);
    expect(stepTarget(game(3), 'decision', -1)).toBe(0);
    expect(stepTarget(game(12), 'turn', 1)).toBe(22);
    expect(stepTarget(game(13), 'turn', -1)).toBe(12);
  });

  it('plays faster at higher speeds and never below a frame', () => {
    expect(intervalMs('decision', 2)).toBeLessThan(intervalMs('decision', 1));
    expect(intervalMs('event', 1000)).toBe(16);
  });
});

describe('turn bands', () => {
  it('draws one band per turn with its active seat and round', () => {
    const b = turnBands(game(), 2);
    expect(b.map((x) => [x.start, x.end, x.turn, x.round, x.player])).toEqual([
      [2, 12, 1, 1, 0],
      [12, 22, 2, 1, 1],
      [22, 32, 3, 2, 0],
      [32, 40, 4, 2, 1],
    ]);
    expect(bandAt(b, 1)).toBeNull();
    expect(bandAt(b, 25)?.turn).toBe(3);
  });
});

describe('replay query', () => {
  it('parses position and playback parameters, ignoring junk', () => {
    expect(parseReplayQuery('?seq=120&unit=turn&speed=4&play=1')).toEqual({ seq: 120, unit: 'turn', speed: 4, play: true });
    expect(parseReplayQuery('?event=7&intent=3&turn=2')).toEqual({ seq: 7, intent: 3, turn: 2 });
    expect(parseReplayQuery('?decision=9')).toEqual({ intent: 9 });
    expect(parseReplayQuery('?seq=-4&turn=0&unit=frame&speed=3')).toEqual({});
  });

  it('resolves seq, then intent, then turn (a round, as the phase bar shows it)', () => {
    const d = game(39);
    expect(resolveStart({ seq: 500 }, d, 2)).toBe(39);
    expect(resolveStart({ seq: 4, intent: 2 }, d, 2)).toBe(4);
    expect(resolveStart({ intent: 2 }, d, 2)).toBe(25);
    expect(resolveStart({ intent: 99 }, d, 2)).toBe(39);
    expect(resolveStart({ turn: 2 }, d, 2)).toBe(22);
    expect(resolveStart({ turn: 9 }, d, 2)).toBe(39);
    expect(resolveStart({}, d, 2)).toBeNull();
  });

  it('writes the position back, keeping unrelated parameters and dropping defaults', () => {
    expect(replaySearch('?intent=3&seat=1', 120, 'decision', 1)).toBe('?seat=1&seq=120');
    expect(replaySearch('', 5, 'turn', 2)).toBe('?seq=5&unit=turn&speed=2');
  });
});
