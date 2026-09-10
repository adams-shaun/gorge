import { mount } from 'svelte';
import type { CardView, PlayerView, SeatInfo, View } from '../protocol';
import '../app.css';
import SeatTable from './SeatTable.svelte';

const card = (id: number): CardView => ({
  id, name: `Archive Card ${id}`, types: id % 2 === 0 ? 'Creature — Wizard' : 'Instant',
  printing: { name: `Archive Card ${id}` }, token: `#${id}`, tapped: false,
  power: 0, toughness: 0, damage: 0, attacking: false, controller: 0, owner: 0,
  summon_sick: false,
});
const graveyard = Array.from({ length: 52 }, (_, i) => card(i + 1));
const player: PlayerView = {
  seat: 0, name: 'Alice', life: 37, lost: false, library_size: 40, hand_size: 1,
  graveyard_size: graveyard.length, hand: [card(100)], battlefield: [], graveyard,
  exile: [card(200)], pool: {}, command: [], commanders: [], commander_casts: [],
};
const view: View = {
  viewer: 0, visibility: 'seat', turn: 1, round: 1, step: 'main1', phase: 'main1',
  active: 0, priority: 0, over: false, draw: false, winner: null, stack: [], pending: [], players: [player],
};
const seats: SeatInfo[] = [{ name: 'Alice', deck: 'archive', colour: '#e5484d' }];

mount(SeatTable, {
  target: document.querySelector('#fixture')!,
  props: { view, seats, onFocus: () => {} },
});
