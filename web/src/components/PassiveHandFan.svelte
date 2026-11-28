<script lang="ts">
  import type { CardView } from '../protocol';
  import { handFanLayout } from '../lib/handfan';
  import { PASSIVE_GAP, PASSIVE_PAD, passiveFaceW } from '../lib/passivehand';
  import CardImage from './CardImage.svelte';
  import CardDetail from './CardDetail.svelte';
  import { HoverCard, type AnchorRect } from '../lib/carddetail.svelte';

  /**
   * PassiveHandFan is a SPECTATOR's view of one player's hand: real card faces
   * drawn in that player's own seat panel, the way a person sitting at the
   * table with every hand face up would see it (inbox-omniscient-rendered-
   * hand-fans). It replaced the rail's text hand list, which only ever showed
   * names.
   *
   * It is deliberately NOT HandFan. HandFan is the seated player's own,
   * interactive hand: the decision's card options, the CAST/PLAY shortcuts and
   * the options menu all live there, wired to the seat's post path. This
   * component takes no options, no payment actions and no callbacks of any
   * kind, so a spectator's fan cannot post an intent — there is nothing in it
   * to wire one to. What it shares with HandFan is the presentation: the same
   * CardImage faces, the same overlap-instead-of-shrink layout maths
   * (lib/handfan), and the same CardDetail inspector on hover/focus through
   * the same HoverCard contract (pointer dwell arms it, focus opens it,
   * leave/blur/Escape close it, supervise closes it when the card leaves).
   *
   * Board decides WHO gets one: only a spectator, and only for a seat whose
   * hand the server actually sent (visibleHand non-null). A public spectator's
   * hands arrive redacted (null), so nothing mounts and no identity, image or
   * object id reaches the DOM. Board also sizes it — the band is taken away
   * from the seat's battlefield before the rows are sized, so the fan never
   * covers a permanent. The faces never rise out of the band (that would
   * leave the seat panel, which clips); a hovered or focused face only comes
   * to the front of its neighbours.
   */
  let {
    seat,
    name,
    cards,
    faceH,
    width,
  }: {
    /** the seat whose hand this is */
    seat: number;
    /** the seat's display name, for the fan's accessible label */
    name: string;
    /** the exposed hand: visibleHand(player), already known to be non-null */
    cards: CardView[];
    /** one face's height, px (lib/passivehand) */
    faceH: number;
    /** the room the band has, px; the fan overlaps rather than exceed it */
    width: number;
  } = $props();

  const faceW = $derived(passiveFaceW(faceH));
  const layout = $derived(handFanLayout(cards.length, { cardWidth: faceW, gap: PASSIVE_GAP, maxWidth: Math.max(faceW, width - PASSIVE_PAD * 2) }));

  const hover = new HoverCard();
  let hovered = $state<CardView | null>(null);
  let anchor = $state<AnchorRect | null>(null);

  // A card leaving the hand while the pointer is over it never fires
  // pointerleave; re-feed the present set so the inspector closes with it.
  $effect(() => {
    if (hovered !== null) hover.supervise(hovered.id, cards);
  });

  function rectOf(el: HTMLElement): AnchorRect {
    const r = el.getBoundingClientRect();
    return { left: r.left, top: r.top, right: r.right };
  }
  function armFor(c: CardView, el: HTMLElement) {
    hovered = c;
    hover.arm(c.id, () => {
      anchor = rectOf(el);
    });
  }
  function openFor(c: CardView, el: HTMLElement) {
    hovered = c;
    hover.open(c.id, () => {
      anchor = rectOf(el);
    });
  }
</script>

<div
  class="passive-hand"
  role="group"
  aria-label="{name}'s hand, {cards.length} {cards.length === 1 ? 'card' : 'cards'}"
  data-passive-hand={seat}
  data-motion-anchor={`${seat}:hand`}
  style:--card-w="{faceW}px"
  style:--pad="{PASSIVE_PAD}px"
>
  {#if cards.length === 0}
    <span class="empty">No cards in hand</span>
  {:else}
    <div class="fan" class:overlap={layout.overlap > 0} style:width="{layout.rowWidth}px">
      {#each cards as c, i (c.id)}
        <div class="card" data-obj={c.id} data-hand-card={c.id} style:left="{i * layout.step}px">
          <!-- A read-only inspection target: focus and hover open the shared
               CardDetail panel; there is no click handler and nothing to post. -->
          <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
          <div
            class="face"
            role="img"
            aria-label={c.name}
            tabindex="0"
            onpointerenter={(e) => armFor(c, e.currentTarget)}
            onpointerleave={() => hover.close()}
            onfocus={(e) => openFor(c, e.currentTarget)}
            onblur={() => hover.close()}
            onkeydown={(e) => hover.keydown(e)}
            aria-describedby={hover.show && hovered?.id === c.id ? `card-detail-${c.id}` : undefined}
          >
            <CardImage card={c} />
          </div>
          {#if hover.show && hovered?.id === c.id && anchor}
            <CardDetail card={c} {anchor} />
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  /* The band fills the slot Board reserved for it and centres the fan in it.
     It clips nothing of its own: the slot is sized to one face plus padding,
     and a wide hand overlaps (lib/handfan) instead of spilling sideways. */
  .passive-hand {
    position: relative;
    box-sizing: border-box;
    width: 100%;
    height: 100%;
    min-width: 0;
    padding: var(--pad);
    display: flex;
    align-items: center;
    justify-content: center;
    background: color-mix(in srgb, var(--felt-sunk) 45%, transparent);
    border-radius: 6px;
  }
  .fan {
    position: relative;
    height: calc(var(--card-w) * 88 / 63);
    flex: none;
  }
  /* A face is a CARD, so its box is the card's shape by arithmetic: 63/88, the
     same expression the fan's height uses, pinned on both axes rather than
     left to the box's content, and the ratio stays declared so the shape
     survives a caller that sets only one axis. A content-sized box grew by
     the font's descender under the inline card image: 2px with one fallback
     font, 3px on a GitHub runner's, 10% of a focus side strip's 19x27px face
     (SpectatorHands.geometry.test.ts's focus8 case). */
  .card {
    position: absolute;
    top: 0;
    width: var(--card-w);
    height: calc(var(--card-w) * 88 / 63);
    aspect-ratio: 63 / 88;
  }
  /* A hovered or focused face comes to the FRONT of the neighbours that
     overlap it; it does not rise (the seat panel clips anything that leaves
     the band). */
  .card:hover,
  .card:focus-within {
    z-index: 5;
  }
  .face {
    /* The wrapper's line box must not add descender space below the face:
       aspect-ratio fixes the card's shape, so a fallback font's descent made
       the box taller than its CardImage (and failed the shape assertion on
       CI's DejaVu Sans). Same rule as CardTile's .face. */
    line-height: 0;
    filter: brightness(0.94);
    cursor: default;
  }
  .card:hover .face,
  .card:focus-within .face {
    filter: brightness(1);
  }
  .face:focus-visible {
    outline: 2px solid var(--initiative);
    outline-offset: 1px;
  }
  .empty {
    color: var(--ink-faint);
    font-size: var(--t-11);
  }
</style>
