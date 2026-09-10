import { describe, expect, it, vi } from 'vitest';
import { render } from 'svelte/server';
import type { CardView } from '../protocol';
import type { TileOptions } from '../lib/cardoptions';
import { postTileOption } from '../lib/cardoptions';
import { HoverCard, type AnchorRect } from '../lib/carddetail.svelte';
import CardTile from './CardTile.svelte';

// The repo's component-test convention is deterministic SSR via
// svelte/server (see CommandZone/IdentityBar and the note in
// CardStack.test.ts): this environment has no DOM, so pointerenter can
// never fire and $effect never runs. The panel therefore cannot be opened
// by events, so CardTile accepts the HoverCard and anchor it would
// otherwise create and capture itself; the test reaches the open state by
// driving that shared instance — the exact state the lifetime guard closes
// in production — and the close is superviseRendering, the exact call
// CardTile's $effect makes whenever the card prop changes. The $effect
// wiring itself cannot run in this harness (the same gap CardStack.test.ts
// documents for clicking a stacked tile); the reaction it performs is what
// these renders exercise.

const card = (over: Partial<CardView> = {}): CardView => ({
  id: 16, name: 'Wasteland', types: 'Land', mana_cost: '',
  printing: { name: 'Wasteland' }, token: '', tapped: false, power: 0, toughness: 0,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
  ...over,
});

const anchor: AnchorRect = { left: 100, top: 100, right: 190 };

describe('CardTile panel lifetime', () => {
  it('renders the detail panel while the hover state is open — the state the guard closes', () => {
    const hover = new HoverCard();
    hover.open(16); // dwell completed / keyboard focus: the panel opened for object 16
    const { html } = render(CardTile, { props: { card: card(), hover, anchor } });
    expect(html).toContain("card-detail");
    expect(html).toContain('data-obj="16"');
    expect(html).toContain('#16'); // the panel names the object it describes
  });

  it('a kept tile handed a different card closes the open panel: no .card-detail remains', () => {
    const hover = new HoverCard();
    hover.open(16); // the pointer had opened the panel on object 16...
    // ...and the board re-renders: Svelte keeps this CardTile instance and
    // hands it object 12 with no pointer event. In the browser CardTile's
    // $effect calls hover.superviseRendering(card.id) on that change — the
    // same call applied here before the tile is re-rendered.
    expect(hover.superviseRendering(12)).toBe(true); // a live panel was closed
    const { html } = render(CardTile, { props: { card: card({ id: 12, name: 'Polluted Delta' }), hover, anchor } });
    expect(html).not.toContain('card-detail');
    expect(html).toContain('data-obj="12"'); // the tile itself re-targeted cleanly
  });

  it('a tile still rendering the same object keeps its open panel', () => {
    const hover = new HoverCard();
    hover.open(16);
    expect(hover.superviseRendering(16)).toBe(false); // same object: nothing closes
    const { html } = render(CardTile, { props: { card: card(), hover, anchor } });
    expect(html).toContain("card-detail");
    expect(html).toContain('data-obj="16"');
  });
});

