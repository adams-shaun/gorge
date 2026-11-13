<script lang="ts">
  import type { DraftCard } from '../../protocol';
  import { factsOf, mvColumns, tint } from '../../lib/draftcards';
  import ManaSymbols from '../ManaSymbols.svelte';
  import CardDetail from '../CardDetail.svelte';
  import { cardView } from '../../lib/draftcards';
  import { DraftHover } from '../../lib/drafthover.svelte';

  /**
   * A pool or deck laid out in mana-value columns (0–1 … 6+, Land, and "?"
   * for a card the server sent no facts for), grouped by the facts each
   * DraftCard carries on the wire. Each card is a one-line strip.
   * With `onactivate` the strips are buttons — click, tap or Enter moves the
   * card — and with `draggable` they also start an HTML5 drag carrying the
   * card's instance id; the drop target is the zone around this grid.
   * `muted` greys out a card the caller wants read as unavailable (a
   * Commander draft's card outside the commanders' colour identity) and
   * names why in its title; activating it still calls `onactivate`, whose
   * owner refuses and says so.
   */
  let {
    cards,
    draggable = false,
    onactivate,
    label,
    muted = null,
    mutedWhy = '',
  }: {
    cards: DraftCard[];
    draggable?: boolean;
    onactivate?: (id: string) => void;
    label?: string;
    muted?: ((c: DraftCard) => boolean) | null;
    mutedWhy?: string;
  } = $props();

  const cols = $derived(mvColumns(cards, factsOf));
  let dragging = $state<string | null>(null);

  // Hovering a strip opens the zoomed CardDetail panel (the one-line strip
  // only names the card). A strip that moves to the other zone leaves this
  // grid under the pointer, so supervise closes a panel whose card is gone.
  const detail = new DraftHover();
  $effect(() => detail.supervise(cards));

  function dragstart(e: DragEvent, id: string) {
    if (!e.dataTransfer) return;
    e.dataTransfer.setData('text/plain', id);
    e.dataTransfer.effectAllowed = 'move';
    dragging = id;
  }
</script>

<div class="mvcols" aria-label={label}>
  {#each cols as col (col.key)}
    <div class="mvcol">
      <div class="h"><span>{col.label}</span><span>{col.cards.length || ''}</span></div>
      {#each col.cards as c (c.id)}
        {@const f = factsOf(c)}
        {#if onactivate}
          <button
            type="button"
            class="strip ev-tint-{tint(f)}"
            class:dragging={dragging === c.id}
            class:muted={muted?.(c) ?? false}
            title={muted?.(c) ? mutedWhy || undefined : undefined}
            data-id={c.id}
            data-muted={muted?.(c) ? '' : undefined}
            {draggable}
            ondragstart={(e) => dragstart(e, c.id)}
            ondragend={() => (dragging = null)}
            onclick={() => onactivate(c.id)}
            onpointerenter={(e) => detail.enter(c, e.currentTarget)}
            onpointerleave={() => detail.leave()}
            onfocus={(e) => detail.focus(c, e.currentTarget)}
            onblur={() => detail.leave()}
            onkeydown={(e) => detail.keydown(e)}
          >
            <span class="nm">{c.name}</span>
            {#if c.promo}<span class="promo-tag" title="Prerelease promo">promo</span>{/if}
            {#if f.cost}<ManaSymbols cost={f.cost} />{/if}
          </button>
        {:else}
          <div
            class="strip ev-tint-{tint(f)}"
            class:muted={muted?.(c) ?? false}
            data-id={c.id}
            role="listitem"
            onpointerenter={(e) => detail.enter(c, e.currentTarget)}
            onpointerleave={() => detail.leave()}
          >
            <span class="nm">{c.name}</span>
            {#if c.promo}<span class="promo-tag" title="Prerelease promo">promo</span>{/if}
            {#if f.cost}<ManaSymbols cost={f.cost} />{/if}
          </div>
        {/if}
      {/each}
    </div>
  {/each}
</div>

{#if detail.show && detail.card && detail.anchor}
  <CardDetail card={cardView(detail.card)} anchor={detail.anchor} />
{/if}

<style>
  /* Seven columns on a wide screen; they wrap rather than shrink to nothing
     on a phone, so the page never scrolls sideways. */
  .mvcols {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(6.5rem, 1fr));
    gap: 0.4rem;
  }
  .mvcol {
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
  }
  .h {
    font: 500 var(--t-10) var(--font-data);
    color: var(--ink-faint);
    letter-spacing: 0.06em;
    padding-bottom: 3px;
    border-bottom: 1px solid var(--edge-felt);
    display: flex;
    justify-content: space-between;
  }
  .strip {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 4px;
    width: 100%;
    padding: 0.3rem 0.4rem;
    border: 0;
    border-left: 3px solid var(--cc, var(--mana-c));
    border-radius: 3px;
    background: color-mix(in srgb, var(--cc, var(--mana-c)) 20%, var(--felt-sunk));
    font: 500 var(--t-11) / 1.2 var(--font-ui);
    color: var(--ink);
    text-align: left;
    min-width: 0;
    /* a comfortable tap target on a phone */
    min-height: 1.75rem;
  }
  button.strip {
    cursor: pointer;
  }
  button.strip[draggable='true'] {
    cursor: grab;
  }
  button.strip:hover {
    background: color-mix(in srgb, var(--cc, var(--mana-c)) 32%, var(--felt-sunk));
  }
  .strip.dragging {
    opacity: 0.4;
  }
  .strip.muted {
    opacity: 0.38;
    filter: grayscale(0.8);
  }
  .nm {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
