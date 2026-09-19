<script lang="ts">
  import { tick } from 'svelte';
  import type { Decision } from '../protocol';
  import { discardCard } from '../lib/discard';
  import { oracle, type OracleCard } from '../lib/oracle';
  import CardImage from './CardImage.svelte';

  /**
   * DiscardModal is the discard-pick ask's "Open the card view" popup
   * (fb-20260918T201739Z-2a7cdc26): the big-card-face surface for a decision
   * whose options are card picks — the Thoughtseize/Duress reveal-pick, the
   * Mind Rot discard, the cleanup-step discard, the cast-cost discard. The
   * player's ask was exactly the affordance the arrange ask already has: a
   * modal like the graveyard/exile pile viewer, since the inline strip's
   * small faces are all the pick ever got.
   *
   * It is presentational only — the posting path is the caller's OWN decision
   * logic, so the wire intent is byte-identical to the inline strip's:
   *
   *  - every face's click calls `onPick(option.index)`, which SeatPanel wires
   *    to `logic.click` — a Min==Max==1 ask (Thoughtseize) posts straight
   *    through on the click (which also closes the ask, and with it this
   *    modal), a range ask toggles into the shared `picked` array;
   *  - the Confirm button calls `onSubmit`, wired to the ordinary
   *    `logic.submit()`, gated on the same `canSubmit` (the decision's own
   *    Min/Max) the inline row's submit uses;
   *  - `showSubmit` mirrors the inline row's gate (logic.showSubmit): a
   *    Min==Max==1 ask renders no submit at all, because the click IS the
   *    answer.
   *
   * Portalled to body like ArrangeModal/PileModal (the rail's capped regions
   * would clip it) and carrying the modal guard markers — `role="dialog"` +
   * `aria-modal` are the structural markers lib/hotkeys.ts'
   * MODAL_PICKER_SELECTOR recognises, so the document hotkeys cannot act
   * through this surface without a hotkey-code change; `data-discard-modal`
   * and `data-pile-modal` are the stable compatibility hooks.
   *
   * Hovering a face shows it large in the aside with the printed description
   * beneath (resolved by name through lib/oracle, the way ArrangeModal and
   * CardDetail do) — the fb-20260914T120705Z "mouseover oracle help popup
   * like every other view" requirement.
   */
  let {
    open,
    decision,
    picked = [],
    showSubmit = true,
    canSubmit = false,
    busy = false,
    onPick,
    onSubmit,
    onClose,
    resolver = oracle.text,
    preview0 = null,
  }: {
    open: boolean;
    decision: Decision;
    /** picked is the caller's picked array (the shared SeatPanelState one) — the order badges read it, so modal and strip always agree. */
    picked?: readonly number[];
    /** showSubmit mirrors the inline row's submit gate (logic.showSubmit): a Min==Max==1 ask has none — the click posts. */
    showSubmit?: boolean;
    canSubmit?: boolean;
    busy?: boolean;
    /** onPick is the one posting path: the caller's own click handler (logic.click), not a private edit state. */
    onPick: (index: number) => void;
    onSubmit: () => void;
    onClose: () => void;
    /** resolver resolves the previewed option's name to its printed facts; injectable for the SSR test harness, exactly as ArrangeModal's is. Production never passes it. */
    resolver?: (name: string) => OracleCard | null | Promise<OracleCard | null>;
    /** preview0 seeds the hovered preview for the SSR test harness (no pointer events), the way ArrangeModal's is. Production never passes it. */
    preview0?: number | null;
  } = $props();

  let dialog = $state<HTMLElement | null>(null);

  // Seeded from the prop (a test's injected preview) at mount/SSR time only
  // — in production only a pointer/focus event ever sets it (the same shape
  // ArrangeModal's preview seed has).
  const initialPreview = () => preview0 ?? null;
  let preview = $state<number | null>(initialPreview());

  /**
   * The decision sequence the hover preview belongs to. The whole surface
   * belongs to ONE ask: a decision with a different seq arriving while this
   * modal is mounted must not keep the old ask's hover, or the aside could
   * name a card the new ask does not offer. The caller closes the modal
   * across a decision change (SeatPanelState.adopt); this is the modal's own
   * guard, so a future mount site that forgets to close still cannot leak it.
   */
  let shownSeq = $state<number | null>(null);

  /** The submit button's label, exactly the inline row's own wording. */
  const submitLabel = $derived(
    decision.min === 0 ? 'Confirm' : decision.min === decision.max ? `Choose ${decision.min}` : `Choose ${decision.min}–${decision.max}`,
  );

  function previewedName(): string | null {
    if (preview === null) return null;
    const o = decision.options.find((x) => x.index === preview);
    return o ? o.label : null;
  }

  // The preview's printed description, resolved by name through the catalog —
  // the same best-effort resolution ArrangeModal's aside uses: the default
  // resolver is the async catalog, which with no meta tag resolves null
  // without a request, and the block renders nothing until a resolver returns
  // text — no catalog is the NORMAL case, not an error. The initialiser
  // covers the synchronous-resolver path (server render never runs the effect
  // below, so a seeded preview's text must be in place from the first render).
  let previewOrc = $state<OracleCard | null>(initialPreviewOrc());

  function initialPreviewOrc(): OracleCard | null {
    const name = previewedName();
    if (name === null) return null;
    const v = resolver(name);
    if (v !== null && typeof (v as Promise<OracleCard | null>).then !== 'function') return v as OracleCard;
    return null;
  }

  $effect(() => {
    const name = previewedName();
    if (name === null) {
      previewOrc = null;
      return;
    }
    // The art and name switch synchronously with `preview`; clear the prior
    // card's printed facts in that same effect before a possibly-async lookup
    // starts, so card B can never be shown with card A's oracle text.
    previewOrc = null;
    const v = resolver(name);
    if (v === null) return;
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
    if (shownSeq !== null && seq !== shownSeq) preview = null;
    shownSeq = seq;
  });

  $effect(() => {
    if (open) {
      void tick().then(() => dialog?.focus());
    } else {
      // Closing drops the hover: reopening starts clean. The aside's derived
      // reads `preview`, so the closed render is already empty; this only
      // makes the NEXT open fresh.
      preview = null;
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

  function submit(): void {
    onSubmit();
  }
</script>

<svelte:window onkeydown={keydown} />

{#if open}
  <!-- The backdrop is pointer-dismissable while Escape is handled globally.
       role="dialog" + aria-modal="true" are the STRUCTURAL modal guard's
       markers (lib/hotkeys.ts MODAL_PICKER_SELECTOR); the data markers are
       the stable compatibility hooks, exactly as ArrangeModal's are. -->
  <div class="backdrop" data-discard-backdrop data-discard-modal data-pile-modal role="presentation" use:portal onclick={backdropClick}>
    <div
      class="dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="discard-modal-title"
      tabindex="-1"
      bind:this={dialog}
    >
      <header>
        <h2 id="discard-modal-title">{decision.prompt}</h2>
        <button type="button" class="close" aria-label="Close the card view" onclick={onClose}>×</button>
      </header>
      <div class="body" data-discard-body>
        <div class="grid-wrap">
          <ul class="cards" data-discard-grid>
            {#each decision.options as o (o.index)}
              {@const card = discardCard(decision, o)}
              {@const at = picked.indexOf(o.index)}
              <li class="card" data-discard-card={o.index}>
                <button
                  type="button"
                  class="face"
                  aria-pressed={at >= 0}
                  aria-label={o.label}
                  onclick={() => onPick(o.index)}
                  disabled={busy}
                  onpointerenter={() => { preview = o.index; }}
                  onpointerleave={() => { if (preview === o.index) preview = null; }}
                  onfocus={() => { preview = o.index; }}
                  onblur={() => { if (preview === o.index) preview = null; }}
                >
                  <CardImage {card} size="tile" pt={false} />
                  {#if at >= 0}<span class="order">{at + 1}</span>{/if}
                  <span class="name">{o.label}</span>
                </button>
              </li>
            {/each}
          </ul>
          <p class="hint">{picked.length} picked{showSubmit ? ` · ${decision.min === decision.max ? `exactly ${decision.min}` : `between ${decision.min} and ${decision.max}`}` : ''}</p>
        </div>
        <aside class="preview" data-discard-preview>
          {#if preview !== null}
            {@const o = decision.options.find((x) => x.index === preview)}
            {#if o}
              {@const card = discardCard(decision, o)}
              <CardImage {card} size="large" pt={false} />
              <p class="name">{o.label}</p>
            {/if}
          {:else}
            <p class="hint">Hover a card for full art.</p>
          {/if}
          {#if previewOrc?.oracle_text}<p class="oracle" data-discard-preview-oracle>{previewOrc.oracle_text}</p>{/if}
        </aside>
      </div>
      {#if showSubmit}
        <footer>
          <p class="hint">{picked.length} of the offered cards picked</p>
          <button
            type="button"
            class="submit"
            data-discard-submit
            onclick={submit}
            disabled={!canSubmit || busy}
          >{submitLabel}</button>
        </footer>
      {/if}
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
  .grid-wrap {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
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
    border-radius: var(--radius-card);
  }
  .face {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.2rem;
    width: 100%;
    padding: 0;
    background: transparent;
    border: 0;
    border-radius: var(--radius-card);
    cursor: pointer;
    line-height: 0;
  }
  .face:focus-visible {
    outline: 2px solid var(--initiative);
  }
  .face:disabled {
    opacity: 0.4;
    cursor: default;
  }
  .face .name {
    line-height: 1.35;
  }
  .card .order {
    position: absolute;
    top: -0.4em;
    left: -0.4em;
    z-index: 1;
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
  .hint {
    margin: 0;
    font-size: var(--t-11);
    color: var(--ink-faint);
  }
  .preview {
    flex: none;
    width: var(--card-w-large, 220px);
    /* Size-invariant, for the same reason ArrangeModal's aside is: a hover
       that resizes the dialog shifts the rows under the cursor and breathes.
       The printed description's box is a FIXED height, scrolled when long,
       its height reserved in min-height below — so the aside is the same
       height in all four states and a resolved description can never re-open
       the loop. With no catalog the box renders nothing and the space stays
       reserved: the no-catalog degradation, not a hole. */
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
    justify-content: flex-end;
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
