import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView } from '../protocol';
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
