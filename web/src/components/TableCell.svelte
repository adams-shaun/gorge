<script lang="ts">
  import type { TableState } from '../lib/tables.svelte';
  import { navigate } from '../lib/router';
  import LifeGrid from './LifeGrid.svelte';

  let { table }: { table: TableState } = $props();

  const state = $derived(table.info.state);
  const halted = $derived(state === 'halted');
  const w = $derived(table.widget);

  function onclick() {
    navigate({ kind: 'table', table: table.info.id });
  }
</script>

<button type="button" class="cell" class:halted onclick={onclick}>
  <header>
    <span class="name">{table.info.name}</span>
    <span class="state state-{state}">{state}</span>
  </header>

  {#if w}
    <LifeGrid life={w.life} lost={w.lost} seats={table.seats} active={w.active} />
    <footer>
      <span class="turn">Turn {w.turn}</span>
      <span class="phase">{w.phase}</span>
      <span class="stack">{w.stack_depth === 0 ? 'stack empty' : `${w.stack_depth} on stack`}</span>
    </footer>
  {:else}
    <div class="empty">Waiting for a match</div>
  {/if}
</button>

<style>
  /* A table cell is felt, not a floating card: it sits on the ground with a
     hairline, and lifts only under the pointer. No shared soft shadow. */
  .cell {
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
    padding: var(--sp-3);
    background: var(--felt-raised);
    border: 1px solid var(--edge-felt);
    border-radius: var(--radius);
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
  }
  .cell:hover {
    border-color: var(--ink-faint);
  }
  .cell.halted {
    border-color: var(--danger);
  }
  header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--sp-2);
  }
  .name {
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* State is a word, not a shouted pill. Colour carries it; a dot marks the
     live one so it reads without relying on hue alone. */
  .state {
    font-size: var(--t-12);
    color: var(--ink-dim);
    white-space: nowrap;
  }
  .state-live {
    color: var(--mana-g);
  }
  .state-live::before {
    content: '';
    display: inline-block;
    width: 0.4em;
    height: 0.4em;
    margin-right: 0.4em;
    border-radius: 999px;
    background: currentColor;
    vertical-align: 0.1em;
  }
  .state-cooldown {
    color: var(--initiative);
  }
  .state-halted {
    color: var(--danger);
  }
  footer {
    display: flex;
    justify-content: space-between;
    gap: var(--sp-2);
    font-family: var(--font-data);
    font-size: 0.6875rem;
    color: var(--ink-dim);
  }
  .phase {
    color: var(--ink-faint);
  }
  .stack {
    color: var(--ink-faint);
  }
  .empty {
    padding: var(--sp-6) 0;
    text-align: center;
    font-size: var(--t-14);
    color: var(--ink-faint);
  }
</style>
