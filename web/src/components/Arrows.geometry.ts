import { mount } from 'svelte';
import type { PlayerView, SeatInfo, StackView, View } from '../protocol';
import '../app.css';
import Board from './Board.svelte';
import Rail from './Rail.svelte';
import Arrows from './Arrows.svelte';

/**
 * Arrows.geometry — the production table hierarchy the arrows overlay lives
 * in, at real layout. routes/Table.svelte renders <main class="table"> with a
 * clipped felt <section class="board"> (BoardStage -> Board) and the stack
 * rail <aside class="rail"> (Rail, whose section.stack is the one scroller);
 * the overlay is hosted by the table root so its box contains BOTH endpoints
 * of a stack-to-stack target arrow. This fixture mounts the production
 * components in that shell — the shell's own grid/clip CSS replicates
 * Table.svelte's scoped rules the way PhaseLane.geometry.ts replicates
 * BoardStage's host (a fixture cannot SSR the live route without a backend).
 *
 * The stack is the brief's case: a counterspell targeting the spell beneath
 * it, both rendered as rail tiles, over enough filler entries that
 * section.stack actually scrolls at the test viewport.
 */

const filler = (id: number): StackView => ({
  id, kind: 'spell', name: `Ritual ${id}`, text: '', controller: 0, targets: [], card: null, optional: false,
});

// view.stack lists bottom of the stack first; the rail renders it reversed,
// so the counterspell (900) is the UPPER rail tile and the bolt (890) the
// lower one — the pair the arrow runs between.
const stack: StackView[] = [
  ...Array.from({ length: 40 }, (_, i) => filler(100 + i)),
  { id: 890, kind: 'spell', name: 'Lightning Bolt', text: 'Lightning Bolt deals 3 damage to any target.', controller: 1, targets: [], card: null, optional: false },
  { id: 900, kind: 'spell', name: 'Counterspell', text: 'Counter target spell.', controller: 0, targets: [{ obj: 890, player: 1, is_player: false, label: 'spell' }], card: null, optional: false },
];

const players: PlayerView[] = [0, 1].map((seat) => ({
  seat, name: `Player ${seat + 1}`, life: 20, lost: false, library_size: 60, hand_size: 7,
  graveyard_size: 0, hand: [], battlefield: [], graveyard: [], exile: [], pool: {},
  command: [], commanders: [], commander_casts: [],
}));
const seats: SeatInfo[] = players.map((p) => ({ name: p.name, deck: 'fixture', colour: p.seat === 0 ? '#e5484d' : '#22c55e' }));
const view: View = {
  viewer: 255, visibility: 'omniscient', turn: 2, round: 2, step: 'main1', phase: 'main1',
  active: 0, priority: 0, over: false, draw: false, winner: null, stack, pending: [], players,
};

const app = document.querySelector('#app')!;
app.innerHTML = '<main class="table"><section class="board"></section><aside class="rail"></aside></main>';
mount(Board, { target: app.querySelector('.board')!, props: { view, seats, options: null } });
mount(Rail, { target: app.querySelector('.rail')!, props: { view, seats, decision: null } });
// The production table root hosts the overlay directly (routes/Table.svelte),
// so the fixture mounts it the same way — as a child of main.table.
mount(Arrows, { target: app.querySelector('.table')!, props: { view, options: null } });

const style = document.createElement('style');
// Replicates routes/Table.svelte's scoped shell rules: the grid, the table
// root as the overlay's containing block, and the felt section's clip.
style.textContent = `
  .table { position: relative; display: grid; grid-template-columns: 1fr minmax(11rem, 15%); grid-template-rows: 1fr; height: 100vh; background: var(--felt); }
  .table > .board { position: relative; overflow: hidden; min-width: 0; }
  .rail { position: relative; min-width: 0; background: var(--instrument); border-left: 1px solid var(--edge-inst); overflow: visible; color: var(--ink-inst); }
`;
document.head.append(style);
