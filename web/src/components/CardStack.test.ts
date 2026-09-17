import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView } from '../protocol';
import { stackFaces, stackIdentical } from '../lib/board';
import type { CardOptions } from '../lib/cardoptions';
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

  // fb-20260916T201423Z: a MIXED collapsed pile carries its readiness on the
  // tab — a second mini-plate above the count — because the lands row merges
  // tapped and untapped members and the split no longer carries the signal.
  const tz = (id: number): CardView => ({ ...zombie(id), tapped: true });
  it('a mixed collapsed pile shows the readiness plate above the count', () => {
    // two untapped, one tapped: tab reads x3 with "2 ready" above it
    const mixed = stackIdentical([zombie(1), zombie(2), tz(3)], { ignoreTapped: true })[0];
    const { html } = render(CardStack, { props: { group: mixed } });
    expect(html).toContain('data-stack-ready');
    expect(html).toContain('>2 ready<');
    expect(html).toContain('>x3<');
  });

  it('a uniform pile tab is unchanged: no readiness plate whether all untapped or all tapped', () => {
    const allUntapped = stackIdentical([zombie(1), zombie(2)])[0];
    expect(render(CardStack, { props: { group: allUntapped } }).html).not.toContain('data-stack-ready');
    const allTapped = stackIdentical([tz(1), tz(2)], { ignoreTapped: true })[0];
    expect(render(CardStack, { props: { group: allTapped } }).html).not.toContain('data-stack-ready');
    expect(render(CardStack, { props: { group: allTapped } }).html).toContain('>x2<');
  });

  it('a group of one never carries a readiness plate', () => {
    expect(render(CardStack, { props: { group: stackIdentical([tz(7)])[0] } }).html)
      .not.toContain('data-stack-ready');
  });

  it('a collapsed mixed pile keeps the union options: only the untapped member is offered an activation and the pile is still actionable', () => {
    // The pile merged a tapped member with an untapped one; the pending
    // decision offers ONLY the untapped member a mana activation. The pile
    // speaks for every member (tileOptionsMany union), so the collapsed tile
    // carries that option — no change was needed for this, and this pins it.
    const mixed = stackIdentical([tz(1), zombie(2)], { ignoreTapped: true })[0];
    const bundle: CardOptions = {
      byObj: new Map([[2, [{ kind: 'activate', label: 'Tap Zombie for mana', index: 5, obj: 2, player: 0 }]]]),
      byPlayer: new Map(),
      picked: [],
      tone: 'offered',
      post: () => {},
    };
    const { html } = render(CardStack, { props: { group: mixed, options: bundle } });
    expect(html).toContain('data-options="1"'); // the union is non-empty: the pile is marked actionable
    expect(html).toContain('data-tone="offered"');
  });

  // fb-20260917T004545Z: a merged lands pile mixes tapped states, and the
  // collapsed face used to show the LEAD member's own rotation — so one
  // member's tap rotated the whole pile's silhouette while the tap badge (the
  // union: the next tap takes the next READY member) stayed up. The collapsed
  // face now presents the PILE's readiness: rotated only when EVERY member is
  // tapped. The lead member stays the face, the data-obj anchor (what arrows
  // target) and the inspector's subject — only the rotation is presentation.
  it('a collapsed MIXED pile presents ready even when its lead member is tapped — no whole-pile rotation', () => {
    // the reported shape: lead 1 tapped by the one click, 2 still ready
    const mixed = stackIdentical([tz(1), zombie(2)], { ignoreTapped: true })[0];
    const { html } = render(CardStack, { props: { group: mixed } });
    expect(html).not.toMatch(/class="card-tile[^"']*tapped/); // the face is NOT rotated
    expect(html).toContain('data-stack-ready'); // the ready plate carries the counts
    expect(html).toContain('>1 ready<');
    expect(html).toContain('>x2<');
    // the lead member stays the face and the anchor: presentation only
    expect(html).toContain('data-obj="1"');
    expect(html).toContain('data-obj-group="1,2"');
  });

  it('a collapsed uniform pile is unchanged: all-tapped renders rotated with no ready plate, all-untapped renders ready', () => {
    const allTapped = stackIdentical([tz(1), tz(2)], { ignoreTapped: true })[0];
    const tappedHtml = render(CardStack, { props: { group: allTapped } }).html;
    expect(tappedHtml).toMatch(/class="card-tile[^"']*tapped/); // genuinely inert: the pile shows it
    expect(tappedHtml).not.toContain('data-stack-ready');
    const allReady = stackIdentical([zombie(1), zombie(2)], { ignoreTapped: true })[0];
    const readyHtml = render(CardStack, { props: { group: allReady } }).html;
    expect(readyHtml).not.toMatch(/class="card-tile[^"']*tapped/);
    expect(readyHtml).not.toContain('data-stack-ready'); // uniform: no plate, as before
  });

  it('a collapsed mixed pile presents rotated again once its LAST member taps (the pile is then inert)', () => {
    const all = stackIdentical([tz(1), tz(2)], { ignoreTapped: true })[0];
    expect(render(CardStack, { props: { group: all } }).html).toMatch(/class="card-tile[^"']*tapped/);
  });
});
