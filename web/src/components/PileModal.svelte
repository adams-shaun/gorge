<script lang="ts">
  import { tick } from 'svelte';
  import type { CardView } from '../protocol';

  /**
   * PileModal is the single card-list surface for hand, graveyard and exile.
   * It is portalled to body so the rail's capped, scrolling regions cannot
   * clip it. The wire never exposes library cards, so library never reaches
   * this component.
   */
  let { open, title, cards, returnFocus = null, onClose }: {
    open: boolean;
    title: string;
    cards: CardView[];
    returnFocus?: HTMLElement | null;
    onClose: () => void;
  } = $props();

  let dialog = $state<HTMLElement | null>(null);
  let wasOpen = false;
  let lastReturnFocus: HTMLElement | null = null;

  $effect(() => {
    if (open) {
      wasOpen = true;
      lastReturnFocus = returnFocus;
      void tick().then(() => dialog?.focus());
    } else if (wasOpen) {
      wasOpen = false;
      lastReturnFocus?.focus();
      lastReturnFocus = null;
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
</script>

<svelte:window onkeydown={keydown} />

{#if open}
  <!-- The backdrop is pointer-dismissable while Escape is handled globally;
       it is not itself a keyboard stop because focus belongs in the dialog. -->
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="backdrop" data-pile-backdrop role="presentation" use:portal onclick={backdropClick}>
    <div
      class="dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="pile-modal-title"
      tabindex="-1"
      bind:this={dialog}
    >
      <header>
        <h2 id="pile-modal-title">{title}</h2>
        <button type="button" class="close" aria-label={`Close ${title}`} onclick={onClose}>×</button>
      </header>
      <ul class="cards" data-pile-scroll>
        {#each cards as card (card.id)}
          <li data-obj={card.id}>
            <span class="card-name">{card.name}</span>
            <span class="types">{card.types}</span>
          </li>
        {/each}
      </ul>
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
    width: min(34rem, calc(100vw - 2rem));
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
    text-transform: capitalize;
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
  .cards {
    margin: 0;
    padding: var(--sp-2) var(--sp-4) var(--sp-4);
    max-height: 70vh;
    overflow-y: auto;
    list-style: none;
  }
  li {
    display: flex;
    justify-content: space-between;
    gap: var(--sp-3);
    padding: 0.2rem 0;
    font-size: var(--t-12);
    line-height: 1.35;
  }
  .card-name {
    min-width: 0;
    overflow: hidden;
    color: var(--ink-inst);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .types {
    flex: none;
    margin-left: auto;
    color: var(--ink-faint);
    font-size: var(--t-11);
    white-space: nowrap;
  }
</style>
