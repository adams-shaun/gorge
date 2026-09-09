import { mount } from 'svelte';
import type { PlayerView, SeatInfo, View } from '../protocol';
import '../app.css';
import IdentityBar from './IdentityBar.svelte';
import SeatTable from './SeatTable.svelte';

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
