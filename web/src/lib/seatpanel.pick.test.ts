import { describe, expect, it } from 'vitest';
import type { Decision, Option } from '../protocol';
import { pickOption } from './seatpanel.svelte';

// pickOption is the seat's pure selection logic, expressed entirely in terms
// of the decision's own Option.Group field (R-E4-2): the panel never finds
// out what a Group's members are, only that a non-empty Group is an
// exclusivity marker. These tests therefore build the decision by hand and
// assert nothing about blockers, creatures or combat.

const block = (index: number, obj: number, group?: string): Option => ({
  index,
  kind: 'block',
  label: `option ${index}`,
  obj,
  player: 0,
  ...(group ? { group } : {}),
});

// A block-style decision: one blocker (obj 5) is offered against two
// attackers (groups share "blocker:5"), a second blocker (obj 6) against one.
const d: Decision = {
  seq: 3,
  player: 0,
  kind: 'blockers',
  prompt: 'declare blockers',
  min: 0,
  max: 4,
  options: [
    block(0, 5, 'blocker:5'), // Bear blocks Alpha
    block(1, 5, 'blocker:5'), // Bear blocks Beta
    block(2, 6, 'blocker:6'), // Wolf blocks Alpha
  ],
};

describe('pickOption — exclusivity by Group', () => {
  it('picking two options of one Group never holds both; the second REPLACES the first', () => {
    // pick Bear blocks Alpha
    expect(pickOption(d, 0, [])).toEqual([0]);
    // move Bear to Beta: Alpha (index 0) is dropped, Beta takes its place
    expect(pickOption(d, 1, [0])).toEqual([1]);
  });

  it('a replacement preserves the click order of the other picks and appends the new one', () => {
    // pick Wolf (2), then Bear against Alpha (0)
    expect(pickOption(d, 2, [])).toEqual([2]);
    expect(pickOption(d, 0, [2])).toEqual([2, 0]);
    // now move Bear to Beta: index 0 is replaced by index 1, Wolf (2) stays put
    expect(pickOption(d, 1, [2, 0])).toEqual([2, 1]);
  });

  it('re-picking an already-picked option toggles it off, even inside a group', () => {
    expect(pickOption(d, 0, [0])).toEqual([]);
    expect(pickOption(d, 1, [1, 2])).toEqual([2]);
  });

  it('different Groups coexist; at most one option per Group is ever held', () => {
    // one option from each group
    expect(pickOption(d, 0, [])).toEqual([0]);
    expect(pickOption(d, 2, [0])).toEqual([0, 2]);
    // adding another member of a represented group replaces that member only
    expect(pickOption(d, 1, [0, 2])).toEqual([2, 1]);
  });

  it('an out-of-range index is a no-op (never panics, never mutates)', () => {
    expect(pickOption(d, 99, [0])).toEqual([0]);
  });

  it('options with no Group are never exclusive (R-E4-2: only the Group contract applies)', () => {
    const ungrouped: Decision = {
      seq: 4,
      player: 0,
      kind: 'choose',
      prompt: 'bottom 2',
      min: 0,
      max: 2,
      options: [
        { index: 0, kind: 'bottom', label: 'A', obj: undefined, player: 0 },
        { index: 1, kind: 'bottom', label: 'B', obj: undefined, player: 0 },
      ],
    };
    expect(pickOption(ungrouped, 0, [])).toEqual([0]);
    expect(pickOption(ungrouped, 1, [0])).toEqual([0, 1]);
  });
});
