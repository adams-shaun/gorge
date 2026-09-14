<script lang="ts">
  import { tick } from 'svelte';
  import type { Decision, Option } from '../protocol';
  import { arrangeCard, arrangeDestination, arrangeFromOrder, arrangeOrder, arrangeSeed, arrangeSplit, moveWithin, type ArrangeSplit } from '../lib/arrange';
  import { oracle, type OracleCard } from '../lib/oracle';
  import CardImage from './CardImage.svelte';

  /**
   * ArrangeModal is the arrange popup (brief Job 4): the card-face surface
   * for a KArrange decision — the top-of-library reorder, Scry and Surveil
   * asks. It is portalled to body like PileModal (the rail cannot clip it)
   * and carries the modal guard markers, so the document hotkeys cannot act
   * through it.
   *
   * The two piles the engine's answer describes are shown as two rows of
   * real card faces (synthesized from the decision's own option labels —
   * the wire carries the decision's Obj/Label, not library CardViews):
   *
   *  - the KEEP row, top of the library in this order; drag a card onto
   *    another to reorder it (HTML5 drag and drop), or click it to drop it
   *    back to the pool row (click-to-place);
   *  - the POOL row, the cards heading to the destination ("the bottom of
   *    the library" / "the graveyard" / …) in offered order; click one to
   *    keep it (it joins the keep pile at the end).
   *
   * Submitting posts the keep row's option indices in display order through
   * the caller's onSubmit — the same picked-order intent the inline
   * click-order picker posts for the same final ordering (lib/arrange.ts's
   * arrangeOrder is that equivalence). The caller owns posting; this
   * component owns only the arrangement.
   *
   * A pure reorder (Min == Max == N) keeps every card: it opens with all
   * options in the keep row — the seat's picked prefix first, the rest in
   * offered order (lib/arrange.ts arrangeSeed) — and the pool row is hidden;
   * a card can only move, never leave (a keep-card's click does not drop on
   * a pure reorder, because the pool it would drop to does not exist).
   * Hovering a face shows it large beside the rows (the reporter's
   * "mouseover for full card art"), with the printed description beneath it
   * (fb-20260914T063020Z Job 2: art AND description — resolved by name
   * through lib/oracle, the way CardDetail resolves its oracle block; no
   * catalog renders no block).
   */
  let { open, decision, seed = [], onSubmit, onClose, resolver = oracle.text, preview0 = null }: {
    open: boolean;
    decision: Decision;
    /** seed is the picked order the arrangement starts from (the seat's current picked set, so reopening the modal keeps the work). */
    seed?: readonly number[];
    onSubmit: (order: number[]) => void;
    onClose: () => void;
    /** resolver resolves the previewed option's name to its printed facts; injectable for the SSR test harness (which has no catalog and no $effect), exactly as CardDetail's is. Production never passes it. */
    resolver?: (name: string) => OracleCard | null | Promise<OracleCard | null>;
    /** preview0 seeds the hovered preview for the SSR test harness (no pointer events), the way CardTile's open0 seeds its menu. Production never passes it. */
    preview0?: number | null;
  } = $props();

  let dialog = $state<HTMLElement | null>(null);

  // The arrangement is the keep ORDER, held as an edit state: null while the
  // player has not touched it (the piles then derive live from the seed —
  // which is also what SSR renders), and the edited order once any move has
  // happened. Working in the modal never posts until submit.
  let edits = $state<readonly number[] | null>(null);
  // Seeded from the prop (a test's injected preview) at mount/SSR time only
  // — in production only a pointer/focus event ever sets it, and the prop is
  // a seed, not a stream (the same shape CardTile's initialAnchor has).
  const initialPreview = () => preview0 ?? null;
  let preview = $state<number | null>(initialPreview());
  let dragging = $state<number | null>(null);

  /**
   * The decision sequence the edit state belongs to. The edit state (and the
   * whole arrangement) belongs to ONE ask: a decision with a different seq
   * arriving while this modal is mounted — the same seat's next arrange ask,
   * answered from another tab or resolved while this one sat open — must not
   * inherit it, or the old ask's ordering would be presented (and could be
   * submitted) as the new ask's answer. The caller closes the popup across a
   * decision change (SeatPanelState.adopt); this is the modal's own guard,
   * so a future mount site that forgets to close still cannot reuse edits.
   * (Effects do not run on the server; the SSR render has nothing to reset.)
   */
  let shownSeq = $state<number | null>(null);

  /** A pure reorder keeps every card — nothing can drop to a pool that does not exist. */
  const pureReorder = $derived(decision.min === decision.max && decision.max === decision.options.length);

  const startOrder = $derived(arrangeSeed(decision, seed));

  const split = $derived.by((): ArrangeSplit => {
    if (edits !== null) return arrangeFromOrder(decision, edits);
    return open ? arrangeSplit(decision, startOrder) : { keep: [], pool: [] };
  });

  /** previewedOption is the option the preview aside is showing, or null. */
  function previewedOption(): Option | null {
    if (preview === null) return null;
    return [...split.keep, ...split.pool].find((x) => x.index === preview) ?? null;
  }

  // The preview's printed description, resolved by name through the catalog
  // (fb-20260914T063020Z: the reporter asked for art AND description; the
  // aside had only art and the name). Best-effort like CardDetail's oracle:
  // the default resolver is the async catalog, which with no meta tag
  // resolves null without a request, and the block renders nothing until a
  // resolver returns text — no catalog is the NORMAL case, not an error.
  // The initialiser covers the synchronous-resolver path (server render
  // never runs the effect below, so a seeded preview's text must be in place
  // from the first render — the same shape CardDetail's initialOracle has).
  let previewOrc = $state<OracleCard | null>(initialPreviewOrc());

  function initialPreviewOrc(): OracleCard | null {
    const o = previewedOption();
    if (o === null) return null;
    const v = resolver(o.label);
    if (v !== null && typeof (v as Promise<OracleCard | null>).then !== 'function') return v as OracleCard;
    return null;
  }

  $effect(() => {
    const o = previewedOption();
    if (o === null) {
      previewOrc = null;
      return;
    }
    const v = resolver(o.label);
    if (v === null) {
      previewOrc = null;
      return;
    }
    if (typeof (v as Promise<OracleCard | null>).then === 'function') {
      let cancelled = false;
      (v as Promise<OracleCard | null>).then((c) => {
        if (!cancelled) previewOrc = c;
      });
      return () => {
        cancelled = true;
      };
    }
    previewOrc = v as OracleCard;
  });

  $effect(() => {
    const seq = decision.seq;
    if (shownSeq !== null && seq !== shownSeq) {
      edits = null;
      preview = null;
      dragging = null;
    }
    shownSeq = seq;
  });

  $effect(() => {
    if (open) {
      void tick().then(() => dialog?.focus());
    } else {
      // Closing drops the edits: reopening seeds from the caller again. The
      // derived reads `open`, so the closed render is already empty; this
      // only makes the NEXT open fresh.
      edits = null;
      preview = null;
      dragging = null;
    }
  });

  function portal(node: HTMLElement) {
    document.body.appendChild(node);
    return { destroy: () => node.remove() };
  }

  function backdropClick(event: MouseEvent): void {
    if (event.target === event.currentTarget) onClose();
  }

  function keydown(event: KeyboardEvent): void {
    if (open && event.key === 'Escape') {
      event.preventDefault();
      onClose();
    }
  }

  function keepCard(optIndex: number): void {
    if (!split.pool.some((x) => x.index === optIndex)) return;
    edits = [...arrangeOrder(split), optIndex];
    preview = null;
  }

  function dropToPool(optIndex: number): void {
    if (pureReorder) return; // nothing can leave a pure reorder's keep pile
    const order = arrangeOrder(split);
    const at = order.indexOf(optIndex);
    if (at < 0) return;
    order.splice(at, 1);
    edits = order;
    preview = null;
  }

  function reorder(optIndex: number, ontoIndex: number): void {
    const order = arrangeOrder(split);
    const from = order.indexOf(optIndex);
    const to = order.indexOf(ontoIndex);
    if (from < 0 || to < 0) return;
    edits = moveWithin(order, from, to);
  }

  function submit(): void {
    onSubmit(split.keep.map((o) => o.index));
  }
