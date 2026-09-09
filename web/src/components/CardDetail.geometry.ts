import { mount } from 'svelte';
import type { CardView } from '../protocol';
import '../app.css';
import CardDetail from './CardDetail.svelte';

/**
 * Geometry fixture for CardDetail — the browser half of the non-occlusion
 * leaf. It mounts the real component (not a CSS replica) so the measured rects
 * are what a real hover produces.
 *
 * The plate shows only when CardImage resolves an image. The resolver
 * (../lib/images) reads an exact-name localStorage key, so the fixture seeds
 * those keys before mounting: a data-URL GIF for the oracle-present card (the
 * `.card-image` box sizes itself from aspect-ratio 63/88, so the img's
 * intrinsic size never affects layout — the box is what geometry is measured
 * on), and an empty string for the plain card, which the resolver stores as a
 * known "no image" so no Scryfall request is ever fired.
 *
 * `anchor` is a hand-card-like position computed from the live viewport: low
 * enough that the roomier side is above the card, so placePanel opens UPWARD
 * and caps the panel at the room above. That cap is exactly what used to
 * overrun — the flex column took the shortfall out of the plate and cropped
 * the card's printed oracle block rather than letting the panel scroll.
 */

const BLANK_GIF = 'data:image/gif;base64,R0lGODlhAQABAIAAAAUEBAAAACwAAAAAAQABAAACAkQBADs=';

localStorage.setItem('gorge.img.Giada, Font of Hope', BLANK_GIF);
// A known-no-image entry: the resolver stores null as '' and reads it back as
// null, so this card resolves no image and the plate stays absent — the normal
// case, not an error.
localStorage.setItem('gorge.img.Squire', '');

const giada: CardView = {
  id: 1,
  name: 'Giada, Font of Hope',
  types: 'Legendary Creature — Angel',
  mana_cost: '1 W',
  printing: { name: 'Giada, Font of Hope' },
  token: '',
  tapped: false,
  power: 3,
  toughness: 3,
  damage: 0,
  attacking: false,
  counters: { '+1/+1': 2 },
  keywords: ['flying', 'vigilance'],
  controller: 0,
  owner: 0,
  summon_sick: false,
};

const squire: CardView = {
  ...giada,
  id: 2,
  name: 'Squire',
  types: 'Creature — Human Soldier',
  mana_cost: '1 W',
  printing: { name: 'Squire' },
  keywords: [],
  counters: {},
  power: 3,
  toughness: 2,
};

// A hand-card-like anchor: near the bottom of the board, so the roomier side
// is above it and placePanel must open UPWARD (ui20). The 190px offset keeps
// the panel tall enough that the 63:88 plate fits inside the room above at
// every viewport the client supports, while the plate+ledger content is still
// taller than that room — the exact shape that used to crop the plate.
const anchor = {
  left: 200,
  top: Math.max(360, Math.round(window.innerHeight - 190)),
  right: 300,
};

mount(CardDetail, {
  target: document.querySelector('#cd-oracle')!,
  props: {
    card: giada,
    anchor,
    resolver: () => ({
      name: 'Giada, Font of Hope',
      mana_cost: '1 W',
      type_line: 'Legendary Creature — Angel',
      oracle_text:
        'Flying, vigilance\nEach other Angel you control enters the battlefield with an additional +1/+1 counter on it.',
      power: '2',
      toughness: '2',
    }),
  },
});

mount(CardDetail, {
  target: document.querySelector('#cd-plain')!,
  props: {
    card: squire,
    anchor,
    resolver: () => null,
  },
});
