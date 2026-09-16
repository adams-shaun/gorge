<script lang="ts">
  import type { PlayerView, StackView } from '../protocol';
  import type { CardOptions } from '../lib/cardoptions';
  import { attachedTo, groupBattlefield, stackIdentical } from '../lib/board';
  import type { SeatCorner } from '../lib/seattable';
  import { layoutStore } from '../lib/layoutsettings.svelte';
  import CardStack from './CardStack.svelte';
  import CommandArea from './CommandArea.svelte';
  import ZoneStepper from './ZoneStepper.svelte';

  /** Quadrant shows one player's battlefield, split into the three rows board.ts groups it into. It has no rules knowledge: grouping and ordering come entirely from groupBattlefield; stackIdentical then collapses interchangeable permanents within a row into one tile with a count (CardStack renders the group). Attachments still come from attachedTo for a group of one — a stacked group has none by the stacking rule. The seat's command zone (CommandArea) draws directly into the creatures row, at creature scale, alongside the CardStacks — not into a private area of its own (CZ2); it draws nothing at all for a seat with no commander roster. `stack` is passed through to it alone: a commander mid-cast is a spell on the stack, not in any zone list. `options` (the pending decision's card-indexed offers, or null) is forwarded to every CardStack, and from there to each tile, and to CommandArea, which looks a commander's own options up by its object id the same way.
   *
   * The keyed-each key is the group's RENDER key (g.render — the visible lead object's id), not the mutable stacking-equivalence string (g.render): `key` changes whenever any visible state changes — that is the grouping working — but a DOM key that changes unmounts CardStack and its CardTile, destroying the tile's local hover state, so an inspector the reader had open closed the moment the same permanent untapped or stopped attacking (fb-20260915T182335Z). Keyed by the lead object's id, a kept tile handed a newer CardView for the same object keeps its open inspector and renders the fresh state (CardTile's superviseRendering guard still closes it when the id itself changes or the object stops being rendered). */
  let { player, colour, corner = 'bl', stack = [], options = null, own = false }: { player: PlayerView; colour: string; corner?: SeatCorner; stack?: StackView[]; options?: CardOptions | null; /** own is true only for the VIEWER'S quadrant (Board computes it from view.viewer): the on-board resize stepper (ZoneStepper) is mounted on this quadrant's rows alone — four copies of one control mutating one shared setting would be noise. The scale/align/flash themselves apply to every quadrant, because the setting is one player preference, not four. */ own?: boolean } = $props();

  // Layout settings (fb-20260916T182801Z): one shared store, read reactively —
  // a change from the Game Options panel re-renders the rows here without
  // event plumbing. rowHover holds the stepper-presence flag per row so the
  // dotted outline stays up while the control is being used (the store's
  // flash covers the post-adjustment window; see layoutsettings.svelte.ts).
  let rowHover = $state<{ creatures: boolean; others: boolean; lands: boolean }>({
    creatures: false,
    others: false,
    lands: false,
  });
  type BattlefieldZone = 'creatures' | 'others' | 'lands';
  function rowOutlined(zone: BattlefieldZone): boolean {
    return layoutStore.flash[zone] || rowHover[zone];
  }
  function hoverRow(zone: BattlefieldZone, hovering: boolean): void {
    rowHover[zone] = hovering;
  }

  const battlefieldGroups = $derived(groupBattlefield(player.battlefield));
  const stacks = $derived({
    lands: stackIdentical(battlefieldGroups.lands),
    creatures: stackIdentical(battlefieldGroups.creatures),
    others: stackIdentical(battlefieldGroups.others),
  });

  // The seat rule goes on the seat's OUTER edge — the table's rim — so four
  // rules frame the table instead of four lines cutting across the middle of
  // it. quadrantFor already decided which corner this seat sits in.
  const OUTER: Record<string, string> = { tl: 'top', tr: 'top', bl: 'bottom', br: 'bottom', l: 'left', r: 'right', top: 'top', bottom: 'bottom' };
  // …and the same fact orients the board. Every seat's permanents are pushed
  // toward the middle of the table, lands at that seat's own rim and
  // creatures nearest the centre, the way four people actually sit round a
  // table. That is what puts the two creature rows of a combat next to each
  // other across the seam, and it leaves each seat's outer corner clear for
  // its identity bar instead of stranding a permanent underneath it.
  // A 1v1 (top/bottom, the viewer at the bottom) faces the bottom seat up and
  // the top seat down, exactly like the top/bottom corners of a 4-seat table.
  // The legacy side-by-side `l`/`r` corners (no longer produced by the
  // mapping) still map to facing-side, so the rendering maps stay total.
  const FACING: Record<string, string> = { tl: 'facing-down', tr: 'facing-down', bl: 'facing-up', br: 'facing-up', l: 'facing-side', r: 'facing-side', top: 'facing-down', bottom: 'facing-up' };
</script>

