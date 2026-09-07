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

  it('renders a command group for every seat that has a roster, in seat order, and none for a seat that has not', () => {
    const { html } = render(Rail, { props: { view: view(), seats, decision: null } });
    // seats 0, 1 and 2 have rosters; seat 3 has none, so it contributes NO
    // group at all — a "no commanders" panel is a hole where the format does
    // not apply, and the rail's height is what U2 was about.
    const groups = [...html.matchAll(/data-command-zone="" data-seat="(\d)"/g)].map((m) => m[1]);
    expect(groups).toEqual(['0', '1', '2']);
    expect(html).not.toContain('data-seat="3"');
    // seat 1's commander has left the zone: that is a state, and it is stated
    expect(html.match(/data-command-zone-empty/g)).toHaveLength(1);
    // and nothing renders the absent-roster state, because nothing mounts it
    expect(html).not.toContain('data-command-empty');
    // the section exists and is named once, not once per seat
    expect(html.match(/>Commanders</g)).toHaveLength(1);
  });

  it('a constructed table renders no commanders section at all — no heading, no rows, no hole', () => {
    const v = view();
    v.players = v.players.map((p) => ({ ...p, commanders: [], command: [], commander_casts: [] }));
    const { html } = render(Rail, { props: { view: v, seats, decision: null } });
    expect(html).not.toContain('data-command-zone');
    expect(html).not.toContain('>Commanders<');
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
