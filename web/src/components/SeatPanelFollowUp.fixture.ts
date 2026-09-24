import { mount } from 'svelte';
import type { CardView, Decision } from '../protocol';
import { SeatPanelState } from '../lib/seatpanel.svelte';
import { resolveCardFollowUp } from '../lib/cardoptions';
import CardTile from './CardTile.svelte';

/**
 * SeatPanelFollowUp fixture is the panel-path twin of CardMenu.fixture.ts's
 * fb-e079def5 stage-1 → stage-2 test. Its point is the SURFACE, not the
 * decode: the captured treasure flow's activate option is posted through the
 * REAL SeatPanelState.click (the path SeatPanel.svelte's own option buttons
 * use), the panel's post arms followUpExpected, and Table.svelte's effect
 * body — decode with resolveCardFollowUp, then hand the object to the tile as
 * autoOpen — is reproduced here because a fixture page cannot run the route
 * component's runes. CardMenu.fixture.ts already proves the decode and the
 * OptionPicker open path; this fixture proves the ARM now reaches them from
 * the panel.
 *
 * window.__armState exposes the armed expectation so the test can await the
 * network round trip (the fixture's stubbed fetch) before decoding. The
 * stage-2 remount is deferred one macrotask, exactly as CardMenu.fixture.ts
 * defers it: synchronously it would happen inside the click's bubble and the
 * portaled radial's window-level close-on-click would shut the freshly opened
 * wheel again — an artifact the real frame-driven flow never has.
 */

// A real SeatPanelState posts through the production api dispatcher; stub
// window.fetch so the accepted intent lands without a server (the same trick
// FeedbackButton.fixture.ts uses). The staging decision is adopted directly,
// so /pending is never read.
window.fetch = async () =>
  new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } });

const ctx = { seat: 0, token: 'fixture-token' };
const panel = new SeatPanelState('fixture-treasure', 1, ctx, null, null);

// The captured priority decision (capture seq 688): two identical Treasure
// activations (obj 205, 207), pass, concede. min == max == 1, so a click IS
// the answer and posts.
const priority: Decision = {
  seq: 688, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1,
  options: [
    { index: 0, kind: 'activate', label: 'Activate Treasure Token for mana', obj: 205, player: 0 },
    { index: 1, kind: 'activate', label: 'Activate Treasure Token for mana', obj: 207, player: 0 },
    { index: 2, kind: 'pass', label: 'Pass priority', player: 0 },
    { index: 3, kind: 'concede', label: 'Concede', player: 0 },
  ],
};
panel.adoptView(priority);

const panelRoot = document.querySelector('#panel')!;
for (const opt of priority.options) {
  const button = document.createElement('button');
  button.type = 'button';
  button.id = `panel-option-${opt.index}`;
  button.textContent = opt.label;
  // SeatPanel.svelte's own option-button shape: a plain click goes straight
  // to logic.click(opt.index), no follow-up flag, no help from the tile path.
  button.addEventListener('click', () => panel.click(opt.index));
  panelRoot.appendChild(button);
}

// The captured follow-up colour ask (capture seq 693): five Kind "mana"
// options all on obj 207 — the 2-6 all-mana shape resolveCardFollowUp opens.
const choose: Decision = {
  seq: 693, player: 0, kind: 'choose', prompt: 'Add 1 mana of any one color — choose the colour', min: 1, max: 1,
  source: 207,
  options: [
    { index: 0, kind: 'mana', label: 'Add W', obj: 207, player: 0 },
    { index: 1, kind: 'mana', label: 'Add U', obj: 207, player: 0 },
    { index: 2, kind: 'mana', label: 'Add B', obj: 207, player: 0 },
    { index: 3, kind: 'mana', label: 'Add R', obj: 207, player: 0 },
    { index: 4, kind: 'mana', label: 'Add G', obj: 207, player: 0 },
  ],
};

const card = (id: number, name: string, types: string): CardView => ({
  id, name, types, mana_cost: '',
  printing: { name }, token: '', tapped: false, power: 0, toughness: 0,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
});

function renderStageTwo(): void {
  document.querySelector('#followup')!.replaceChildren();
  mount(CardTile, {
    target: document.querySelector('#followup')!,
    props: {
      card: card(207, 'Treasure Token', 'Artifact Token'),
      tileOptions: {
        list: choose.options,
        pickedOrder: [],
        tone: 'initiative',
        autoOpen: true,
        post: () => {},
      },
    },
  });
}

declare global {
  interface Window {
    __armState: () => { seq: number; obj: number } | null;
    __decode: () => { seq: number; obj: number } | null;
  }
}

window.__armState = () => panel.followUpExpected;
// Table.svelte's $effect body, reproduced: read the panel's armed
// expectation, decode it against the next decision, clear it, and remount the
// tile open when the decode opens it. Same sequencing guard — a same-seq
// decision leaves the expectation armed.
window.__decode = () => {
  const expected = panel.followUpExpected;
  if (expected === null || choose.seq === expected.seq) return null;
  const open = resolveCardFollowUp(expected, choose);
  panel.followUpExpected = null;
  if (open !== null) setTimeout(renderStageTwo, 0);
  return open;
};
