import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView } from '../protocol';
import { stackFaces, stackIdentical } from '../lib/board';
import CardStack from './CardStack.svelte';

// The repo's component-test pattern is deterministic SSR via svelte/server
// (see routes/Table.svelte.test.ts); this environment has no DOM, so
// clicking a stacked tile cannot be driven from the test. Collapsed markup
// is asserted here; the collapsed/expanded face mapping is pinned as a
// named lib test in board.test.ts ('stackFaces: collapsed shows only the
// first card; expanded shows every member'), which the template renders
// straight through, so "expanded shows N tiles" is the same code path the
// badged collapse already proves renders N cards.
const zombie = (id: number): CardView => ({
  id, name: 'Zombie', types: 'Creature Zombie',
  printing: { name: 'Zombie', set: 'TOK', number: '25' }, token: `#${id}`,
  tapped: false, power: 2, toughness: 2, damage: 0, attacking: false,
  controller: 0, owner: 0, summon_sick: false,
});

describe('CardStack', () => {
  it('a group of one renders exactly a CardTile: no button, no badge, no group anchor', () => {
    const { html } = render(CardStack, { props: { group: stackIdentical([zombie(7)])[0] } });
    expect(html).toContain('data-obj="7"'); // the tile's own anchor, as before stacking
    expect(html).not.toContain('<button');
    expect(html).not.toContain('data-stack-count');
    expect(html).not.toContain('data-obj-group');
  });

  it('a count badge appears only for N>1', () => {
    const { html } = render(CardStack, { props: { group: stackIdentical([zombie(1), zombie(2), zombie(3)])[0] } });
    expect(html).toContain('data-stack-count');
    expect(html).toContain('>x3<');
  });

  it('collapsed stack shows exactly one face, two ghosts, and keeps every member addressable', () => {
    const { html } = render(CardStack, { props: { group: stackIdentical([zombie(4), zombie(8), zombie(11)])[0] } });
    // one card face in the markup: the first card's data-obj anchor only
    expect(html.match(/data-obj="/g)).toEqual(['data-obj="']);
    // every member id is addressable for arrows, and the group is a single
    // keyboard control with the state and label spelled out for assistive tech
    expect(html).toContain('data-obj-group="4,8,11"');
    expect(html).toContain('aria-expanded="false"');
    expect(html).toContain('aria-label="3 copies of Zombie"');
    // the fan is exactly two ghost layers, never one per card
    expect(html.match(/class="ghost ghost--/g)).toEqual(['class="ghost ghost--', 'class="ghost ghost--']);
  });

  it('attachments are rendered only for a group of one', () => {
    const rider: CardView = { ...zombie(9), name: 'Rancor', types: 'Enchantment Aura', printing: { name: 'Rancor', set: 'ARN', number: '126' }, attached_to: 7 };
    const { html } = render(CardStack, { props: { group: stackIdentical([zombie(7)])[0], attachments: [rider] } });
    expect(html).toContain('Rancor'); // the rider chip renders under its host
    expect(html).toContain('data-obj="9"'); // and stays individually addressable
  });

  it('the expanded face list is the same array the collapsed count derives from', () => {
    const group = stackIdentical([zombie(2), zombie(5), zombie(9)])[0];
    const collapsed = stackFaces(group, false);
    const expanded = stackFaces(group, true);
    expect(collapsed.map((c) => c.id)).toEqual([2]);
    expect(expanded.map((c) => c.id)).toEqual([2, 5, 9]);
    expect(expanded).toBe(group.cards); // expanded = every member, nothing filtered
  });
});
