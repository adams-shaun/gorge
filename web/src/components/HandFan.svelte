<script lang="ts">
  import type { CardView, PlayerView } from '../protocol';
  import { visibleHand } from '../lib/board';
  import { handFanLayout, type HandFanSpec } from '../lib/handfan';
  import CardImage from './CardImage.svelte';
  import CardDetail from './CardDetail.svelte';
  import { HoverCard, type AnchorRect } from '../lib/carddetail.svelte';

  /**
   * HandFan is the SEATED PLAYER'S OWN hand, drawn as real card faces along
   * the bottom edge — the competitive-client layout (Arena, MTGO, XMage:
   * full-size faces in a centred row that overlaps rather than shrinks as the
   * hand grows). It is NOT the rail's HandList, which stays text because it
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
  }: { player: PlayerView; width?: number } = $props();

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
           chrome, just the face plus the shared hover inspector. -->
      <div
        class="card"
        style:left="{i * layout.step}px"
        data-obj={c.id}
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
    {/each}
    {#if hover.show && hovered && anchor}
      <CardDetail card={hovered} anchor={anchor} />
    {/if}
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
</style>
