import { describe, expect, it } from 'vitest';
import type { Decision, Option } from '../protocol';
import { isSearchPick, searchCard, searchMatches, searchOptions } from './search';

// search.ts is the library-search ask's pure half (fb-20260916T181754Z):
// the shape predicate, the display filter and the display sort. The
// decisions here are built by hand from the wire shape the engine poses
// (effects/zone.go effSearchLibrary): a KChoose whose options are kind
// "search" with Obj set and the card's bare name as Label, in LIBRARY scan
// order — deliberately unalphabetical below so the sort has something to do.

const opt = (index: number, label: string, obj?: number): Option =>
  ({ index, kind: 'search', label, obj, player: 1 });

/** A library in scan order: nothing about it is alphabetical. */
const library: Decision = {
  seq: 9,
  player: 1,
  kind: 'choose',
  prompt: 'Search your library for a card',
  min: 0,
  max: 1,
  options: [
    opt(0, 'Wooded Foothills', 401),
    opt(1, 'an Island', 402),
    opt(2, 'Mistveil Plains', 403),
    opt(3, "Burning Sun's Avatar", 404),
    opt(4, 'forest', 405),
    opt(5, 'a card', 406),
  ],
};

describe('isSearchPick — the library-search shape', () => {
  it('a decision whose every option is a search pick with an Obj is one', () => {
    expect(isSearchPick(library)).toBe(true);
  });

  it('null and empty decisions are not (the fail-to-find-only Min 0 shape renders generically)', () => {
    expect(isSearchPick(null)).toBe(false);
    expect(isSearchPick({ ...library, options: [] })).toBe(false);
  });

  it('a mixed decision — any option of another kind — falls through to the generic list', () => {
    const mixed: Decision = {
      ...library,
      options: [opt(0, 'Wooded Foothills', 401), { index: 1, kind: 'pass', label: 'Pass', player: 1 }],
    };
    expect(isSearchPick(mixed)).toBe(false);
  });

  it('an option with no Obj is not a card pick: the decision stays generic', () => {
    const noObj: Decision = { ...library, options: [opt(0, 'Wooded Foothills'), opt(1, 'an Island', 402)] };
    expect(isSearchPick(noObj)).toBe(false);
  });

  it('another card-pick kind ("discard") is not the search shape — the two stay disjoint', () => {
    const discard: Decision = {
      ...library,
      options: [{ index: 0, kind: 'discard', label: 'Discard Bear', obj: 7, player: 1 }],
    };
    expect(isSearchPick(discard)).toBe(false);
  });

  it('a concede-only decision is not one', () => {
    const concedeOnly: Decision = {
      ...library,
      options: [{ index: 0, kind: 'concede', label: 'Concede', player: 1 }],
    };
    expect(isSearchPick(concedeOnly)).toBe(false);
  });
});

describe('searchMatches — the case-insensitive substring filter', () => {
  it('an empty or whitespace-only filter matches everything', () => {
    expect(searchMatches(' anything ', '')).toBe(true);
    expect(searchMatches('anything', '   ')).toBe(true);
  });

  it('a substring matches case-insensitively, anywhere in the label', () => {
    expect(searchMatches('Wooded Foothills', 'foo')).toBe(true);
    expect(searchMatches('Wooded Foothills', 'FOOTH')).toBe(true);
    expect(searchMatches('an Island', 'isla')).toBe(true);
    expect(searchMatches('Mistveil Plains', 'veil pla')).toBe(true);
  });

  it('a non-substring does not match', () => {
    expect(searchMatches('Wooded Foothills', 'delta')).toBe(false);
  });
});

describe('searchOptions — the display list', () => {
  it('an empty filter returns every option sorted A→Z, case-insensitively', () => {
    const shown = searchOptions(library, '');
    expect(shown.map((o) => o.label)).toEqual([
      'a card',
      'an Island',
      "Burning Sun's Avatar",
      'forest',
      'Mistveil Plains',
      'Wooded Foothills',
    ]);
    expect(shown.map((o) => o.index)).toEqual([5, 1, 3, 4, 2, 0]);
  });

  it("the sort never mutates the decision's own option order (the wire stays library order)", () => {
    const before = library.options.map((o) => o.index);
    searchOptions(library, '');
    expect(library.options.map((o) => o.index)).toEqual(before);
  });

  it('the filter narrows the display list only; the survivors keep their sorted order and wire indexes', () => {
    const shown = searchOptions(library, 'e');
    expect(shown.map((o) => o.index)).toEqual([4, 2, 0]);
    expect(shown.map((o) => o.label)).toEqual(['forest', 'Mistveil Plains', 'Wooded Foothills']);
    expect(shown.every((o) => o.obj !== undefined)).toBe(true);
  });

  it('a filter matching nothing yields an empty display list (the grid shows its no-match line)', () => {
    expect(searchOptions(library, 'delta')).toEqual([]);
  });

  it('two identical labels (copies of one card) keep their offered library order — the sort is stable', () => {
    const copies: Decision = {
      ...library,
      options: [opt(0, 'Grizzly Bears', 501), opt(1, 'Grizzly Bears', 502), opt(2, 'Aven Cloudchaser', 503)],
    };
    expect(searchOptions(copies, '').map((o) => o.index)).toEqual([2, 0, 1]);
  });

  it("the wire indexes are the engine's, whatever the display order — a click posts the option's own index", () => {
    const shown = searchOptions(library, '');
    // The alphabetically FIRST card is index 5 on the wire; clicking it in
    // display order must post 5, never a display position.
    expect(shown[0].index).toBe(5);
  });
});

describe('searchCard — the synthesized face', () => {
  it("builds the card from the option's own label (the bare card name) and Obj", () => {
    const o = library.options[0];
    const card = searchCard(library, o);
    expect(card.id).toBe(401);
    expect(card.name).toBe('Wooded Foothills');
    expect(card.printing.name).toBe('Wooded Foothills');
    expect(card.controller).toBe(1);
  });

  it('an unnamed library object ("a card") renders as exactly that label', () => {
    const card = searchCard(library, library.options[5]);
    expect(card.name).toBe('a card');
  });
});
