import { describe, expect, it, vi } from 'vitest';
import type { Decision, Option } from '../protocol';
import {
  optionsByObj,
  cardOptions,
  hasCardOptions,
  optionSetFor,
  optionSetForMany,
  tileOptions,
  tileOptionsMany,
  postSingleAction,
  singleActionIcon,
  type CardOptions,
} from './cardoptions';

// cardoptions.ts is the mechanism behind "options on the card", "highlight
// cards with options" and "highlight valid targets" — one index keyed on
// Option.obj. These tests build the decision by hand and assert nothing
// about creatures, combat or rules: the client (R-E4-2) only groups options
// the server already sent, and the only selection constraint is the option's
// own index (R-E4-1).

const opt = (index: number, obj: number | undefined, kind = 'cast', label = `option ${index}`): Option => ({
  index, kind, label, obj, player: 0,
});

describe('optionsByObj — index a decision by the object each option concerns', () => {
  it('groups options by obj, preserving wire order within each object', () => {
    const d: Decision = {
      seq: 1, player: 0, kind: 'priority', prompt: 'You have priority',
      min: 0, max: 1,
      options: [opt(0, 5, 'cast', 'Cast Fireball'), opt(1, 7, 'cast', 'Cast Bear'), opt(2, 5, 'ability', 'Activate Fireball')],
    };
    const m = optionsByObj(d);
    expect(m.get(5)?.map((o) => o.index)).toEqual([0, 2]); // wire order inside obj 5
    expect(m.get(7)?.map((o) => o.index)).toEqual([1]);
    expect(m.size).toBe(2);
  });

  it('options with no obj belong to no card and are not indexed — but nothing is dropped', () => {
    const d: Decision = {
      seq: 1, player: 0, kind: 'priority', prompt: 'You have priority',
      min: 0, max: 1,
      options: [opt(0, 5, 'cast'), opt(1, undefined, 'pass'), opt(2, undefined, 'concede')],
    };
    const m = optionsByObj(d);
    expect(m.size).toBe(1); // only the card-anchored cast
    expect(hasCardOptions(m, 5)).toBe(true);
    expect(hasCardOptions(m, 0)).toBe(false); // the pass/concede never index onto a card
    // but the full option set is still reachable from the decision (the panel)
    expect(d.options.length).toBe(3);
  });

  it('a null decision indexes nothing', () => {
    expect(optionsByObj(null).size).toBe(0);
  });

  it('two options on the same obj keep their own indices (R-E4-1: index is the identity, not position)', () => {
    const d: Decision = {
      seq: 1, player: 0, kind: 'priority', prompt: 'You have priority',
      min: 0, max: 1,
      options: [opt(3, 5, 'cast'), opt(8, 5, 'ability')],
    };
    const list = cardOptions(optionsByObj(d), 5) ?? [];
    expect(list.map((o) => o.index)).toEqual([3, 8]);
  });
});

describe('cardOptions / hasCardOptions — cheap single-object lookups', () => {
  const d: Decision = {
    seq: 1, player: 0, kind: 'target', prompt: 'Choose a target', min: 1, max: 1,
    options: [opt(0, 10, 'permanent', 'Grizzly Bears (Ari)'), opt(1, 12, 'player', 'Ari')],
  };
  const m = optionsByObj(d);

  it('a tile ask for a present obj returns its options', () => {
    expect(cardOptions(m, 10)?.map((o) => o.index)).toEqual([0]);
    expect(hasCardOptions(m, 10)).toBe(true);
  });

  it('a tile ask for an absent obj is a clean false/null, never a crash or a fabricated option', () => {
    expect(cardOptions(m, 999)).toBeNull();
    expect(hasCardOptions(m, 999)).toBe(false);
  });
});

describe('optionSetFor — one object\'s options plus its picked indices', () => {
  it('returns the object\'s options and the picked ordinals among them, in click order', () => {
    const d: Decision = {
      seq: 1, player: 0, kind: 'blockers', prompt: 'declare blockers', min: 0, max: 4,
      options: [opt(0, 5), opt(1, 5), opt(2, 6)],
    };
    const m = optionsByObj(d);
    // picked = [index 2 (obj 6), then index 1 (obj 5)] — obj 5's option is the
    // SECOND pick, so its ordinal is 2, in click order.
    expect(optionSetFor(m, 5, [2, 1])).toEqual({ list: [d.options[0], d.options[1]], pickedOrder: [2] });
    // obj 6's option is the FIRST pick, ordinal 1.
    expect(optionSetFor(m, 6, [2, 1])).toEqual({ list: [d.options[2]], pickedOrder: [1] });
  });

  it('an object with no options returns null (no affordance, no mark)', () => {
    const d: Decision = {
      seq: 1, player: 0, kind: 'blockers', prompt: 'declare blockers', min: 0, max: 4,
      options: [opt(0, 5)],
    };
    expect(optionSetFor(optionsByObj(d), 6, [0])).toBeNull();
  });
});

