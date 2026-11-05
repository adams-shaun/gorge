import { mount } from 'svelte';
import '../../app.css';
import BetweenGames from './BetweenGames.svelte';
import type { DeckRequest, EventSeries } from '../../protocol';

// The between-games panel against a recording fetch double: the pool (with
// the deck played last game), an echoing deck check, and a sideboard route
// whose status the test sets (window.sideboardStatus) before it clicks.
// onstale counts into window.staleCalls; onstate records the adopted state.
type W = { sideboardBodies: unknown[]; sideboardStatus: number; staleCalls: number; adopted: unknown[] };
const w = window as unknown as W;
w.sideboardBodies = [];
w.sideboardStatus = 200;
w.staleCalls = 0;
w.adopted = [];

const card = (id: string, name: string, cost: string, type: string) => ({ id, name, set: 'FDN', number: '1', rarity: 'C', mana_cost: cost, mana_value: 1, colors: 'G', type_line: type });
const pool = {
  event: 'e1',
  seat: 0,
  pool: [card('c1', 'Llanowar Elves', 'G', 'Creature Elf Druid'), card('c2', 'Giant Growth', 'G', 'Instant'), card('c3', 'Lightning Bolt', 'R', 'Instant')],
  deck: { main: ['c1', 'c2'], basics: [0, 0, 0, 0, 38] },
  rev: 2,
  ready: true,
  playing: { main: ['c1', 'c2'], basics: [0, 0, 0, 0, 38] },
  playing_rev: 2,
  suggestion: { main: ['c1'], basics: [0, 0, 0, 0, 39] },
};

window.fetch = async (input, init) => {
  const url = String(input);
  const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
  if (url.endsWith('/api/events/e1/pool')) return json(200, pool);
  if (url.endsWith('/api/events/e1/deck/check')) {
    const d = JSON.parse(String(init?.body)) as DeckRequest;
    const total = d.main.length + d.basics.reduce((a, n) => a + n, 0);
    return json(200, { valid: total >= 40, total, error: total >= 40 ? undefined : `build: deck has ${total} cards, needs at least 40`, auto_lands: [0, 0, 0, 0, 0] });
  }
  if (url.endsWith('/api/events/e1/sideboard')) {
    w.sideboardBodies.push(JSON.parse(String(init?.body)));
    if (w.sideboardStatus === 409) return json(409, { code: 'conflict', message: 'eventhost: seat 0 already sideboarded for game 2' });
    if (w.sideboardStatus === 422) return json(422, { code: 'invalid_deck', message: 'build: Lightning Bolt is not in your pool' });
    return json(200, { event: 'e1', phase: 'ROUND1', round: 1, best_of: 3, seats: [], you: 0, series: [], standings: [] });
  }
  return json(404, {});
};

const rebuildWindow = new URLSearchParams(location.search).get('game') === '1';
const series: EventSeries = rebuildWindow
  ? { round: 2, a: 0, b: 2, best_of: 3, state: 'between', game: 1, games: [], chooser: null, waiting: [0, 2], score: [0, 0], winner: null }
  : { round: 1, a: 0, b: 3, best_of: 3, state: 'between', table: 'e1-r1-s03-g1-a1', game: 2, games: [3], chooser: 0, waiting: [0, 3], score: [0, 1], winner: null };
const seats = [0, 1, 2, 3].map((s) => ({ seat: s, kind: s === 0 ? 'human' : 'bot', name: s === 0 ? 'Ada' : `Bot ${s + 1}`, joined: true, picks: 42, ready: true }));

mount(BetweenGames, {
  target: document.getElementById('app')!,
  props: {
    event: 'e1',
    token: 'evtok0',
    you: 0,
    series,
    seats,
    onstate: (st) => w.adopted.push(st),
    onstale: () => {
      w.staleCalls++;
    },
  },
});
