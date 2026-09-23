import { describe, expect, it } from 'vitest';
import type { Decision, Option } from '../protocol';
import { pickOption } from './seatpanel.svelte';

// pickOption against a raised per-Group cap (Decision.groupLimit): an option
// Group is one listed type of the EACH multi-type search grammar and may
// contribute up to groupLimit picks. The client stays rules-ignorant (R-E4-2):
// it derives the cap from the wire field alone, appends while the group has
// room, and replaces the oldest member once it is full. At the default cap
// (absent field, or 1) the pick is the historical replace-on-represented.

const opt = (index: number, obj: number, group?: string): Option => ({
  index,
  kind: 'search',
  label: `option ${index}`,
  obj,
  player: 0,
  ...(group ? { group } : {}),
});

const eachTwo = (groupLimit?: number): Decision => ({
  seq: 9,
  player: 0,
  kind: 'choose',
  prompt: 'choose one card of each listed type',
  min: 0,
  max: 4,
  ...(groupLimit !== undefined ? { groupLimit } : {}),
  options: [
    opt(0, 1, '0'),
    opt(1, 2, '0'),
    opt(2, 3, '1'),
    opt(3, 4, '1'),
  ],
});

describe('pickOption — per-Group cap from groupLimit', () => {
  it('appends a second member of a group while the raised cap has room', () => {
    const d = eachTwo(2);
    expect(pickOption(d, 0, [])).toEqual([0]);
    expect(pickOption(d, 1, [0])).toEqual([0, 1]);
    expect(pickOption(d, 2, [0, 1])).toEqual([0, 1, 2]);
  });
  it('replaces the oldest member once a raised cap is full', () => {
    const d: Decision = {
      seq: 10,
      player: 0,
      kind: 'choose',
      prompt: 'choose two of each listed type',
      min: 0,
      max: 4,
      groupLimit: 2,
      options: [opt(0, 1, '0'), opt(1, 2, '0'), opt(2, 3, '0'), opt(3, 4, '1')],
    };
    let picked = pickOption(d, 0, []); // [0]
    picked = pickOption(d, 1, picked); // [0,1] -- group "0" full
    expect(picked).toEqual([0, 1]);
    picked = pickOption(d, 2, picked); // group "0" is full: oldest member replaced
    expect(picked).toEqual([1, 2]);
    picked = pickOption(d, 3, picked); // group "1" appends
    expect(picked).toEqual([1, 2, 3]);
  });
  it('keeps the historical replace at the default cap (no field)', () => {
    const d = eachTwo();
    expect(pickOption(d, 0, [])).toEqual([0]);
    expect(pickOption(d, 1, [0])).toEqual([1]);
  });
  it('treats an explicit groupLimit of 1 like the default cap', () => {
    const d = eachTwo(1);
    expect(pickOption(d, 0, [])).toEqual([0]);
    expect(pickOption(d, 1, [0])).toEqual([1]);
  });
});
