<script lang="ts">
  import { onMount } from 'svelte';
  import { fetchCapabilities } from '../lib/api';
  import { listEventSets } from '../lib/eventapi';
  import { commanderLabel, commanderSets } from '../lib/newevent';
  import type { EventSetInfo } from '../protocol';

  /**
   * The lobby's draft-event entry point, beside "Play 1v1 vs a bot": a
   * button to /events/new, drawn only on a server that serves /api/events.
   * It asks GET /api/capabilities, which always answers 200, rather than
   * probing /api/events, which 404s on a server without -events (the smoke
   * gate counts any 4xx as a browser failure). A failed fetch leaves it
   * hidden. initialEnabled seeds the answer for the SSR test harness, where
   * onMount never runs.
   *
   * With events on, it also reads the set catalogue: a set the server hosts
   * a Commander draft for (EventSetInfo.commander) adds a second button,
   * straight to the create screen in that mode.
   */
  let { initialEnabled = false }: { initialEnabled?: boolean } = $props();
  // svelte-ignore state_referenced_locally
  let enabled = $state(initialEnabled);
  let sets = $state<EventSetInfo[]>([]);
  const commander = $derived(commanderSets(sets).length > 0);

  onMount(async () => {
    try {
      enabled = (await fetchCapabilities()).events;
    } catch {
      enabled = false;
    }
    if (!enabled) return;
    try {
      sets = (await listEventSets()) ?? [];
    } catch {
      sets = [];
    }
  });
</script>

{#if enabled}
  <div class="draftentry" data-testid="draft-entry">
    <div class="text">
      <h2>Draft a 4-player pod</h2>
      <p class="hint">Open three packs with three bots, build a 40-card deck, then play a best-of-one round robin.</p>
    </div>
    <a class="draft" href="/events/new" data-testid="draft-button">Draft</a>
  </div>
  {#if commander}
    <div class="draftentry" data-testid="commander-draft-entry">
      <div class="text">
        <h2>{commanderLabel(sets)}</h2>
        <p class="hint">Draft commanders with the pod, build around one or a partnered pair, then play a four-player Commander free-for-all.</p>
      </div>
      <a class="draft" href="/events/new?mode=commander" data-testid="commander-draft-button">Commander draft</a>
    </div>
  {/if}
{/if}

<style>
  .draftentry {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--sp-2) var(--sp-4);
    padding: var(--sp-3) var(--sp-4);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    background: var(--instrument);
    color: var(--ink-inst);
    /* Same column as PlayVsBot and DeckLeaderboard, so the strips line up. */
    max-width: 32rem;
    flex: none;
  }
  .text {
    flex: 1 1 16rem;
    min-width: 0;
  }
  h2 {
    margin: 0;
    font-size: var(--t-14);
    font-weight: 600;
    color: var(--ink);
  }
  .hint {
    margin: 0;
    font-size: var(--t-12);
    color: var(--ink-dim);
  }
  .draft {
    background: var(--gilt);
    color: var(--felt-sunk);
    border-radius: var(--radius);
    padding: 0.4rem 1.1rem;
    font-weight: 600;
    font-size: var(--t-14);
    text-decoration: none;
  }
</style>
