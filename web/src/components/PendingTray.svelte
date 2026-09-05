<script lang="ts">
  import type { PendingView } from '../protocol';

  /** PendingTray lists the triggers/replacement effects waiting on a decision, each labelled and, when optional, saying who decides. */
  let { pending }: { pending: PendingView[] } = $props();
</script>

{#if pending.length === 0}
  <p class="empty">none</p>
{:else}
  <ul class="pending">
    {#each pending as p (p.source)}
      <li data-obj={p.source}>
        {p.label}{#if p.optional} · optional · decider {p.decider ?? p.controller}{/if}
      </li>
    {/each}
  </ul>
{/if}

<style>
  .pending { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: .25rem; }
  .pending li { font-size: .8rem; background: #1b1b1f; border-radius: 4px; padding: .3rem .5rem; }
  .empty { margin: 0; font-size: .8rem; opacity: .5; }
</style>