<div class="quadrant rule-{OUTER[corner]} {FACING[corner]}" class:lost={player.lost} style:--seat={colour} data-seat={player.seat} data-lost={player.lost}>
  <!-- Nonlands above, lands below (survey #22), so a board stays parseable as
       it grows and the row a combat is read from is always in the same place.
       The seat's commanders draw first in the creatures row (CZ2): a
       commander is a creature, so it takes the row's own --card-w rather
       than a scale of its own, and it sits beside the CardStacks it competes
       with in combat instead of in a private area elsewhere on the seat's
       rim. Nothing is drawn here for a seat with no commander roster. -->
  <div
    class="row creatures"
    class:zone-outline={rowOutlined('creatures')}
    style:--row-scale={layoutStore.scale('creatures')}
    data-align={layoutStore.align('creatures')}
    data-zone-row="creatures"
  >
    <CommandArea {player} {stack} {options} />
    {#each stacks.creatures as g (g.render)}
      <CardStack group={g} attachments={g.cards.length === 1 ? attachedTo(player.battlefield, g.cards[0].id) : []} {options} />
    {/each}
    {#if own}
      <ZoneStepper zone="creatures" label="creature" onhover={(h) => hoverRow('creatures', h)} />
    {/if}
  </div>
  <div
    class="row others"
    class:zone-outline={rowOutlined('others')}
    style:--row-scale={layoutStore.scale('others')}
    data-align={layoutStore.align('others')}
    data-zone-row="others"
  >
    {#each stacks.others as g (g.render)}
      <CardStack group={g} attachments={g.cards.length === 1 ? attachedTo(player.battlefield, g.cards[0].id) : []} {options} />
    {/each}
    {#if own}
      <ZoneStepper zone="others" label="non-creature" onhover={(h) => hoverRow('others', h)} />
    {/if}
  </div>
  <div
    class="row lands"
    class:zone-outline={rowOutlined('lands')}
    style:--row-scale={layoutStore.scale('lands')}
    data-align={layoutStore.align('lands')}
    data-zone-row="lands"
  >
    {#each stacks.lands as g (g.render)}
      <CardStack group={g} attachments={g.cards.length === 1 ? attachedTo(player.battlefield, g.cards[0].id) : []} {options} />
    {/each}
    {#if own}
      <ZoneStepper zone="lands" label="land" onhover={(h) => hoverRow('lands', h)} />
    {/if}
  </div>
</div>

<style>
  /*
   * Felt: the seat is identified by a rule on its outer edge, not by a
   * saturated wash over the whole quadrant. Four translucent colour fields
   * behind card art muddy every card on the table; a rule costs nothing and
   * says the same thing. Same vocabulary as the identity bar and life grid.
   */
  .quadrant {
    position: relative;
    box-sizing: border-box;
    width: 100%;
    height: 100%;
    background: var(--felt-raised);
    padding: var(--sp-3);
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
    overflow: auto;
    overscroll-behavior: contain;
  }
  /* An eliminated seat's whole quadrant reads as greyed out (survey report:
     "there are 2 dead players, but their health doesn't reflect 0" — the fix
     is NOT forcing life to 0, which would be a lie for a commander-damage or
     empty-library loss; it is making the board itself say "this seat is
     done"). The scrim is a `::after` overlay rather than `filter`/`opacity`
     on `.quadrant` itself: `filter` (like `transform`) would make `.quadrant`
     a containing block for any `position: fixed` descendant, and CardDetail
     — the hover inspector every commander tile and card tile opens — is
     exactly that, nested three components deep inside these rows. Greying
     the ANCESTOR would silently drag the inspector's popup off the viewport
     and grey it too, breaking "a spectator still wants to see what they
     had" the moment they hover a dead seat's card. A `::after` with
     `backdrop-filter` dims and desaturates only what is painted BELOW it —
     the felt and the cards — while an inspector popup, painted above it at
     CardDetail's own z-index 9, is completely unaffected: full colour, and
     positioned by the viewport exactly as it always was. `pointer-events:
     none` keeps every card underneath fully hoverable and focusable through
     the scrim, so nothing here makes a dead seat's board uninspectable. */
  .quadrant.lost::after {
    content: '';
    position: absolute;
    inset: 0;
    z-index: 2;
    pointer-events: none;
    background: color-mix(in srgb, var(--felt-sunk) 62%, transparent);
    backdrop-filter: grayscale(0.9) brightness(0.62);
  }
  /* Seats along the top of the table read from their own rim inward, so the
     column runs the other way and packs against the seam. */
  .facing-down {
    flex-direction: column-reverse;
    justify-content: flex-start;
    padding-top: var(--sp-8);
  }
  .facing-up {
    flex-direction: column;
    justify-content: flex-start;
    padding-bottom: var(--sp-8);
  }
  .facing-side {
    flex-direction: column;
    justify-content: flex-end;
    padding-top: var(--sp-8);
  }

  .rule-top { border-top: 2px solid var(--seat); }
  .rule-bottom { border-bottom: 2px solid var(--seat); }
  .rule-left { border-left: 2px solid var(--seat); }
  .rule-right { border-right: 2px solid var(--seat); }

  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--sp-3);
    align-items: flex-end;
    align-content: flex-end;
    min-height: var(--sp-2);
  }
  /* Every battlefield row uses the gameplay-card scale shared with the hand.
     Size remains a row property rather than a CardTile `large` mode, so all
     of the tile chrome continues to follow the same --card-w geometry.

     No row stretches. A creature row with flex:1 pushed the lands to the far
     edge and opened a hole in the middle of every seat that had two
     permanents; the rows simply sit together against the seam instead.

     fb-20260916T182801Z: --row-scale is the layout-settings multiplier
     (lib/layoutsettings.svelte.ts); data-align packs the row's main axis
     left/centre/right; position:relative anchors the on-board resize
     stepper; .zone-outline is the dotted outline the store pulses for
     FLASH_MS after an adjustment (and the row holds it while its stepper is
     hovered/focused — rowHover). */
  .row {
    --card-w: calc(var(--play-card-w) * var(--row-scale, 1));
    position: relative;
  }
  .row[data-align='center'] { justify-content: center; }
  .row[data-align='right'] { justify-content: flex-end; }
  .row.zone-outline {
    outline: 2px dashed var(--ink-dim);
    outline-offset: 3px;
  }
  .row.creatures {
    align-items: flex-start;
    align-content: flex-start;
  }
</style>
