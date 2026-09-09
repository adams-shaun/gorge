<script lang="ts">
  import type { CardView, PlayerView } from '../protocol';
  import { visibleHand } from '../lib/board';
  import { handFanLayout, type HandFanSpec } from '../lib/handfan';
  import type { CardOptions, TileOptions } from '../lib/cardoptions';
  import { tileOptions } from '../lib/cardoptions';
  import CardImage from './CardImage.svelte';
  import CardDetail from './CardDetail.svelte';
  import { HoverCard, type AnchorRect } from '../lib/carddetail.svelte';

  /**
   * HandFan is the SEATED PLAYER'S OWN hand, drawn as real card faces along
   * the bottom edge — the competitive-client layout (Arena, MTGO, XMage:
   * full-size faces in a centred row that overlaps rather than shrinks as the
   * hand grows). It is NOT the rail's HandList (see the note in
   * lib/cardoptions.ts' sibling, HandList.svelte), which stays text because it
   * has to fit four hands in a rail (survey #24); that reasoning is about the
   * rail and four hands, and has no purchase on the one hand you own.
   *
   * It renders ONLY for the seated player's own seat — `player` here is the
   * viewer's own PlayerView, whose hand is never redacted (view.go fills Hand
   * for the viewer's seat under every visibility). A spectator never mounts
   * it (Table.svelte gates on `seated`), but it still degrades honestly if it
   * is ever handed a null hand: a Go nil slice marshals to a literal JSON
   * `null` (view.go), and `handFan`/`visibleHand` both treat that as empty and
   * render nothing rather than throwing (the exact shape that once broke the
   * whole client — see the note in e2e/smoke.spec.ts).
   *
   * Hover/focus opens the SAME CardDetail panel every other card surface
   * opens, through the same HoverCard machinery — pointer dwell arms it,
   * keyboard focus opens it, leave/blur/Escape close it, and supervise keeps
   * a panel from hanging on a card that has since left the hand. The card
   * faces are CardImage (width from the `--card-w` custom property set on the
   * fan's container — never the `.card-image` width directly), so a hand card
   * gets exactly the same blank-vs-art treatment as a board tile.
   *
   * THE SAME OPTIONS AFFORDANCE AS A BOARD TILE (ui21), FROM THE SAME INDEX.
   * The board's tiles mark and menu the cards a pending decision offers
   * something to; a hand card was the blind spot — casting a card from hand
   * is the commonest decision a seat makes, and `tileOptions` (lib/cardoptions)
   * already returns exactly what a hand card needs: this card's options, its
   * picked ordinals, the tone, and the post callback. One mechanism, one
   * index, one post path (R-E4-1): the affordance here is a FAN-SPECIFIC
   * PRESENTATION of the same data, not a second index or a second post path.
   * Each hand card wears the tone ring marking, a badge carrying the count,
   * a menu of the server's own option labels that posts each option's own
   * index, and a picked chip in click order — exactly what a board tile gets,
   * adapted to the fan's geometry. See the task report for why the shape
   * differs (the fan overlaps; a menu must open UP, the hand sits at the
   * board's bottom edge; and the tile's all-four-corners signal hierarchy
   * has no purchase on a bare face, so the badge marks the card's top edge).
   *
   * The fan never covers the seat panel's decision UI (that panel floats at
   * the TOP of the board, this fan at the BOTTOM) and never intercepts a
   * click meant for anything else: the container is pointer-events:none and
   * only each card face accepts the pointer, so the empty gutter between
   * cards and the area behind them stay click-through to the board below.
   */

  let {
    player,
    /** the room the fan may occupy, px. Left at 0 the component measures its own container width; a unit test passes a concrete number. */
    width = 0,
    /** the pending decision's card-indexed offers (ui21's CardOptions bundle),
     *  or null for a spectator / when nothing is pending. A hand card the
     *  decision offers something to is marked and carries the same options
     *  menu as a board tile (R-E4-1: each item posts the option's own index). */
    options = null,
    /** open0 seeds the fan's open menu (by card id), injectable for the repo's
     *  SSR test harness just as CardTile's `open0` is: this environment has no
     *  DOM and no pointer events, so a test cannot click a badge to open the
     *  menu; the affordance's own state is what a test drives by hand the way
     *  it drives the detail panel. Production never passes it and the default
     *  is that no menu is open. */
    open0 = null,
  }: { player: PlayerView; width?: number; options?: CardOptions | null; open0?: number | null } = $props();

  const hand = $derived(visibleHand(player) ?? []);

  const CARD_W = 128;
  const GAP = 10;
  // The constraint is the room the BOARD has, so the fan never grows past the
  // viewport and the overlap tightens instead.
  //
  // This measures the full-width track element, NOT the fan row inside it.
  // Observing the fan itself measured this component's own output -- the row's
  // width is set from layout.rowWidth, so maxWidth settled at whatever width
  // the fan had already chosen and the constraint never bound. The overlap
  // path was reachable only from a unit test passing `width` explicitly; in a
  // real client a large hand simply ran off the board. The track is laid out
  // by the board and never by the fan, which is what makes it a real
  // measurement.
  let container = $state<HTMLElement | null>(null);
  let measured = $state(0);
  $effect(() => {
    if (container === null) return;
    const ro = new ResizeObserver((es) => {
      measured = es[0]?.contentRect.width ?? 0;
    });
    ro.observe(container);
    return () => ro.disconnect();
  });
  const room = $derived(width > 0 ? width : measured);
  const maxW = $derived(room > 0 ? Math.max(CARD_W, room) : Number.MAX_SAFE_INTEGER);
  const spec: HandFanSpec = $derived({ cardWidth: CARD_W, gap: GAP, maxWidth: maxW });
  const layout = $derived(handFanLayout(hand.length, spec));

  // One hover state for the whole fan, same contract as CardTile/HandList.
  const hover = new HoverCard();
  let hovered = $state<CardView | null>(null);
  let anchor = $state<AnchorRect | null>(null);

  // The options menu is this hand's own open/closed state: at most one card's
  // menu is open at a time (the same single-open contract a board tile has),
  // tracked here because a fan's cards are per-item instances and the fan
  // keeps per-card menu state in one place, keyed by card id. Nothing on the
  // wire drives it and nothing outside reads it.
  // svelte-ignore state_referenced_locally
  let openCard = $state<number | null>(open0);
  function toggleCard(id: number) {
    openCard = openCard === id ? null : id;
  }
  function openForCard(id: number) {
    return openCard === id;
  }

  // TIE THE PANEL TO THE OBJECT, NOT TO A POINTER EVENT: if the hovered card
  // leaves the hand (played, discarded, exiled) while the pointer is still
  // over it, its element is removed and pointerleave never fires — the panel
  // would hang on a card that no longer exists. Re-feed the present set on
  // every change; HoverCard closes when the one it shows is gone.
  $effect(() => {
    if (hovered !== null) hover.supervise(hovered.id, hand);
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

{#if hand.length > 0}
  <!-- The track spans the board and is the measured element; the fan row is
       centred inside it. Both are pointer-transparent; only a face claims the
       pointer. -->
  <div class="handtrack" bind:this={container}>
  <div class="handfan" style:--card-w="{CARD_W}px" style:width="{layout.rowWidth}px">
    {#each hand as c, i (c.id)}
      <!-- A hand card is NOT a board permanent: no tapped/attacking/counters
           chrome, just the face plus the shared hover inspector. When the
           pending decision offers this card something it also carries the
           same options affordance a board tile gets — the tone ring, the
           options badge + menu (each item posting its own index, R-E4-1),
           and the picked chip — adapted to the fan. -->
      {@const opt = options ? tileOptions(options, c.id) : null}
      <div class="card" class:marked={!!opt} data-obj={c.id} style:left="{i * layout.step}px">
        <div
          class="face"
          data-tone={opt?.tone ?? ''}
          data-options={opt ? opt.list.length : undefined}
          data-selected={opt && opt.pickedOrder.length > 0 ? opt.pickedOrder.join(',') : undefined}
          tabindex="0"
          role="button"
          onpointerenter={(e) => armFor(c, e.currentTarget)}
          onpointerleave={() => hover.close()}
          onfocus={(e) => openFor(c, e.currentTarget)}
          onblur={() => hover.close()}
          onkeydown={(e) => hover.keydown(e)}
          aria-describedby={hover.show && hovered === c ? `card-detail-${c.id}` : undefined}
        >
          <CardImage card={c} />
        </div>
        {#if opt}
          <!-- The options affordance sits OUTSIDE the role="button" face so a
               real button is never nested inside one; it anchors to the card's
               TOP EDGE (a bare face has no corner meaning to preserve, and the
               badge clears the card in front of it on the overlap fan). -->
          <div class="tile-actions">
            <button
              class="badge badge--{opt.tone}"
              class:selected={opt.pickedOrder.length > 0}
              type="button"
              aria-haspopup="menu"
              aria-expanded={openForCard(c.id)}
              aria-label="{opt.list.length} {opt.list.length === 1 ? 'action' : 'actions'} for {c.name}"
              title="Options for {c.name}"
              onclick={() => toggleCard(c.id)}
            >
              <span class="badge__n data">{opt.list.length}</span>
            </button>
            {#if opt.pickedOrder.length > 0}
              <span class="sel data" aria-label="picked {opt.pickedOrder.join(', ')}">{opt.pickedOrder.join(',')}</span>
            {/if}
            {#if openForCard(c.id)}
              <ul class="menu" role="menu" aria-label="Options for {c.name}">
                {#each opt.list as o (o.index)}
                  <li role="none">
                    <button class="menu__item" type="button" role="menuitem" onclick={() => opt.post(o.index)}>
                      {o.label}
                    </button>
                  </li>
                {/each}
              </ul>
            {/if}
          </div>
        {/if}
        {#if hover.show && hovered === c && anchor}
          <CardDetail card={hovered} anchor={anchor} />
        {/if}
      </div>
    {/each}
  </div>
  </div>
{/if}

<style>
  /* The fan floats along the bottom of the felt, centred, above the viewer's
     own board (their quadrant's lands row), the bottom edge of the play area.
     pointer-events:none on the row means only a card face (set back to auto
     below) ever claims the pointer — the empty gutter and the space behind
     the fan stay click-through, and the fan can never eat a click or hover
     meant for the seat panel's decision UI (which floats at the TOP of the
     board, the opposite edge). */
  .handtrack {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 6;
    pointer-events: none;
  }
  /* The fan owns the whole bottom edge of the board, full width. Anything
     else that wants to live down here (the seated player's identity bar) is
     lifted clear of it by --own-hand-h, published from the board rather than
     hard-coded twice. */
  .handfan {
    position: relative;
    margin-inline: auto;
    height: calc(var(--card-w) * 88 / 63);
    pointer-events: none;
  }
  /* Each face is absolutely positioned by the layout's step, then that step
     is also the negative margin so a face slides UNDER the one ahead of it
     when the fan overlaps: the first card is front-most (highest stacking),
     the last is behind it. Hovering raises the face. */
  .card {
    position: absolute;
    top: 0;
    width: var(--card-w);
    aspect-ratio: 63 / 88;
    pointer-events: auto;
    cursor: pointer;
    transition: transform 0.12s ease-out, filter 0.12s ease-out;
    transform: translateY(0);
    filter: brightness(0.92);
  }
  /* The hovered / focused face lifts off the row, the competitive raise, and
     the faces behind it go under rather than over it. */
  .card:hover,
  .card:focus-visible {
    transform: translateY(-0.9rem);
    filter: brightness(1);
    z-index: 10;
  }
  .card:focus-visible {
    outline: 2px solid var(--initiative);
    outline-offset: 1px;
  }
  /* The board marking, restated on a hand card: a card the pending decision
     offers something to wears the seat panel's own initiative/offered
     register (see lib/seatpanel toneOf) — blocked (targets/blocks/modes)
     warms to --initiative, an open window (cast/activate) cools to
     --offered. This is a ring around the face, the same grammar the board
     tiles use. */
  .face[data-tone='initiative'] {
    box-shadow: 0 0 0 2px var(--initiative);
    border-radius: var(--radius-card);
  }
  .face[data-tone='offered'] {
    box-shadow: 0 0 0 2px var(--offered);
    border-radius: var(--radius-card);
  }
  .face[data-tone=''] {
    box-shadow: none;
  }
  .face[data-selected] {
    box-shadow: 0 0 0 2px var(--ink), 0 0 0 4px var(--felt-sunk);
    border-radius: var(--radius-card);
  }
  .face[data-tone='initiative'][data-selected] {
    box-shadow: 0 0 0 2px var(--ink), 0 0 0 4px var(--initiative);
  }
  .face[data-tone='offered'][data-selected] {
    box-shadow: 0 0 0 2px var(--ink), 0 0 0 4px var(--offered);
  }

  /* The options affordance, anchored to the card's TOP edge. Unlike a board
     tile, a bare face has no corner that already means something (no keyword
     marks, no state band), and the hand must OPEN UPWARD — it sits at the
     board's bottom, so a menu that opened down would leave the felt. The
     badge clears the card in front of it (the overlap fan's front-most card
     is the lowest index; the cards ahead are behind it), so it is never
     covered. */
  .tile-actions {
    position: absolute;
    top: 1px;
    right: 1px;
    z-index: 20;
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 2px;
    line-height: 1;
  }
  .badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 1.1rem;
    height: 1.1rem;
    padding: 0 0.25rem;
    border-radius: 3px;
    border: 1px solid var(--edge-inst);
    background: var(--instrument);
    color: var(--ink);
    font-family: var(--font-data);
    font-size: var(--t-10);
    font-weight: 600;
    cursor: pointer;
  }
  .badge--initiative {
    background: var(--initiative);
    border-color: var(--initiative);
    color: var(--felt-sunk);
  }
  .badge--offered {
    background: var(--offered);
    border-color: var(--offered);
    color: var(--felt-sunk);
  }
  .badge.selected {
    outline: 2px solid var(--ink);
    outline-offset: 1px;
  }
  .sel {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 1rem;
    height: 1rem;
    padding: 0 0.2rem;
    border-radius: 2px;
    background: var(--ink);
    color: var(--felt-sunk);
    font-size: var(--t-10);
    font-weight: 600;
  }
  .badge__n {
    font-size: inherit;
  }
  .badge:hover,
  .badge[aria-expanded='true'] {
    border-color: var(--ink-dim);
    color: var(--ink);
  }
  /* The menu opens UP, above the fan, so it is never clipped by the board's
     bottom edge (the fan's own row is at the bottom of the felt). */
  .menu {
    position: absolute;
    bottom: calc(100% + 3px);
    right: 0;
    z-index: 20;
    margin: 0;
    padding: 2px;
    list-style: none;
    min-width: 9rem;
    max-width: 14rem;
    max-height: 12rem;
    overflow-y: auto;
    background: var(--instrument);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    box-shadow: var(--shadow-lift);
  }
  .menu__item {
    display: block;
    width: 100%;
    text-align: left;
    background: none;
    border: 0;
    border-left: 2px solid transparent;
    border-radius: 0;
    color: var(--ink-inst);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    line-height: 1.35;
    padding: var(--sp-1) var(--sp-2);
    cursor: pointer;
  }
  .menu__item:hover,
  .menu__item:focus-visible {
    background: color-mix(in srgb, var(--ink) 7%, var(--instrument));
    border-left-color: var(--ink-dim);
    color: var(--ink);
  }
</style>
