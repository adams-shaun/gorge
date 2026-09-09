import { mount } from 'svelte';
import type { CardView, PlayerView, SeatInfo, View } from '../protocol';
import '../app.css';
import BoardStage from './BoardStage.svelte';

const count = new URLSearchParams(location.search).get('seats') === '4' ? 4 : 2;
const colours = ['#e5484d', '#30a46c', '#4a8fd4', '#d8a24a'];
const card = (id: number, seat: number): CardView => ({
  id, name: `Commander ${seat + 1}`, types: 'Legendary Creature', mana_cost: '2 W',
  tapped: false, power: 2, toughness: 2, damage: 0, attacking: false,
  controller: seat, owner: seat, summon_sick: false, printing: { name: `Commander ${seat + 1}` }, token: `#${id}`,
});
const players: PlayerView[] = Array.from({ length: count }, (_, seat) => {
  const commander = card(seat + 1, seat);
  return {
    seat, name: `Player ${seat + 1}`, life: 40, lost: false, library_size: 90, hand_size: 7, graveyard_size: 0,
    hand: [], battlefield: [], graveyard: [], exile: [], pool: {}, command: [commander], commanders: [commander], commander_casts: [],
  };
});
const seats: SeatInfo[] = players.map((p, seat) => ({ name: p.name, deck: 'fixture', colour: colours[seat] }));
const view: View = {
  viewer: 0, visibility: 'seat', turn: 1, round: 1, step: 'main1', phase: 'main1', active: 0, priority: 0,
  over: false, draw: false, winner: null, stack: [], pending: [], players,
};

const target = document.querySelector('#app')!;
target.innerHTML = '<main class="table"><section class="stage"></section><aside></aside></main>';
mount(BoardStage, {
  target: target.querySelector('.stage')!,
  props: {
    view, seats, seat: 0,
    stops: { yours: new Set<string>(), opponents: new Set<string>() },
    onToggle: () => {},
  },
});

const style = document.createElement('style');
style.textContent = `
  .table { display:grid; grid-template-columns:1fr minmax(17rem, 18%); width:100vw; height:100vh; background:var(--felt); }
  .stage { min-width:0; overflow:hidden; }
`;
document.head.append(style);
