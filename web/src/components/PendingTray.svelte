<script lang="ts">
  import type { PendingView } from '../protocol';

  /** PendingTray lists the triggers/replacement effects waiting on a decision, each labelled and, when optional, saying who decides. */
  let { pending }: { pending: PendingView[] } = $props();
</script>

{#if pending.length === 0}
  <p class="empty">Nothing waiting</p>
{:else}
  <ul class="pending">
    {#each pending as p (p.source)}
      <li data-obj={p.source} class:optional={p.optional}>
        <span class="label">{p.label}</span>
        {#if p.optional}
          <span class="who">Seat {p.decider ?? p.controller} chooses</span>
        {/if}
      </li>
    {/each}
  </ul>
{/if}

<style>
  /*
   * The one thing no surveyed platform does: showing what is ABOUT to hit the
   * stack, in the same visual language as the stack itself. The tray sits
   * directly above the transcript and below the stack, so adjacency encodes
   * the mechanic — these entries descend into the list above them.
   *
   * This is where the design spends its boldness (design plan, principle 4):
   * the leading rule is the only place in the rail that carries the initiative
   * colour, so the eye finds it before anything else in the instrument.
   */
  .pending {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .pending li {
    position: relative;
    font-size: var(--t-12);
    line-height: 1.35;
    padding: var(--sp-2) var(--sp-2) var(--sp-2) var(--sp-3);
    background: var(--instrument-raised);
    border-left: 2px solid var(--initiative);
  }
  /* An optional trigger is a question, not an inevitability: the rule goes
     hollow so the two read differently at a glance. */
  .pending li.optional {
    border-left-color: var(--offered);
  }
  .label {
    display: block;
    color: var(--ink-inst);
  }
  .who {
    display: block;
    margin-top: 2px;
    font-family: var(--font-data);
    font-size: 0.6875rem;
    color: var(--ink-faint);
  }
  .empty {
    margin: 0;
    font-size: var(--t-12);
    color: var(--ink-faint);
  }
</style>
