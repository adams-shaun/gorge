import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import type { CardView, PlayerView, SeatInfo, View } from '../protocol';
import CommandZone from './CommandZone.svelte';
import Rail from './Rail.svelte';

const card = (id: number, name: string, manaCost?: string): CardView => ({
  id, name, types: 'Legendary Creature', mana_cost: manaCost,
  tapped: false, power: 0, toughness: 0, damage: 0, attacking: false,
  controller: 0, owner: 0, summon_sick: false, printing: { name }, token: `#${id}`,
});

const player = (over: Partial<PlayerView> = {}): PlayerView => ({
  seat: 0, name: 'P0', life: 20, lost: false, library_size: 30, hand_size: 7, graveyard_size: 0,
  hand: [], battlefield: [], graveyard: [], exile: [], pool: {},
  command: [], commanders: [], commander_casts: [], ...over,
});

describe('CommandZone', () => {
  it('a commander in the command zone is marked command zone and shows its cast cost; one on the battlefield is marked battlefield with no cost', () => {
    const inZone = card(1, 'Isamaru', '1 W');
    const onBoard = card(2, 'Soldier', 'W');
    const p = player({ commanders: [inZone, onBoard], command: [inZone], battlefield: [onBoard] });
    const { html } = render(CommandZone, { props: { player: p, colour: '#e5484d' } });
    expect(html).toContain('data-commander="0"');
    expect(html).toContain('data-cmd-zone="command"');
    expect(html).toContain('data-cmd-zone="battlefield"');
    // the castable commander's cost renders as pips (1 and W), the away one
    // offers no cost because there is no command-zone cast to price
    expect(html).toContain('>1<');
    expect(html).toContain('>W<');
    expect(html).not.toContain('data-tax=');
  });

  it('a roster with nothing in the command zone says so explicitly — an empty zone is a state, not a bug', () => {
    const cast = card(1, 'Isamaru');
    const p = player({ commanders: [cast], battlefield: [cast], commander_casts: [1] });
    const { html } = render(CommandZone, { props: { player: p, colour: '#22c55e' } });
    expect(html).toContain('data-command-zone-empty');
    expect(html).not.toContain('data-cmd-zone="command"');
  });

  it('a Constructed seat with no commanders renders an explicit no-commanders state', () => {
    const { html } = render(CommandZone, { props: { player: player(), colour: '#22c55e' } });
    expect(html).toContain('data-command-empty');
    expect(html).not.toContain('<li');
  });

  it('a commander on the stack (a cast in progress) is not marked command zone', () => {
    const cast = card(1, 'Isamaru');
    const p = player({ commanders: [cast] });
    const { html } = render(CommandZone, {
      props: { player: p, colour: '#22c55e', stack: [{ id: 1, kind: 'spell', name: 'Isamaru', text: '', controller: 0, targets: [], card: cast }] },
    });
    expect(html).toContain('data-cmd-zone="stack"');
    expect(html).not.toContain('data-cmd-zone="command"');
  });
});

describe('a four-seat game renders every seat"s command zone (rail mount)', () => {
  const seats: SeatInfo[] = [0, 1, 2, 3].map((i) => ({ name: `S${i}`, deck: `deck-${i}`, colour: `#00000${i}` }));
  const view = (): View => ({
    viewer: 4, visibility: 'public', turn: 5, step: 'main1', phase: 'main1', active: 0, priority: 0,
    over: false, draw: false, winner: null, stack: [], pending: [],
    players: [
      // seat 0: commander still in the zone, one prior cast
      player({ seat: 0, name: 'S0', commanders: [card(10, 'Isamaru', 'W')], commander_casts: [1], command: [card(10, 'Isamaru', 'W')] }),
      // seat 1: commander already on the battlefield — command zone empty
      player({ seat: 1, name: 'S1', commanders: [card(11, 'Zur', '1 U U')], commander_casts: [2], battlefield: [card(11, 'Zur', '1 U U')] }),
      // seat 2: two commanders, one in the zone after three prior casts
      player({
        seat: 2, name: 'S2',
        commanders: [card(12, 'Edgar', '2 W B'), card(13, 'Liliana', '2 B B')],
        commander_casts: [3, 0],
        command: [card(12, 'Edgar', '2 W B')], graveyard: [card(13, 'Liliana', '2 B B')],
      }),
      // seat 3: no commanders at all — the format-less seat
      player({ seat: 3, name: 'S3' }),
    ],
  });

  it('renders one command section per seat, including the empty and the absent ones', () => {
    const { html } = render(Rail, { props: { view: view(), seats, decision: null } });
    // every seat, in seat order
    const sections = [...html.matchAll(/data-command-zone="" data-seat="(\d)"/g)].map((m) => m[1]);
    expect(sections).toEqual(['0', '1', '2', '3']);
    // seat 0's zone is occupied; seats 1 and 3 have no command-zone commander
    expect(html).toContain('data-command-zone="" data-seat="0"');
    expect(html).toContain('data-command-zone="" data-seat="1"');
    expect(html).toContain('data-command-zone="" data-seat="2"');
    expect(html).toContain('data-command-zone="" data-seat="3"');
    expect(html.match(/data-command-zone-empty/g)).toHaveLength(1);
    expect(html.match(/data-command-empty/g)).toHaveLength(1);
  });

  it('the displayed next-cast tax is the derived number — {2} × commander_casts — never the raw count', () => {
    const { html } = render(Rail, { props: { view: view(), seats, decision: null } });
    // seat 2's Edgar has THREE prior casts: the tax must read 6, not 3.
    expect(html).toContain('data-tax="6"');
    expect(html).toContain('data-casts="3"');
    expect(html).toContain('>+6<');
    expect(html).not.toContain('data-tax="3"');
    // seat 0's Isamaru has one prior cast: +2, and the printed cost still renders
    expect(html).toContain('data-tax="2"');
    expect(html).toContain('data-casts="1"');
    expect(html).toContain('>+2<');
    // seat 2's Liliana has never been cast: no tax chip at all
    expect(html.match(/data-tax=/g)).toHaveLength(2);
    // the commander's own cost is offered (the decide-with pips)
    expect(html).toContain('>2<');
    expect(html).toContain('>W<');
    expect(html).toContain('>B<');
  });

  it('only the castable commander carries the command-zone marker — a battlefield commander reads as away', () => {
    const { html } = render(Rail, { props: { view: view(), seats, decision: null } });
    expect(html.match(/data-cmd-zone="command"/g)).toHaveLength(2); // seats 0 and 2
    expect(html).toContain('data-cmd-zone="battlefield"'); // seat 1
    expect(html).toContain('data-cmd-zone="graveyard"'); // seat 2's dead one
  });
});
