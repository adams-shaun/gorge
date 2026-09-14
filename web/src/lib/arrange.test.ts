import { describe, expect, it } from 'vitest';
import type { Decision, Option } from '../protocol';
import { pickOption } from './seatpanel.svelte';
import { arrangeCard, arrangeDestination, arrangeOrder, arrangeSplit, moveWithin } from './arrange';

// A KArrange reorder the way effRearrangeTopOfLibrary poses it: Min == Max ==
// len(options), every option Kind "bottom", Label the card's name, Obj the
// library object. Indices are dense from 0 (the invariant Engine.ask enforces).
const opt = (index: number, name: string, obj: number, kind = 'bottom'): Option =>
  ({ index, kind, label: name, obj, player: 0 });
const reorder: Decision = {
  seq: 7, player: 0, kind: 'arrange', min: 5, max: 5,
  prompt: 'Rearrange the top 5 card(s); the first card you pick goes on top',
  source: 9,
  options: [
    opt(0, 'Brazen Borrower', 11),
    opt(1, 'Fabled Pass', 12),
    opt(2, 'Gitaxian Probe', 13),
    opt(3, 'Spell Pierce', 14),
    opt(4, 'Unholy Heat', 15),
  ],
};
const scry: Decision = {
  ...reorder, min: 0,
  prompt: 'Scry 5: pick the cards to keep on top, in order; the rest go to the bottom of your library',
};
const surveil: Decision = {
  ...reorder, min: 0, options: reorder.options.map((o) => ({ ...o, kind: 'graveyard' })),
};

describe('arrangeDestination — pile B’s destination, in the UI’s words', () => {
  it('names the destination the shared Option.Kind carries', () => {
    expect(arrangeDestination(reorder)).toBe('the bottom of the library');
    expect(arrangeDestination(surveil)).toBe('the graveyard');
    expect(arrangeDestination({ ...reorder, options: [opt(0, 'x', 1, 'exile')] })).toBe('exile');
  });
});

describe('arrangeSplit — the keep/pool piles of a KArrange ask', () => {
  it('splits a pure reorder: everything picked, in picked order; pool empty', () => {
    const { keep, pool } = arrangeSplit(reorder, [3, 1, 0, 2, 4]);
    expect(keep.map((o) => o.index)).toEqual([3, 1, 0, 2, 4]);
    expect(keep.map((o) => o.obj)).toEqual([14, 12, 11, 13, 15]);
    expect(pool).toEqual([]);
  });

  it('splits a scry: the picked keep pile in order, the rest in offered order', () => {
    const { keep, pool } = arrangeSplit(scry, [2, 0]);
    expect(keep.map((o) => o.index)).toEqual([2, 0]);
    expect(pool.map((o) => o.index)).toEqual([1, 3, 4]);
  });

  it('an unknown picked index is ignored, never invented', () => {
    expect(arrangeSplit(scry, [2, 99]).keep.map((o) => o.index)).toEqual([2]);
    expect(arrangeSplit(scry, [2, 99]).pool.map((o) => o.index)).toEqual([0, 1, 3, 4]);
  });

  it('nothing picked keeps everything in the pool, offered order intact', () => {
    const { keep, pool } = arrangeSplit(surveil, []);
    expect(keep).toEqual([]);
    expect(pool.map((o) => o.index)).toEqual([0, 1, 2, 3, 4]);
  });
});

describe('arrangeOrder — the byte-equivalence with the old click-order answer', () => {
  it('the popup’s final keep order posts exactly the picked array clicking in that order would build', () => {
    // The old path: click option 3, then 1, then 0, then 2, then 4 — each
    // click appends (pickOption), so `picked` IS the click order, and submit
    // posts [...picked]. The popup path: arrange the keep pile to the same
    // final order and post arrangeOrder. The two arrays must be identical,
    // because both become the intent's `choices` on the same decision.
    let clicked: number[] = [];
    for (const i of [3, 1, 0, 2, 4]) clicked = pickOption(reorder, i, clicked);
    const popupOrder = arrangeOrder(arrangeSplit(reorder, [3, 1, 0, 2, 4]));
    expect(clicked).toEqual([3, 1, 0, 2, 4]);
    expect(popupOrder).toEqual(clicked);
  });

  it('a scry keeps only the kept cards’ indices, in keep order', () => {
    expect(arrangeOrder(arrangeSplit(scry, [4, 1]))).toEqual([4, 1]);
    expect(arrangeOrder(arrangeSplit(scry, []))).toEqual([]);
  });
});

describe('moveWithin — the drag-reorder move, pure', () => {
  const list = ['a', 'b', 'c', 'd'];
  it('moves an item forward and backward', () => {
    expect(moveWithin(list, 0, 2)).toEqual(['b', 'c', 'a', 'd']);
    expect(moveWithin(list, 3, 1)).toEqual(['a', 'd', 'b', 'c']);
  });
  it('a same-position or out-of-range move is a copy, not a corruption', () => {
    expect(moveWithin(list, 2, 2)).toEqual(list);
    expect(moveWithin(list, -1, 0)).toEqual(list);
    expect(moveWithin(list, 0, 9)).toEqual(list);
    const out = moveWithin(list, 0, 1);
    expect(out).not.toBe(list); // a fresh array; the input is never mutated
    expect(list).toEqual(['a', 'b', 'c', 'd']);
  });
});

describe('arrangeCard — the face an option renders as', () => {
  it('carries the option’s own name and object id (the decision is the payload; the library itself is not projected)', () => {
    const c = arrangeCard(reorder, reorder.options[2]);
    expect(c.id).toBe(13);
    expect(c.printing.name).toBe('Gitaxian Probe');
    expect(c.name).toBe('Gitaxian Probe');
    expect(c.types).toBe('');
  });
  it('an option without an obj degrades to a placeholder id, never undefined', () => {
    const c = arrangeCard(reorder, opt(0, 'Mystery', undefined as unknown as number));
    expect(c.id).toBe(-1);
  });
});