describe('optionSetForMany — a collapsed stack group of interchangeable permanents', () => {
  it('folds every member\'s options together, in member order then wire order', () => {
    const d: Decision = {
      seq: 1, player: 0, kind: 'priority', prompt: 'You have priority', min: 0, max: 1,
      options: [opt(0, 5), opt(1, 6), opt(2, 5)],
    };
    const m = optionsByObj(d);
    const set = optionSetForMany(m, [5, 6], []);
    expect(set?.list.map((o) => o.index)).toEqual([0, 2, 1]);
  });

  it('returns null when no member is offered anything', () => {
    const d: Decision = {
      seq: 1, player: 0, kind: 'priority', prompt: 'You have priority', min: 0, max: 1,
      options: [opt(0, 5)],
    };
    expect(optionSetForMany(optionsByObj(d), [6, 7], [])).toBeNull();
  });
});

describe('single-action card affordance', () => {
  it('uses wire kind for cast and dedicated mana-tap icons, and stays neutral for an opaque ability', () => {
    expect(singleActionIcon(opt(4, 9, 'cast'))).toBe('cast');
    expect(singleActionIcon(opt(5, 9, 'activate'))).toBe('tap');
    expect(singleActionIcon(opt(6, 9, 'ability'))).toBe('action');
    expect(singleActionIcon(opt(7, 9, 'permanent'))).toBe('action');
  });

  it('posts the sole option own wire index, never its list position (R-E4-1)', () => {
    const post = vi.fn();
    const tile: import('./cardoptions').TileOptions = {
      list: [opt(17, 9, 'cast', 'Cast Wasteland')],
      pickedOrder: [], tone: 'offered', post,
    };
    postSingleAction(tile);
    expect(post).toHaveBeenCalledWith(17);
    expect(post).not.toHaveBeenCalledWith(0);
  });

  it('does not turn a multi-option menu into an invented direct action (R-E4-2)', () => {
    const post = vi.fn();
    postSingleAction({
      list: [opt(3, 9), opt(8, 9)], pickedOrder: [], tone: 'offered', post,
    });
    expect(post).not.toHaveBeenCalled();
  });
});

describe('tileOptions / tileOptionsMany — the bundle reductions the components render', () => {
  const post = vi.fn();
  const bundle = (d: Decision, tone: CardOptions['tone'] = 'offered'): CardOptions => ({
    byObj: optionsByObj(d), picked: [], tone, post,
  });

  it('tileOptions returns null for an object the decision offers nothing — the no-badge state', () => {
    const d: Decision = {
      seq: 1, player: 0, kind: 'priority', prompt: 'You have priority', min: 0, max: 1,
      options: [opt(0, 5)],
    };
    expect(tileOptions(bundle(d), 6)).toBeNull();
    expect(tileOptions(bundle(d), 5)?.list.map((o) => o.index)).toEqual([0]);
  });

  it('tileOptionsMany folds a stack group; tileOptions handles a single member', () => {
    const d: Decision = {
      seq: 1, player: 0, kind: 'blockers', prompt: 'declare blockers', min: 0, max: 4,
      options: [opt(3, 5), opt(4, 6)],
    };
    const b = bundle(d, 'initiative');
    expect(tileOptions(b, 5)?.pickedOrder).toEqual([]);
    expect(tileOptions(b, 5)?.tone).toBe('initiative');
    expect(tileOptionsMany(b, [5, 6])?.list.map((o) => o.index)).toEqual([3, 4]);
  });

  it('the posted index is the option\'s OWN index, never a position in a rebuilt list (R-E4-1)', () => {
    // Reorder the DECISION's options so a naive "first in my rebuilt list"
    // position would pick the wrong option; the bundle must still hand back
    // the option's OWN index field, which is unrelated to its array position.
    const d: Decision = {
      seq: 1, player: 0, kind: 'priority', prompt: 'You have priority', min: 0, max: 1,
      options: [opt(5, 9, 'ability', 'Activate #9'), opt(2, 9, 'cast', 'Cast #9')],
    };
    // The option at array position 0 carries index 5; the one at position 1
    // carries index 2.
    expect(d.options[0].index).toBe(5);
    expect(d.options[1].index).toBe(2);
    const b = bundle(d);
    const t = tileOptions(b, 9);
    expect(t?.list.map((o) => o.index)).toEqual([5, 2]); // wire order, own index fields
    t?.post(t.list[0].index); // posting the first list entry carries index 5
    expect(post).toHaveBeenCalledWith(5);
    // even if the tile had re-ordered, index 5 still names the ability
    expect(d.options[0].kind).toBe('ability');
    expect(d.options[0].label).toBe('Activate #9');
    expect(d.options[1].label).toBe('Cast #9');
  });
});
