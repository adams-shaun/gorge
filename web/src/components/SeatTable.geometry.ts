import { mount } from 'svelte';
import type { CardView, PlayerView, SeatInfo, View } from '../protocol';
import '../app.css';
import IdentityBar from './IdentityBar.svelte';
import SeatTable from './SeatTable.svelte';
import Rail from './Rail.svelte';

const card = (id: number, name = `Card ${id}`): CardView => ({
  id, name, types: 'Instant', printing: { name }, token: `#${id}`, tapped: false,
  power: 0, toughness: 0, damage: 0, attacking: false, controller: 0, owner: 0,
  summon_sick: false,
});

const player = (seat: number, name: string): PlayerView => ({
  seat, name, life: 40, lost: false, library_size: 60, hand_size: 7, graveyard_size: 0,
  hand: [], battlefield: [], graveyard: [], exile: [], pool: {}, command: [], commanders: [], commander_casts: [],
});

const players = [player(0, 'Ari'), player(1, 'Bo')];
const seats: SeatInfo[] = [
  { name: 'Ari', deck: 'red', colour: '#e5484d' },
  { name: 'Bo', deck: 'green', colour: '#22c55e' },
];
const view = (priority: number): View => ({
  viewer: 255, visibility: 'public', turn: 1, round: 1, step: 'main1', phase: 'main1', active: 1, priority,
  over: false, draw: false, winner: null, stack: [], pending: [], players,
});

mount(SeatTable, { target: document.querySelector('#seat-idle')!, props: { view: view(1), seats, onFocus: () => {} } });
mount(SeatTable, { target: document.querySelector('#seat-priority')!, props: { view: view(0), seats, onFocus: () => {} } });
mount(IdentityBar, { target: document.querySelector('#identity-idle')!, props: { player: players[0], players, colour: '#e5484d', active: false, priority: false, corner: 'tl' } });
mount(IdentityBar, { target: document.querySelector('#identity-priority')!, props: { player: players[0], players, colour: '#e5484d', active: false, priority: true, corner: 'tl' } });

// The rail fixture mounts the WHOLE instrument column — seat table, mana pool
// focus pane, decision line, stack tile, pending tray — at a fixed width, so
// the floor the rail actually needs is measured rather than guessed. The
// content is deliberately representative: four seats, one with a name long
// enough to exercise the ellipsis, a stack entry, an optional pending trigger
// and a live decision prompt.
const longName = 'Bartholomew Q. Fineganworth';
const railPlayers = [
  player(0, longName),
  player(1, 'Ari'),
  player(2, 'Bo'),
  player(3, 'Cy'),
];
railPlayers[0].pool = { W: 3, U: 2, B: 1, R: 4, G: 0, C: 2 };
railPlayers[1].hand_size = 9;
railPlayers[1].library_size = 41;
railPlayers[1].graveyard_size = 12;
railPlayers[1].exile = [card(11), card(12)];
const railSeats: SeatInfo[] = railPlayers.map((p, i) => ({ name: p.name, deck: 'WURGc Control Mirror — long deck label ' + i, colour: '#e5484d' }));
const railView: View = {
  viewer: 255, visibility: 'public', turn: 4, round: 2, step: 'main1', phase: 'main1', active: 1, priority: 1,
  over: false, draw: false, winner: null,
  stack: [{
    id: 900, kind: 'spell', name: 'Slow but Absolutely Inevitable Zooming Doomwhisper', controller: 1,
    text: 'Deal 3 damage to any target. If a creature died this turn, draw a card.', targets: [], optional: false,
  }],
  pending: [{ source: 901, controller: 2, label: 'Longwinded Ambush Elemental trigger', optional: true, decider: 2 }],
  players: railPlayers,
};
mount(Rail, {
  target: document.querySelector('#rail')!,
  props: {
    view: railView,
    seats: railSeats,
    decision: { player: 1, kind: 'priority', prompt: 'Pass priority, cast a spell, or hold up mana for something clever later this turn?' },
    emphasizeTop: true,
    onToggleLog: () => {},
  },
});
