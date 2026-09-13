import { mount } from 'svelte';
import type { CardView, Option } from '../protocol';
import type { TileOptions } from '../lib/cardoptions';
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
