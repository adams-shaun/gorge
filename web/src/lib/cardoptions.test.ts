import { describe, expect, it, vi } from 'vitest';
import type { Decision, Option } from '../protocol';
import {
  optionsByObj,
  optionsByPlayer,
  cardOptions,
  hasCardOptions,
  optionSetFor,
  optionSetForMany,
  tileOptions,
  tileOptionsMany,
  playerOptions,
  postSingleAction,
  postTileOption,
  resolveCardFollowUp,
  singleActionIcon,
  scenarioIconOf,
  actionAccessibleLabel,
  tileScenario,
  ACTION_GLYPHS,
  singleTapOptionOf,
  pileTone,
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

describe('pileTone — a whole pile affordance tone (fb-20260916T225802Z)', () => {
  const bundleFor = (options: Option[], tone: 'initiative' | 'offered' = 'initiative'): CardOptions => {
    const d: Decision = {
      seq: 1, player: 0, kind: 'priority', prompt: 'pile tone', min: 0, max: 1, options,
    };
    return { byObj: optionsByObj(d), byPlayer: optionsByPlayer(d), picked: [], tone, post: () => {} };
  };

  it('a pile holding a card the decision offers something to wears the bundle\'s tone', () => {
    // the flashback-cast shape: an option whose obj is a graveyard card id
    const bundle = bundleFor([opt(7, 3, 'cast', 'Flashback Bolt')], 'offered');
    expect(pileTone(bundle, [{ id: 3 }, { id: 4 }])).toBe('offered');
  });

  it('an untouched pile reads idle — no ring, no claim', () => {
    const bundle = bundleFor([opt(7, 3, 'cast', 'Flashback Bolt')]);
    expect(pileTone(bundle, [{ id: 4 }, { id: 5 }])).toBe('idle');
    expect(pileTone(bundle, [])).toBe('idle');
  });

  it('a null bundle (spectator, nothing pending) is idle even over a full pile', () => {
    expect(pileTone(null, [{ id: 3 }])).toBe('idle');
  });

  it('not gated by pile owner: the same rule glows an opponent-targeted option', () => {
    // a target option on a card in an OPPONENT's graveyard must glow there
    // too; the engine validates every posted option.
    const bundle = bundleFor([opt(21, 300, 'permanent', 'Target their Bolt')], 'initiative');
    expect(pileTone(bundle, [{ id: 300 }])).toBe('initiative');
  });
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

describe('optionsByPlayer — index a decision by the seat a player-target option names', () => {
  const playerOpt = (index: number, player: number): Option => ({ index, kind: 'player', label: `target player ${player}`, player });

  it('groups player-kind options by their player field', () => {
    const d: Decision = {
      seq: 1, player: 0, kind: 'target', prompt: 'Choose a target', min: 1, max: 1,
      options: [playerOpt(0, 1), opt(1, 9, 'permanent', 'target Bear')],
    };
    const m = optionsByPlayer(d);
    expect(m.get(1)?.map((o) => o.index)).toEqual([0]);
    expect(m.size).toBe(1); // the permanent-target option is not indexed here at all
  });

  it('does NOT index pass/concede/cast options by their acting player — only kind "player" targets', () => {
    const d: Decision = {
      seq: 1, player: 2, kind: 'priority', prompt: 'You have priority',
      min: 1, max: 1,
      options: [opt(0, undefined, 'pass', 'Pass'), opt(1, 5, 'cast', 'Cast Fireball')],
    };
    // Neither option is kind "player", so optionsByPlayer must find nothing —
    // otherwise seat 2 (the acting seat on both) would be wrongly marked as
    // a legal target of its own priority window.
    expect(optionsByPlayer(d).size).toBe(0);
  });

  it('a null decision indexes nothing', () => {
    expect(optionsByPlayer(null).size).toBe(0);
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
    expect(singleActionIcon(opt(7, 9, 'permanent'))).toBe('target');
  });

  it('maps every scenario the reporter named to its own icon (task fb-9946410e)', () => {
    // play from hand → cast (✦, kept); target of a spell/ability/trigger →
    // bullseye; select as attacker → sword; select as defender → shield;
    // tap/activate → ↻ (kept); everything else stays neutral.
    expect(scenarioIconOf('cast')).toBe('cast');
    expect(scenarioIconOf('permanent')).toBe('target');
    expect(scenarioIconOf('player')).toBe('target');
    expect(scenarioIconOf('attacker')).toBe('attack');
    expect(scenarioIconOf('block')).toBe('block');
    expect(scenarioIconOf('activate')).toBe('tap');
    for (const neutral of ['ability', 'sacrifice', 'discard', 'x', 'name', 'type', 'number', 'exile', 'division', 'pay_2']) {
      expect(scenarioIconOf(neutral)).toBe('action');
    }
  });

  it('every icon has a glyph and a count-badge noun — the tables the badges render from', () => {
    const icons = new Set(['cast', 'tap', 'target', 'attack', 'block', 'action']);
    for (const kind of ['cast', 'activate', 'permanent', 'player', 'attacker', 'block', 'ability']) {
      icons.delete(scenarioIconOf(kind));
    }
    expect(icons.size).toBe(0); // every mapped icon is covered by the tables
    for (const icon of ['cast', 'tap', 'target', 'attack', 'block', 'action'] as const) {
      expect(ACTION_GLYPHS[icon]).toBeTruthy();
      expect(ACTION_GLYPHS[icon].length).toBeGreaterThan(0);
    }
  });

  it('actionAccessibleLabel prefixes a target candidate with its scenario verb, in one place (fb-9946410e r2)', () => {
    // A target option's wire label is only the candidate's own name, so the
    // direct badge's accessible name would read neither "target" nor the
    // scenario. The prefix lives here, once, not per renderer.
    expect(actionAccessibleLabel({ kind: 'permanent', label: 'Wasteland (Ari)' })).toBe('Target Wasteland (Ari)');
    expect(actionAccessibleLabel({ kind: 'player', label: 'Ari' })).toBe('Target Ari');
  });

  it('every other scenario kind already names its scenario in the wire label and passes through verbatim', () => {
    // cast/activate/attacker/block labels are minted with their scenario verb
    // server-side ("Cast X", "Tap X for mana", "Attack with X at Y", "X blocks
    // Y"), and the neutral kinds are descriptive — re-phrasing them would only
    // risk lying about what the option does.
    expect(actionAccessibleLabel({ kind: 'cast', label: 'Cast Fireball' })).toBe('Cast Fireball');
    expect(actionAccessibleLabel({ kind: 'activate', label: 'Tap Island for mana' })).toBe('Tap Island for mana');
    expect(actionAccessibleLabel({ kind: 'attacker', label: 'Attack with Bear at Ari' })).toBe('Attack with Bear at Ari');
    expect(actionAccessibleLabel({ kind: 'block', label: 'Bear blocks Hill Giant' })).toBe('Bear blocks Hill Giant');
    expect(actionAccessibleLabel({ kind: 'ability', label: 'Wasteland: sacrifice it' })).toBe('Wasteland: sacrifice it');
  });

  describe('tileScenario — the count badge\'s scenario', () => {
    const tile = (list: Option[]): import('./cardoptions').TileOptions =>
      ({ list, pickedOrder: [], tone: 'offered', post: vi.fn() });

    it('a same-scenario list names the scenario: three target options are "3 targets"', () => {
      const s = tileScenario(tile([
        opt(0, 5, 'permanent', 'Bear'), opt(1, 5, 'player', 'Ari'), opt(2, 5, 'permanent', 'Hill Giant'),
      ]));
      expect(s).toEqual({ icon: 'target', noun: 'targets' });
    });

    it('an attacker list is "attacks", a blocker list is "blocks", a cast list is "plays"', () => {
      expect(tileScenario(tile([opt(0, 5, 'attacker'), opt(1, 5, 'attacker')]))).toEqual({ icon: 'attack', noun: 'attacks' });
      expect(tileScenario(tile([opt(0, 5, 'block')]))).toEqual({ icon: 'block', noun: 'blocks' });
      expect(tileScenario(tile([opt(0, 5, 'cast'), opt(1, 5, 'cast')]))).toEqual({ icon: 'cast', noun: 'plays' });
    });

    it('a list of all-neutral options still names the neutral scenario, not a fake one', () => {
      expect(tileScenario(tile([opt(0, 5, 'ability'), opt(1, 5, 'ability')]))).toEqual({ icon: 'action', noun: 'actions' });
    });

    it('a genuinely MIXED list gets null — the badge falls back to the bare neutral count', () => {
      expect(tileScenario(tile([opt(0, 5, 'cast'), opt(1, 5, 'ability')]))).toBeNull();
      expect(tileScenario(tile([opt(0, 5, 'permanent'), opt(1, 5, 'block')]))).toBeNull();
      expect(tileScenario(tile([]))).toBeNull();
    });
  });

  it('posts the sole option own wire index, never its list position (R-E4-1)', () => {
    const post = vi.fn();
    const tile: import('./cardoptions').TileOptions = {
      list: [opt(17, 9, 'cast', 'Cast Wasteland')],
      pickedOrder: [], tone: 'offered', post,
    };
    postSingleAction(tile);
    expect(post).toHaveBeenCalledWith(17, false, false);
    expect(post).not.toHaveBeenCalledWith(0);
  });

  it('peels one homogeneous mana activation from a collapsed pile by wire index', () => {
    const post = vi.fn();
    const tile: import('./cardoptions').TileOptions = {
      // Stack member order is deliberately not wire-index order.
      list: [opt(17, 9, 'activate', 'Tap Island for mana'), opt(4, 10, 'activate', 'Tap Island for mana')],
      pickedOrder: [], tone: 'offered', post,
    };
    expect(singleTapOptionOf(tile)?.index).toBe(4);
    postSingleAction(tile);
    expect(post).toHaveBeenCalledTimes(1);
    expect(post).toHaveBeenCalledWith(4, false, false);
  });

  it.each([
    { list: [] as Option[], case: 'an empty pile' },
    { list: [opt(3, 9, 'activate'), opt(8, 10, 'cast')], case: 'mixed action kinds' },
    { list: [opt(3, 9, 'activate', 'Tap Island for mana'), opt(8, 10, 'activate', 'Tap Forest for mana')], case: 'different wire semantics' },
  ])('keeps the dropdown for $case', ({ list }) => {
    const post = vi.fn();
    const tile: import('./cardoptions').TileOptions = {
      list, pickedOrder: [], tone: 'offered', post,
    };
    expect(singleTapOptionOf(tile)).toBeNull();
    postSingleAction(tile);
    expect(post).not.toHaveBeenCalled();
  });

  it('does not turn a multi-option menu into an invented direct action (R-E4-2)', () => {
    const post = vi.fn();
    postSingleAction({
      list: [opt(3, 9), opt(8, 9)], pickedOrder: [], tone: 'offered', post,
    });
    expect(post).not.toHaveBeenCalled();
  });

  // prio3 review r2: the tile affordances (direct icon, radial wheel, list
  // menu) are the one path a Ctrl-held cast/ability takes, and Ctrl must
  // arrive at SeatPanelState.click as `{ holdPriority: true }` — otherwise
  // the action arms pass-after-acting. These are the pure half; the mounted
  // Ctrl-click through CardTile/OptionPicker is CardMenu.test.ts.
  describe('holdPriority — the Ctrl modifier threads through the tile post', () => {
    const post = vi.fn();
    const tile = (): import('./cardoptions').TileOptions => ({
      list: [opt(3, 9, 'cast', 'Cast Fireball'), opt(8, 9, 'cast', 'Cast Shock')],
      pickedOrder: [], tone: 'offered', post,
    });

    it('postTileOption arms the follow-up and carries the modifier as post\'s third argument (fb-e079def5)', () => {
      postTileOption(tile(), tile().list[0]);
      expect(post).toHaveBeenLastCalledWith(3, true, false);
      postTileOption(tile(), tile().list[1], true);
      expect(post).toHaveBeenLastCalledWith(8, true, true);
    });

    it('postSingleAction carries it too, keeping its expectFollowUp argument', () => {
      const single = (): import('./cardoptions').TileOptions => ({
        list: [opt(17, 9, 'cast', 'Cast Fireball')], pickedOrder: [], tone: 'offered', post,
      });
      postSingleAction(single(), true, true);
      expect(post).toHaveBeenLastCalledWith(17, true, true);
      postSingleAction(single(), false, true);
      expect(post).toHaveBeenLastCalledWith(17, false, true);
      postSingleAction(single());
      expect(post).toHaveBeenLastCalledWith(17, false, false);
    });
  });

  // fb-e079def5: the stage-2 colour wheel never opened after a wheel-answered
  // stage-1 (a Talisman's ability pick), because only the single-action badge
  // path armed the follow-up expectation. resolveCardFollowUp is the ONE
  // decoder Table.svelte's $effect runs; these pin its contract.
  describe('resolveCardFollowUp — the one decoder of the armed card-follow-up expectation', () => {
    const expected = { seq: 9, obj: 83 };
    const decision = (seq: number, options: Option[]): Decision => ({
      seq, player: 0, kind: 'choose', prompt: 'Choose a colour of mana', min: 1, max: 1, options,
    });

    it('a same-object follow-up with 2-6 options re-opens that card\'s picker', () => {
      const d = decision(10, [
        opt(0, 83, 'mana', 'Add B'),
        opt(1, 83, 'mana', 'Add R'),
      ]);
      expect(resolveCardFollowUp(expected, d)).toEqual({ seq: 10, obj: 83 });
      expect(resolveCardFollowUp(expected, decision(11, [
        opt(0, 83, 'mana', 'Add W'), opt(1, 83, 'mana', 'Add U'), opt(2, 83, 'mana', 'Add B'),
        opt(3, 83, 'mana', 'Add R'), opt(4, 83, 'mana', 'Add G'), opt(5, 83, 'mana', 'Add C'),
      ]))).toEqual({ seq: 11, obj: 83 }); // six is still a wheel
    });

    it('arms nothing when the follow-up carries no options on the object — a tapped-out source, a target ask on other objects', () => {
      expect(resolveCardFollowUp(expected, decision(10, [
        opt(0, undefined, 'pass', 'Pass priority'),
        opt(1, 41, 'activate', 'Tap Island for mana'),
      ]))).toBeNull();
      // one option on the object is a direct-action badge, not a picker
      expect(resolveCardFollowUp(expected, decision(10, [opt(0, 83, 'mana', 'Add B')]))).toBeNull();
    });

    it('arms nothing for a >6-option follow-up (the list menu keeps its own shape)', () => {
      const d = decision(10, Array.from({ length: 7 }, (_, i) => opt(i, 83, 'mana', `Add ${i}`)));
      expect(resolveCardFollowUp(expected, d)).toBeNull();
    });

    it('an unarmed post (expected null) and a same-seq decision decode to nothing', () => {
      const d = decision(10, [opt(0, 83, 'mana', 'Add B'), opt(1, 83, 'mana', 'Add R')]);
      expect(resolveCardFollowUp(null, d)).toBeNull();
      expect(resolveCardFollowUp(expected, decision(9, [opt(0, 83, 'mana', 'Add B'), opt(1, 83, 'mana', 'Add R')]))).toBeNull();
      expect(resolveCardFollowUp(expected, null)).toBeNull();
    });
  });
});

describe('tileOptions / tileOptionsMany — the bundle reductions the components render', () => {
  const post = vi.fn();
  const bundle = (d: Decision, tone: CardOptions['tone'] = 'offered'): CardOptions => ({
    byObj: optionsByObj(d), byPlayer: optionsByPlayer(d), picked: [], tone, post,
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

  it('playerOptions marks a seat that is the ONLY legal target (no permanent to target) — the gap that had it marked nowhere at all', () => {
    const d: Decision = {
      seq: 1, player: 0, kind: 'target', prompt: 'Choose a target', min: 1, max: 1,
      options: [{ index: 4, kind: 'player', label: 'Player Bob', player: 2 }],
    };
    const b = bundle(d, 'initiative');
    expect(playerOptions(b, 2)?.list.map((o) => o.index)).toEqual([4]);
    expect(playerOptions(b, 2)?.tone).toBe('initiative');
    expect(playerOptions(b, 0)).toBeNull(); // the acting player is not itself marked
  });
});
