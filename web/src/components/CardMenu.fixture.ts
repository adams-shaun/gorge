import { mount } from 'svelte';
import type { CardView, Decision, Option } from '../protocol';
import { resolveCardFollowUp, type TileOptions } from '../lib/cardoptions';
import '../app.css';
import CardTile from './CardTile.svelte';

/**
 * CardMenu fixture mounts the real CardTile/OptionPicker affordances with a
 * recording post callback, so a mounted test can drive REAL clicks — plain
 * and Ctrl-held — through the same tile path Table.svelte's boardOptions.post
 * finally hands to SeatPanelState.click(index, { holdPriority }). Every post
 * lands in window.__posted as [index, expectFollowUp, holdPriority].
 */

const card = (id: number, name: string, types: string): CardView => ({
  id, name, types, mana_cost: '',
  printing: { name }, token: '', tapped: false, power: 0, toughness: 0,
  damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
});

const posted: [number, boolean, boolean][] = [];
(window as unknown as { __posted: [number, boolean, boolean][] }).__posted = posted;

const opt = (index: number, kind: string, label: string): Option => ({ index, kind, label, obj: 16, player: 0 });

function tile(list: Option[]): TileOptions {
  return {
    list,
    pickedOrder: [],
    tone: 'offered',
    post: (index: number, expectFollowUp = false, holdPriority = false) => {
      posted.push([index, expectFollowUp, holdPriority]);
    },
  };
}

// Three options: the seeded-open radial wheel (the 2..6 option shape).
mount(CardTile, {
  target: document.querySelector('#radial')!,
  props: {
    card: card(16, 'Wasteland', 'Land'),
    tileOptions: tile([
      { index: 3, kind: 'cast', label: 'Cast Wasteland', obj: 16, player: 0 },
      { index: 8, kind: 'ability', label: 'Activate Wasteland', obj: 16, player: 0 },
      { index: 12, kind: 'ability', label: 'Wasteland: sacrifice it', obj: 16, player: 0 },
    ]),
  },
});

// One option: the direct-action icon (OptionPicker posts it with
// expectFollowUp — the Underground Sea continuation idiom).
mount(CardTile, {
  target: document.querySelector('#single')!,
  props: {
    card: card(17, 'Fireball', 'Instant'),
    tileOptions: tile([opt(21, 'cast', 'Cast Fireball')]),
  },
});

// fb-e079def5: the two-stage mana continuation, the reported Talisman of
// Indulgence flow. Stage 1 is the ability wheel the player answers THROUGH
// the picker (postTileOption arms the follow-up expectation); the post
// callback plays Table.svelte's other half — decode the follow-up decision
// through resolveCardFollowUp and render stage 2 with autoOpen when the
// decode opens it. Pre-fix, stage 1's post carried expectFollowUp=false, the
// decode armed nothing, and stage 2 surfaced only in the seat panel's
// generic option list. The stage-2 remount is deferred by one macrotask to
// mimic the network round trip: synchronously it would happen INSIDE the
// click's bubble, and OptionPicker's window-level close-on-click would shut
// the freshly opened wheel again — an artifact of the synchronous fixture
// the real frame-driven flow never has.
const stageTwo: Decision = {
  seq: 10, player: 0, kind: 'choose', prompt: 'Choose a colour of mana', min: 1, max: 1,
  options: [
    { index: 0, kind: 'mana', label: 'Add B', obj: 16, player: 0 },
    { index: 1, kind: 'mana', label: 'Add R', obj: 16, player: 0 },
  ],
};

function remountStageTwo(autoOpen: boolean): void {
  document.querySelector('#followup')!.replaceChildren();
  mount(CardTile, {
    target: document.querySelector('#followup')!,
    props: {
      card: card(16, 'Talisman of Indulgence', 'Artifact'),
      tileOptions: {
        list: stageTwo.options,
        pickedOrder: [],
        tone: 'initiative',
        autoOpen,
        post: (index: number, expectFollowUp = false) => {
          posted.push([index, expectFollowUp, false]);
        },
      },
    },
  });
}

mount(CardTile, {
  target: document.querySelector('#followup')!,
  props: {
    card: card(16, 'Talisman of Indulgence', 'Artifact'),
    tileOptions: {
      list: [
        { index: 5, kind: 'mana', label: 'Add C', obj: 16, player: 0 },
        { index: 6, kind: 'mana', label: 'Add B or R', obj: 16, player: 0 },
      ],
      pickedOrder: [],
      tone: 'initiative',
      post: (index: number, expectFollowUp = false) => {
        posted.push([index, expectFollowUp, false]);
        const open = resolveCardFollowUp(expectFollowUp ? { seq: 9, obj: 16 } : null, stageTwo);
        setTimeout(() => remountStageTwo(open !== null), 0);
      },
    },
  },
});

// fb-20260923T020152Z: the declare-attackers wheel. Attacker options are
// SELECTIONS in a still-pending multi-pick decision (SeatPanelState.click
// toggles them into `picked` and posts nothing), so clicking one must post
// its own wire index and leave the wheel open for the next attacker. A third
// attacker is present so a test can also prove the wheel still SURVIVES a
// successful pick rather than merely being re-opened.
mount(CardTile, {
  target: document.querySelector('#attackers')!,
  props: {
    card: card(16, 'Grizzly Bears', 'Creature'),
    tileOptions: tile([
      { index: 40, kind: 'attacker', label: 'Attack with Grizzly Bears at Ari', obj: 16, player: 1 },
      { index: 41, kind: 'attacker', label: 'Attack with Grizzly Bears at Bo', obj: 16, player: 2 },
      { index: 42, kind: 'attacker', label: 'Attack with Grizzly Bears at Cy', obj: 16, player: 3 },
    ]),
  },
});
