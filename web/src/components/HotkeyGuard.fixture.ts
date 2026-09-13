import { mount, unmount } from 'svelte';
import type { CardView, Decision, Option, PlayerView, SeatInfo, View } from '../protocol';
import { SeatPanelState } from '../lib/seatpanel.svelte';
import '../app.css';
import HotButtonStrip from './HotButtonStrip.svelte';
import PileModal from './PileModal.svelte';
import CardTile from './CardTile.svelte';

/**
 * HotkeyGuard fixture mounts the REAL hotkey wiring — HotButtonStrip (whose
 * onMount owns the document-level hotkeys) with a real SeatPanelState — so a
 * mounted test can drive keyboard presses at the live document and observe
 * what the grammar + modal guard actually do:
 *
 *  - window.__posts: every intent POST body the seat panel sent;
 *  - window.__state / __view / __armRun: programmatic End Turn arming,
 *    because opening the radial picker by pointer would itself cancel a run
 *    (the panel's pointerdown capture is correct, pre-existing behaviour);
 *  - window.__openPile: mounts a real PileModal (open), whose onClose
 *    unmounts it — the marker [data-pile-modal] appears/disappears with it;
 *  - real three- and seven-option CardTiles exercise both portaled
 *    OptionPicker shapes and their shared [data-option-picker] guard.
 */

interface FixtureWindow {
  __posts: unknown[];
  __state: SeatPanelState;
  __view: View;
  __armRun: () => void;
  __openPile: () => void;
  /** adopts a FRESH decision (next seq), so a test gets a second answerable window — the stub always serves seq 7 otherwise. */
  __newDecision: () => void;
}

const win = window as unknown as FixtureWindow;

const decision: Decision = {
  seq: 7, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1,
  options: [
    { index: 5, kind: 'cast', label: 'Cast Bolt', obj: 11, player: 0 } as Option,
    { index: 9, kind: 'pass', label: 'Pass priority', player: 0 } as Option,
  ],
};

// The stub is installed at module top, before any mount, so SeatPanel's
// mount-time refreshPending and every intent POST land here instead of the
// (absent) backend. The pending GET re-serves the same decision — the same
// seq, so adopt is a no-op — and everything else 404s.
const posts: unknown[] = [];
win.__posts = posts;
window.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
  const u = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
  const method = init?.method ?? 'GET';
  if (method === 'POST' && u.includes('/intent')) {
    posts.push(init?.body === undefined ? null : JSON.parse(String(init.body)));
    return new Response(null, { status: 200 });
  }
  if (u.includes('/pending')) {
    return new Response(JSON.stringify(decision), { status: 200, headers: { 'Content-Type': 'application/json' } });
  }
  return new Response(JSON.stringify({ code: 'http', message: 'fixture stub' }), { status: 404, headers: { 'Content-Type': 'application/json' } });
}) as typeof window.fetch;

const card = (id: number, name: string): CardView => ({
  id, name, types: 'Instant', mana_cost: '',
  printing: { name }, token: '', tapped: false, power: 0, toughness: 0,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
});

const player: PlayerView = {
  seat: 0, name: 'Ari', life: 20, lost: false, library_size: 53, hand_size: 0,
  graveyard_size: 0, hand: [], battlefield: [], graveyard: [], exile: [], pool: {},
  command: [], commanders: [], commander_casts: [],
};
const seats: SeatInfo[] = [{ name: 'Ari', deck: 'deck', colour: '#e5484d' }];
const view: View = {
  viewer: 0, visibility: 'seat', turn: 1, round: 1, step: 'main1', phase: 'main1',
  active: 0, priority: 0, over: false, draw: false, winner: null,
  players: [player], stack: [], pending: [], decision,
};
win.__view = view;

// Manual seat: auto off, pass-after-acting off, empty-window floor off, no
// stops — the player answers everything, so every POST the fixture observes
// was caused by the key or click under test, never by the machine paths.
const state = new SeatPanelState('fx', 1, { seat: 0, token: 'tok' }, null);
state.skipEmpty = false;
state.setAuto(false);
state.setActPass(false);
state.adoptView(decision);
win.__state = state;

win.__armRun = () => {
  state.startEndTurn(view);
  state.considerAuto(view);
};

mount(HotButtonStrip, {
  target: document.querySelector('#fixture')!,
  props: { view, seats, state, ctx: { seat: 0, token: 'tok' }, table: 'fx', match: 1 },
});

// The pile modal is mounted imperatively so the fixture starts closed and a
// test opens exactly what it exercises (hand/graveyard/exile dialog).
let pile: ReturnType<typeof mount> | null = null;
win.__openPile = () => {
  if (pile !== null) return;
  pile = mount(PileModal, {
    target: document.querySelector('#pile')!,
    props: {
      open: true,
      title: "Ari's graveyard",
      cards: [card(31, 'Bolt'), card(32, 'Shock')],
      returnFocus: null,
      onClose: () => {
        if (pile !== null) void unmount(pile);
        pile = null;
      },
    },
  });
};

let nextSeq = 8;
win.__newDecision = () => {
  state.adoptView({ ...decision, seq: nextSeq++ });
};

// The radial card-action picker: clicking the badge opens it (a real
// OptionPicker open), which is the modal the Escape grammar must respect.
mount(CardTile, {
  target: document.querySelector('#radial')!,
  props: {
    card: card(16, 'Wasteland'),
    tileOptions: {
      list: [
        { index: 3, kind: 'cast', label: 'Cast Wasteland', obj: 16, player: 0 } as Option,
        { index: 8, kind: 'ability', label: 'Activate Wasteland', obj: 16, player: 0 } as Option,
        { index: 12, kind: 'ability', label: 'Wasteland: sacrifice it', obj: 16, player: 0 } as Option,
      ],
      pickedOrder: [],
      tone: 'offered',
      post: () => {},
    },
  },
});

// Seven options select OptionPicker's rectangular list branch. It is a modal
// in exactly the same sense as the radial wheel: the portaled list covers the
// active card decision until the player picks or closes it.
mount(CardTile, {
  target: document.querySelector('#menu')!,
  props: {
    card: card(17, 'Many Modes'),
    tileOptions: {
      list: Array.from({ length: 7 }, (_, i) => ({
        index: 20 + i,
        kind: 'ability',
        label: `Activate mode ${i + 1}`,
        obj: 17,
        player: 0,
      } as Option)),
      pickedOrder: [],
      tone: 'offered',
      post: () => {},
    },
  },
});