</script>

<svelte:window onkeydown={keydown} />

{#if open}
  <!-- The backdrop is pointer-dismissable while Escape is handled globally.
       data-arrange-modal names this surface for tests; role="dialog" +
       aria-modal="true" are the STRUCTURAL modal guard's markers
       (lib/hotkeys.ts MODAL_PICKER_SELECTOR), which is what actually keeps
       the document hotkeys from acting underneath — the data markers
       (data-pile-modal and this one) are the stable compatibility hooks. -->
  <div class="backdrop" data-arrange-backdrop data-arrange-modal data-pile-modal role="presentation" use:portal onclick={backdropClick}>
    <div
      class="dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="arrange-modal-title"
      tabindex="-1"
      bind:this={dialog}
    >
      <header>
        <h2 id="arrange-modal-title">{decision.prompt}</h2>
        <button type="button" class="close" aria-label="Close the arrange view" onclick={onClose}>×</button>
      </header>
      <div class="body" data-arrange-body>
        <div class="piles">
          <section class="pile keep">
            <h3>{pureReorder ? 'Top of the library, in this order' : 'Keep, in this order'}</h3>
            <ul class="cards" data-arrange-keep>
              {#each split.keep as o, i (o.obj)}
                {@const card = arrangeCard(decision, o)}
                <li
                  class="card"
                  class:dragging={dragging === o.index}
                  data-arrange-card={o.index}
                  data-arrange-keep-card={o.index}
                  draggable="true"
                  ondragstart={(e) => { dragging = o.index; e.dataTransfer?.setData('text/plain', String(o.index)); }}
                  ondragover={(e) => e.preventDefault()}
                  ondrop={(e) => { e.preventDefault(); if (dragging !== null) reorder(dragging, o.index); dragging = null; }}
                  ondragend={() => { dragging = null; }}
                >
                  <button
                    type="button"
                    class="face"
                    aria-label={`${i + 1}: ${o.label}`}
                    onclick={() => dropToPool(o.index)}
                    onpointerenter={() => { preview = o.index; }}
                    onpointerleave={() => { if (preview === o.index) preview = null; }}
                    onfocus={() => { preview = o.index; }}
                    onblur={() => { if (preview === o.index) preview = null; }}
                    onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); dropToPool(o.index); } }}
                  >
                    <CardImage {card} size="tile" pt={false} />
                    <span class="order">{i + 1}</span>
                    <span class="name">{o.label}</span>
                  </button>
                </li>
              {/each}
              {#if split.keep.length === 0}<li class="empty">Nothing kept — pick a card from below.</li>{/if}
            </ul>
          </section>
          {#if split.pool.length > 0 || decision.min < decision.max}
            <section class="pile pool">
              <h3>Rest go to {arrangeDestination(decision)}</h3>
              <ul class="cards" data-arrange-pool>
                {#each split.pool as o (o.obj)}
                  {@const card = arrangeCard(decision, o)}
                  <li
                    class="card dim"
                    data-arrange-card={o.index}
                    data-arrange-pool-card={o.index}
                  >
                    <button
                      type="button"
                      class="face"
                      aria-label={`Keep ${o.label}`}
                      onpointerenter={() => { preview = o.index; }}
                      onpointerleave={() => { if (preview === o.index) preview = null; }}
                      onfocus={() => { preview = o.index; }}
                      onblur={() => { if (preview === o.index) preview = null; }}
                      onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); keepCard(o.index); } }}
                      onclick={() => keepCard(o.index)}
                    >
                      <CardImage {card} size="tile" pt={false} />
                      <span class="name">{o.label}</span>
                    </button>
                  </li>
                {/each}
              </ul>
              <p class="hint">Click a card to keep it (it joins the top at the end); drag cards above to reorder.</p>
            </section>
          {/if}
        </div>
        <aside class="preview" data-arrange-preview>
          {#if preview !== null}
            {@const o = [...split.keep, ...split.pool].find((x) => x.index === preview)}
            {#if o}
              {@const card = arrangeCard(decision, o)}
              <CardImage {card} size="large" pt={false} />
              <p class="name">{o.label}</p>
            {/if}
          {:else}
            <p class="hint">Hover a card for full art.</p>
          {/if}
          {#if previewOrc?.oracle_text}<p class="oracle" data-arrange-preview-oracle>{previewOrc.oracle_text}</p>{/if}
        </aside>
      </div>
      <footer>
        <p class="hint">{split.keep.length} kept on top · {split.pool.length} to {arrangeDestination(decision)}</p>
        <button
          type="button"
          class="submit"
          data-arrange-submit
          onclick={submit}
          disabled={split.keep.length < decision.min || split.keep.length > decision.max}
        >Confirm</button>
      </footer>
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 1000;
    display: grid;
    place-items: center;
    padding: min(8vh, 4rem) var(--sp-4);
    background: rgb(5 8 12 / 72%);
  }
  .dialog {
    width: min(56rem, calc(100vw - 2rem));
    max-height: 84vh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    color: var(--ink-inst);
    background: var(--instrument);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    box-shadow: 0 1rem 3rem rgb(0 0 0 / 45%);
    outline: none;
  }
  .dialog:focus-visible {
    border-color: var(--initiative);
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--sp-3);
    padding: var(--sp-3) var(--sp-4);
    border-bottom: 1px solid var(--edge-inst);
  }
  h2 {
    margin: 0;
    font-size: var(--t-14);
    font-weight: 600;
    color: var(--ink);
  }
  .close {
    border: 0;
    padding: 0.1rem 0.35rem;
    background: none;
    color: var(--ink-dim);
    font: inherit;
    font-size: 1.35rem;
    line-height: 1;
    cursor: pointer;
  }
  .close:hover {
    color: var(--ink);
  }
  .body {
    display: flex;
    gap: var(--sp-4);
    padding: var(--sp-3) var(--sp-4);
    overflow-y: auto;
    min-height: 0;
  }
  .piles {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
  }
  h3 {
    margin: 0 0 var(--sp-1);
    font-size: var(--t-11);
    font-weight: 600;
    color: var(--ink-dim);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .cards {
    margin: 0;
    padding: var(--sp-2);
    list-style: none;
    display: flex;
    flex-wrap: wrap;
    gap: var(--sp-2);
    --card-w: var(--card-w-large, 132px);
  }
  .card {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.2rem;
    width: var(--card-w);
    cursor: grab;
    border-radius: var(--radius-card);
  }
  .card.dragging {
    opacity: 0.4;
  }
  .face {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.2rem;
    width: 100%;
    padding: 0;
    background: transparent;
    border: 0;
    border-radius: var(--radius-card);
    cursor: grab;
    line-height: 0;
  }
  .face:focus-visible {
    outline: 2px solid var(--initiative);
  }
  .face .name {
    line-height: 1.35;
  }
  .card.dim {
    opacity: 0.55;
  }
  .card .order {
    position: absolute;
    top: -0.4em;
    left: -0.4em;
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    font-size: 0.6875rem;
    line-height: 1.3;
    color: var(--felt-sunk);
    background: var(--initiative);
    border-radius: 2px;
    padding: 0.15em 0.3em;
  }
  .name {
    max-width: 100%;
    overflow: hidden;
    color: var(--ink-inst);
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--t-11);
    text-align: center;
  }
  .empty {
    color: var(--ink-faint);
    font-size: var(--t-12);
    padding: var(--sp-2);
  }
  .hint {
    margin: 0;
    font-size: var(--t-11);
    color: var(--ink-faint);
  }
  .preview {
    flex: none;
    width: var(--card-w-large, 220px);
    /* Size-invariant: the aside is as tall as its tallest content (the large
       card plus its name) whether or not a card is hovered. Without this the
       dialog is content-sized and centred, so a hover grows it, the rows
       shift under the stationary cursor, the hover target changes, the
       preview clears, the dialog shrinks back under the cursor — a hover
       feedback loop that made the whole modal breathe (observed as an
       endlessly oscillating layout in the browser gate). A constant box
       breaks the loop at the only place it can be broken: the resize.

       The printed description (data-arrange-preview-oracle) is part of that
       contract: its box is a FIXED height, scrolled when the text runs long,
       and its height is reserved in min-height below — so the aside is the
       same height in all four states (hovered/not × text/no-text) and a
       resolved description can never re-open the breathing loop. With no
       catalog the box renders nothing and the space stays reserved: the
       no-catalog degradation, not a hole. */
    --preview-oracle-h: 9rem;
    min-height: calc(var(--card-w-large, 220px) * 88 / 63 + 3.5rem + var(--preview-oracle-h) + var(--sp-2));
    justify-content: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--sp-2);
  }
  .preview .name {
    white-space: normal;
    text-align: center;
  }
  .preview .oracle {
    height: var(--preview-oracle-h);
    margin: 0;
    align-self: stretch;
    overflow-y: auto;
    font-size: var(--t-11);
    line-height: 1.45;
    color: var(--ink-dim);
  }
  footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--sp-3);
    padding: var(--sp-3) var(--sp-4);
    border-top: 1px solid var(--edge-inst);
  }
  .submit {
    background: var(--initiative);
    color: var(--felt-sunk);
    border: 0;
    border-radius: var(--radius);
    padding: var(--sp-2) var(--sp-4);
    font-family: var(--font-ui);
    font-size: var(--t-14);
    font-weight: 600;
    cursor: pointer;
  }
  .submit:disabled {
    opacity: 0.4;
    cursor: default;
  }
</style>
