import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView, PlayerView, SeatInfo } from '../protocol';
import ZoneViewer from './ZoneViewer.svelte';

const card = (id: number, name: string, types: string): CardView => ({
  id, name, types, tapped: false, power: 0, toughness: 0, damage: 0, attacking: false,
  controller: 0, owner: 0, summon_sick: false, printing: { name }, token: `#${id}`,
});

/** A player with the wire shape the client sees; override hand/graveyard/exile as needed. */
const player = (over: Partial<PlayerView> = {}): PlayerView => ({
  seat: 0, name: 'P0', life: 40, lost: false, library_size: 30, hand_size: 7, graveyard_size: 0,
  hand: [], battlefield: [], graveyard: [], exile: [], pool: {}, command: [], commanders: [], commander_casts: [],
  ...over,
});

const seats = (n: number): SeatInfo[] =>
  Array.from({ length: n }, (_, i) => ({ name: `P${i}`, deck: `deck-${i}`, colour: `#00000${i}` }));

describe('ZoneViewer — one collapsible group per seat (ui10 bug 3)', () => {
  it('renders a group for EVERY seat, each naming its seat and carrying its colour', () => {
    const p0 = player({ seat: 0, name: 'Ari' });
    const p1 = player({ seat: 1, name: 'Bo' });
    const { html } = render(ZoneViewer, { props: { players: [p0, p1], seats: seats(2) } });
    expect(html).toContain("Ari's zones");
    expect(html).toContain("Bo's zones");
    expect(html.match(/border-left-color:/g)).toHaveLength(2);
    expect(html.match(/data-seat="/g)).toHaveLength(2);
  });

  it('starts collapsed: a one-line summary per seat with counts, and no disclosure until asked', () => {
    const p = player({ graveyard: [card(1, 'Bolt', 'Instant')], exile: [card(2, 'Bear', 'Creature')], hand_size: 7 });
    const { html } = render(ZoneViewer, { props: { players: [p], seats: seats(1) } });
    // the one-line summary carries the four counts (hand, graveyard, exile, library)
    expect(html).toContain('7 hand');
    expect(html).toContain('1 gy');
    expect(html).toContain('1 ex');
    expect(html).toContain('30 lib');
    // collapsed: the panel is not expanded, so no card is disclosed anywhere
    expect(html).not.toContain('zone-cards');
    expect(html).not.toContain('data-obj=');
    // the seat toggle exists and reports itself closed
    expect(html).toContain('aria-expanded="false"');
  });

  it('a zero-count zone renders greyed with no expander and no disclosure', () => {
    const p = player({ graveyard: [], exile: [] });
    const { html } = render(ZoneViewer, { props: { players: [p], seats: seats(1), startOpen: true } });
    expect(html).toContain('zone-row empty');
    // the library row is always there, count-only
    expect(html).toContain('>library<');
    expect(html).toContain('>30<');
    // an empty card-list zone discloses nothing (no ul of cards)
    expect(html).not.toContain('data-zone="graveyard"');
  });

  it('hand is grouped INSIDE the seat zone list, reading hand → graveyard → exile → library (ui10 bug 2)', () => {
    const p = player({ hand: [card(1, 'Isamaru', 'Legendary Creature')], graveyard: [card(2, 'Bolt', 'Instant')], exile: [card(3, 'Bear', 'Creature')] });
    const { html } = render(ZoneViewer, { props: { players: [p], seats: seats(1), startOpen: true } });
    // all four zone rows are inside the same expanded panel, in CR 400.1 zone order
    const handIdx = html.indexOf('>hand<');
    const gyIdx = html.indexOf('>graveyard<');
    const exIdx = html.indexOf('>exile<');
    const libIdx = html.indexOf('>library<');
    expect(handIdx).toBeGreaterThanOrEqual(0);
    expect(gyIdx).toBeGreaterThan(handIdx);
    expect(exIdx).toBeGreaterThan(gyIdx);
    expect(libIdx).toBeGreaterThan(exIdx);
    // the hand has its own disclosure, opened by startOpen
    expect(html).toContain('data-zone="hand"');
    expect(html).toContain('Isamaru');
  });

  it('an opponent hand this viewer may not see is a COUNT, not cards — and no expander (redaction rule)', () => {
    // Go nil slice serializes to JSON null; the public-spectator wire shape.
    const p = player({ hand: null as unknown as CardView[] });
    const { html } = render(ZoneViewer, { props: { players: [p], seats: seats(1), startOpen: true } });
    // count row, redaction, no card list for the hand
    expect(html).toContain('data-hand-count');
    expect(html).toContain('>7<'); // hand_size, still true
    // no hand disclosure and no hand cards invented
    expect(html).not.toContain('data-zone="hand"');
    expect(html).not.toContain('aria-controls="zone-list-0-hand"');
  });
});

describe('ZoneViewer — per-zone disclosure in an expanded seat', () => {
  it('a zone with cards is a toggle; an empty zone is a plain greyed row with no disclosure target', () => {
    const p = player({ graveyard: [card(1, 'Bolt', 'Instant')], exile: [] });
    const { html } = render(ZoneViewer, { props: { players: [p], seats: seats(1), startOpen: true } });
    // graveyard (has a card) is a zone-toggle with a disclosure target
    expect(html).toContain('aria-controls="zone-list-0-graveyard"');
    // exile (empty) is a plain greyed row, not a toggle, with no disclosure ul
    expect(html).toContain('zone-row empty');
    expect(html).not.toContain('zone-list-0-exile');
  });

  it('expanding lists every card, most-recently-added first, name and types per row', () => {
    const p = player({ graveyard: [card(1, 'Lightning Bolt', 'Instant'), card(2, 'Grizzly Bears', 'Creature — Bear')] });
    const { html } = render(ZoneViewer, { props: { players: [p], seats: seats(1), startOpen: true } });
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

  it('each seat section carries its seat colour and the group names the seat', () => {
    const p = player({ seat: 0, name: 'Ari', graveyard: [card(1, 'Bolt', 'Instant')] });
    const { html } = render(ZoneViewer, { props: { players: [p], seats: [{ name: 'Ari', deck: '', colour: '#eab308' }], startOpen: true } });
    expect(html).toContain("Ari's zones");
    expect(html).toContain('border-left-color: #eab308');
  });
});
