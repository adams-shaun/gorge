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
    <span class="badge state-{state}">{state.toUpperCase()}</span>
  </header>

  {#if w}
    <LifeGrid life={w.life} lost={w.lost} seats={table.seats} active={w.active} />
    <div class="marker">T{w.turn} · {w.phase}</div>
    <div class="stack">{w.stack_depth} on stack</div>
  {:else}
    <div class="empty">waiting for a match…</div>
  {/if}
</button>

<style>
  .cell {
    display: flex;
    flex-direction: column;
    gap: .5rem;
    padding: .75rem;
    background: #1b1b1f;
    border: 1px solid #333;
    border-radius: 8px;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
  }
  .cell.halted { border-color: #e5484d; box-shadow: 0 0 0 1px #e5484d inset; }
  header { display: flex; align-items: center; justify-content: space-between; gap: .5rem; }
  .name { font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .badge { font-size: .7rem; letter-spacing: .05em; padding: .15rem .4rem; border-radius: 999px; background: #333; }
  .badge.state-live { background: #22c55e; color: #05130a; }
  .badge.state-cooldown { background: #eab308; color: #1a1400; }
  .badge.state-halted { background: #e5484d; color: white; }
  .marker { text-align: center; font-variant-numeric: tabular-nums; opacity: .85; }
  .stack { text-align: center; font-size: .75rem; opacity: .6; }
  .empty { text-align: center; opacity: .5; padding: 1rem 0; }
</style>
