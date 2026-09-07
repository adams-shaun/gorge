import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView, PlayerView } from '../protocol';
import Quadrant from './Quadrant.svelte';

/**
 * CZ2: a commander is a creature, so its tile belongs in the creatures row,
 * at the row's own --card-w (104px), not in a private rim-pinned area at
 * land scale. These tests assert the DOM structure that claim depends on —
 * the commander tile's markup falls inside the same span of html as the
 * creatures row's own CardStacks and before the others row begins — since
 * CommandArea.svelte.test.ts already covers the tile's own three states,
 * inspectability and tax.
 *
 * Task 3 (lost seats greyed on the board) is also covered here at the
 * Quadrant level, since Quadrant is what carries the `lost` class and the
 * greying scrim.
 */

const card = (id: number, name: string, types = 'Creature', manaCost?: string): CardView => ({
  id, name, types, mana_cost: manaCost,
  tapped: false, power: 2, toughness: 2, damage: 0, attacking: false,
  controller: 0, owner: 0, summon_sick: false, printing: { name }, token: `#${id}`,
});

const player = (over: Partial<PlayerView> = {}): PlayerView => ({
  seat: 0, name: 'P0', life: 40, lost: false, library_size: 90, hand_size: 7, graveyard_size: 0,
  hand: [], battlefield: [], graveyard: [], exile: [], pool: {},
  command: [], commanders: [], commander_casts: [], ...over,
});

describe('Quadrant — commanders share the creatures row at creature scale (CZ2)', () => {
  it('the commander tile and the real creature permanent both fall inside the creatures row"s own markup', () => {
    const cmd = card(1, 'Isamaru', 'Legendary Creature');
    const creature = card(2, 'Grizzly Bears');
    const p = player({ commanders: [cmd], command: [cmd], battlefield: [creature] });
    const { html } = render(Quadrant, { props: { player: p, colour: '#e5484d' } });

    const rowStart = html.indexOf('row creatures');
    const rowOthersStart = html.indexOf('row others');
    expect(rowStart).toBeGreaterThanOrEqual(0);
    expect(rowOthersStart).toBeGreaterThan(rowStart);

    const tileIdx = html.indexOf('data-cmd-state="command"');
    const creatureIdx = html.indexOf('data-obj="2"');
    expect(tileIdx).toBeGreaterThan(rowStart);
    expect(tileIdx).toBeLessThan(rowOthersStart);
    expect(creatureIdx).toBeGreaterThan(rowStart);
    expect(creatureIdx).toBeLessThan(rowOthersStart);
  });

  it('the old private rim-pinned command area is gone entirely', () => {
    const cmd = card(1, 'Isamaru');
    const p = player({ commanders: [cmd], command: [cmd] });
    const { html } = render(Quadrant, { props: { player: p, colour: '#e5484d' } });
    expect(html).not.toContain('command-area');
    expect(html).not.toContain('side-start');
    expect(html).not.toContain('side-end');
  });

  it('a seat with no commanders adds nothing to the creatures row', () => {
    const creature = card(2, 'Grizzly Bears');
    const p = player({ battlefield: [creature] });
    const { html } = render(Quadrant, { props: { player: p, colour: '#e5484d' } });
    expect(html).not.toContain('data-commander');
    expect(html).not.toContain('data-cmd-state');
  });
});

describe('Quadrant — an eliminated seat is greyed out on the board (Task 3)', () => {
  it('a lost seat carries the lost class and a data-lost flag Board/CSS key off', () => {
    const p = player({ lost: true });
    const { html } = render(Quadrant, { props: { player: p, colour: '#e5484d' } });
    expect(html).toContain('data-lost="true"');
    expect(html).toMatch(/class="quadrant[^"]*\blost\b/);
  });

  it('a live seat carries neither', () => {
    const p = player({ lost: false });
    const { html } = render(Quadrant, { props: { player: p, colour: '#e5484d' } });
    expect(html).toContain('data-lost="false"');
    expect(html).not.toMatch(/class="quadrant[^"]*\blost\b/);
  });
});