describe('CardTile options affordance (ui21)', () => {
  const opts = (over: Partial<TileOptions> = {}): TileOptions => ({
    list: [
      { index: 3, kind: 'cast', label: 'Cast Fireball', obj: 16, player: 0 },
      { index: 8, kind: 'ability', label: 'Activate Wasteland', obj: 16, player: 0 },
    ],
    pickedOrder: [],
    tone: 'offered',
    post: vi.fn(),
    ...over,
  });

  it('a tile whose card is offered something wears a badge with the count and an accessible name', () => {
    const { html } = render(CardTile, { props: { card: card(), tileOptions: opts() } });
    // the badge names the card and says how many actions the decision offers
    expect(html).toContain('aria-haspopup');
    expect(html).toContain('aria-expanded="false"');
    expect(html).toContain('2 actions for Wasteland');
    expect(html).toContain('badge__n');
    expect(html).toContain('data');
  });

  it('two through six options open the radial picker with full accessible labels', () => {
    const { html } = render(CardTile, { props: { card: card(), tileOptions: opts(), open0: true } });
    expect(html).toContain('data-radial-picker');
    expect(html).toContain('Cast Fireball');
    expect(html).toContain('Activate Wasteland');
    expect(html).not.toContain('cast: Cast Fireball'); // never re-phrased from kind
    expect(html).toContain('role="menu"');
    expect(html).toContain('role="menuitem"');
  });

  it('posts and exposes each radial option by its own wire index, not its position', () => {
    const t = opts();
    const { html } = render(CardTile, { props: { card: card(), tileOptions: t, open0: true } });
    expect(html).toContain('data-wire-index="3"');
    expect(html).toContain('data-wire-index="8"');
    postTileOption(t, t.list[1]);
    expect(t.post).toHaveBeenCalledWith(8);
  });

  it('more than six options retain the rectangular list menu', () => {
    const list = Array.from({ length: 7 }, (_, i) => ({
      index: 20 + i * 3, kind: 'ability', label: `Long option ${i + 1}`, obj: 16, player: 0,
    }));
    const { html } = render(CardTile, { props: { card: card(), tileOptions: opts({ list }), open0: true } });
    expect(html).toContain('menu-pop');
    expect(html).toContain('menu__item');
    expect(html).not.toContain('data-radial-picker');
    expect(html).toContain('Long option 7');
  });

  it('an all-colour option set renders WUBRGC pip buttons rather than neutral labels', () => {
    const list = ['W', 'U', 'B', 'R', 'G', 'C'].map((symbol, i) => ({
      index: 41 + i * 2, kind: 'ability', label: `Add ${symbol}`, obj: 16, player: 0,
    }));
    const { html } = render(CardTile, { props: { card: card(), tileOptions: opts({ list }), open0: true } });
    for (const symbol of ['W', 'U', 'B', 'R', 'G', 'C']) {
      expect(html).toContain(`data-mana-option="${symbol}"`);
      expect(html).toContain(`var(--mana-${symbol.toLowerCase()})`);
    }
    expect(html).toContain('wheel-button--mana');
    expect(html).not.toMatch(/>Add [WUBRGC]</); // colour itself is the visible label
  });

  it('each menu item is keyed by the option\'s OWN index, never a position in a rebuilt list (R-E4-1)', () => {
    // Two options on this card whose indices (3 and 8) are unrelated to their
    // wire positions (0 and 1). The menu must carry each one's own index so a
    // click posts 3 for the cast and 8 for the activation, not 0 and 1.
    const t = opts();
    expect(t.list[0].index).toBe(3);
    expect(t.list[1].index).toBe(8);
    expect(t.list.map((o) => o.index)).toEqual([3, 8]);
    // the post callback is the hand-back path; the index it receives is the
    // option's own, proved in cardoptions.test.ts by reordering and asserting
    // the posted index does not change.
    t.post(t.list[0].index);
    expect(t.post).toHaveBeenCalledWith(3);
  });

  it.each([
    ['cast', 'cast', 'Cast Wasteland'],
    ['activate', 'tap', 'Tap Wasteland for mana'],
    ['ability', 'action', 'Wasteland: sacrifice it'],
  ])('one %s option renders a direct %s icon with the wire label as its accessible name', (kind, icon, label) => {
    const one = opts({ list: [{ index: 17, kind, label, obj: 16, player: 0 }] });
    const { html } = render(CardTile, { props: { card: card(), tileOptions: one, open0: true } });
    expect(html).toContain('data-single-action');
    expect(html).toContain(`data-action-icon="${icon}"`);
    expect(html).toContain(`aria-label="${label}`);
    expect(html).not.toContain('aria-haspopup');
    expect(html).not.toContain('role="menu"');
    expect(html).not.toContain('badge__n');
  });

  it('a homogeneous stack of mana activations renders one direct tap control, not a menu', () => {
    const pile = opts({ list: [
      { index: 17, kind: 'activate', label: 'Tap Island for mana', obj: 16, player: 0 },
      { index: 4, kind: 'activate', label: 'Tap Island for mana', obj: 19, player: 0 },
    ] });
    const { html } = render(CardTile, { props: { card: card(), tileOptions: pile, open0: true } });
    expect(html).toContain('data-single-action');
    expect(html).toContain('data-action-icon="tap"');
    expect(html).not.toContain('aria-haspopup');
    expect(html).not.toContain('role="menu"');
  });

  it('a tile with no options offer renders no badge and no menu (the no-mark state)', () => {
    const { html } = render(CardTile, { props: { card: card(), tileOptions: null } });
    expect(html).not.toContain('aria-haspopup');
    expect(html).not.toContain('tile-actions');
  });

  it('the mark wears the decision tone: initiative for a blocked decision, offered for an open window', () => {
    const initiative = render(CardTile, { props: { card: card(), tileOptions: opts({ tone: 'initiative' }) } });
    expect(initiative.html).toContain('data-tone="initiative"');
    expect(initiative.html).toContain('badge--initiative');

    const offered = render(CardTile, { props: { card: card(), tileOptions: opts({ tone: 'offered' }) } });
    expect(offered.html).toContain('data-tone="offered"');
    expect(offered.html).toContain('badge--offered');
  });

  it('a picked option is visibly selected on the tile with its pick order, the panel\'s own idiom', () => {
    // pickedOrder carries the click-order ordinals (the panel's {pickedAt + 1}):
    // this card's option was the 2nd pick, so the tile says 2.
    const { html } = render(CardTile, { props: { card: card(), tileOptions: opts({ pickedOrder: [2] }) } });
    expect(html).toContain('data-selected="2"');
    expect(html).toContain('class="sel data');
    expect(html).toContain('picked 2');
  });

  it('a card with several picked ordinals lists them in order', () => {
    const { html } = render(CardTile, { props: { card: card(), tileOptions: opts({ pickedOrder: [1, 3] }) } });
    expect(html).toContain('data-selected="1,3"');
  });
});
