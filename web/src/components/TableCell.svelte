<script lang="ts">
  import type { TableState } from '../lib/tables.svelte';
  import { navigate } from '../lib/router';
  import LifeGrid from './LifeGrid.svelte';

  // `roomy` is the page's call, not the cell's: the lobby sizes its grid from
  // how many tables it is showing (lib/lobby.ts), and a roomy cell spends the
  // extra space on seat names and a life total you can read across a room.
  // One component either way — a Commander cell and a constructed cell differ
  // only in which section they sit in.
  // `note` is the last thing that actually happened at this table (the feed's
  // last non-routine line). A compact cell has no room for it and does not
  // ask for one.
  let { table, roomy = false, note = '' }: { table: TableState; roomy?: boolean; note?: string } = $props();

  const state = $derived(table.info.state);
  const halted = $derived(state === 'halted');
  const w = $derived(table.widget);

  function onclick() {
    navigate({ kind: 'table', table: table.info.id });
  }
</script>

<button type="button" class="cell" class:halted class:roomy onclick={onclick}>
  <header>
    <span class="name">{table.info.name}</span>
    <span class="state state-{state}">{state}</span>
  </header>

  {#if w}
    <!-- The life grid is the cell's content, so it takes the height the
         header and footer do not; the wrapper exists only to hand it that
         height. -->
    <div class="life"><LifeGrid life={w.life} lost={w.lost} seats={table.seats} active={w.active} names={roomy} /></div>
    {#if roomy}<p class="note">{note}</p>{/if}
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
  /* A roomy cell is the same cell with more air. The life total's size is not
     set here: the page sets --life-size once (lib/lobby.ts), so every cell on
     the page reads at one size whichever section it is in. */
  .cell.roomy {
    gap: var(--sp-4);
    padding: var(--sp-4);
  }
  .life {
    flex: 1;
    display: grid;
  }
  /* What last happened here. One line, truncated, and its height is held even
     when there is nothing to say, so a cell never resizes as the game runs. */
  .note {
    margin: 0;
    min-height: 1.35em;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--t-12);
    color: var(--ink-dim);
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
  /* A table between matches keeps its size — the grid must not reflow every
     time a cooldown ends somewhere on the page. */
  .empty {
    flex: 1;
    display: grid;
    place-items: center;
    padding: var(--sp-6) 0;
    text-align: center;
    font-size: var(--t-14);
    color: var(--ink-faint);
  }
</style>
