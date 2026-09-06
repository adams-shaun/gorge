import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView, PlayerView } from '../protocol';
import ZoneViewer from './ZoneViewer.svelte';

const card = (id: number, name: string, types: string): CardView => ({
  id, name, types, tapped: false, power: 0, toughness: 0, damage: 0, attacking: false,
  controller: 0, owner: 0, summon_sick: false, printing: { name }, token: `#${id}`,
});

const player = (graveyard: CardView[], exile: CardView[], librarySize = 30): PlayerView => ({
  seat: 0, name: 'P0', life: 20, lost: false, library_size: librarySize, hand_size: 7, graveyard_size: graveyard.length,
  hand: [], battlefield: [], graveyard, exile, pool: {},
});

describe('ZoneViewer', () => {
  it('a zero-count zone renders greyed with no expander and no disclosure', () => {
    const { html } = render(ZoneViewer, { props: { player: player([], []), colour: '#e5484d' } });
    expect(html).toContain('zone-row empty');
    expect(html).not.toContain('aria-expanded');
    expect(html).not.toContain('zone-cards');
    // the library row is always there, count-only
    expect(html).toContain('>library<');
    expect(html).toContain('30');
  });

  it('the toggle carries aria-expanded and stays closed by default', () => {
    const { html } = render(ZoneViewer, { props: { player: player([card(1, 'Bolt', 'Instant')], []), colour: '#e5484d' } });
    expect(html).toContain('aria-expanded="false"');
    expect(html).not.toContain('zone-cards');
  });

  it('expanding lists every card, most-recently-added first, name and types per row', () => {
    const gy = [card(1, 'Lightning Bolt', 'Instant'), card(2, 'Grizzly Bears', 'Creature — Bear')];
    const { html } = render(ZoneViewer, { props: { player: player(gy, []), colour: '#22c55e', startOpen: true } });
    expect(html).toContain('aria-expanded="true"');
    expect(html).toContain('Lightning Bolt');
    expect(html).toContain('Instant');
    expect(html).toContain('Grizzly Bears');
    expect(html).toContain('Creature — Bear');
    // the last-arrived card is the first row of the disclosure
    const rows = html.split('data-obj=').slice(1);
    expect(rows.length).toBe(2);
    expect(rows[0].startsWith('"2"')).toBe(true);
    expect(rows[1].startsWith('"1"')).toBe(true);
    expect(html).toContain('data-zone="graveyard"');
  });

  it('each zone row carries its seat colour and the section names the seat', () => {
    const { html } = render(ZoneViewer, { props: { player: player([card(1, 'Bolt', 'Instant')], []), colour: '#eab308' } });
    expect(html).toContain('P0');
    expect(html).toContain('border-left-color: #eab308');
  });
});
