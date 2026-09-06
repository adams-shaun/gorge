<script lang="ts">
  import type { PlayerView } from '../protocol';
  import { zonesFor, type ZoneName } from '../lib/zones';

  /** ZoneViewer is one player's zone strip on the rail: a row per zone — graveyard, exile, then the library count — each a count in mono with an expand affordance that discloses the card names, most-recently-added first. Library is count-only because the wire carries only `library_size`, never the cards, and no list is invented for it. Rows stay quiet until asked: a zero-count zone renders greyed with no expander, and a zone whose cards were redacted (the array null while its `_size` field is not) shows its count without a false expander. This component renders fields already on the view and decides nothing about the game. */
  let { player, colour, startOpen = false }: { player: PlayerView; colour: string; startOpen?: boolean } = $props();

  const zones = $derived(zonesFor(player));

  // Per-zone disclosure state. startOpen lets a caller render a zone already
  // expanded (and lets the component test assert the open rendering); it is
  // read once as the initial state — the rail only ever mounts a ZoneViewer
  // per seat, so disclosure must not track later prop changes.
  // svelte-ignore state_referenced_locally
  const open = $state<Record<ZoneName, boolean>>({ graveyard: startOpen, exile: startOpen });

  function toggle(z: ZoneName) {
    open[z] = !open[z];
  }

  function listId(z: ZoneName): string {
    return `zone-list-${player.seat}-${z}`;
  }
</script>

<section class="zones" style:border-left-color={colour}>
  <h3>{player.name}'s zones</h3>
  {#each zones as z (z.zone)}
    {#if z.count === 0}
      <div class="zone-row empty"><span class="zone-name">{z.zone}</span><span class="count">0</span></div>
    {:else if z.cards.length === 0}
      <div class="zone-row"><span class="zone-name">{z.zone}</span><span class="count">{z.count}</span></div>
    {:else}
      <button
        type="button"
        class="zone-row zone-toggle"
        aria-expanded={open[z.zone]}
        aria-controls={listId(z.zone)}
        onclick={() => toggle(z.zone)}>
        <span class="zone-name">{z.zone}</span>
        <span class="count">{z.count}</span>
        <span class="affordance" aria-hidden="true">{open[z.zone] ? '▾' : '▸'}</span>
      </button>
      {#if open[z.zone]}
        <ul id={listId(z.zone)} class="zone-cards" data-zone={z.zone}>
          {#each z.cards as c (c.id)}
            <li data-obj={c.id}>
              <span class="card-name">{c.name}</span>
              <span class="types">{c.types}</span>
            </li>
          {/each}
        </ul>
      {/if}
    {/if}
  {/each}
  <div class="zone-row">
    <span class="zone-name">library</span>
    <span class="count">{player.library_size}</span>
  </div>
</section>

<style>
  /* The instrument register, like the rest of the rail: hairlines and flat
     raised panels, never felt tokens. The seat colour is the only saturated
     thing, and it stays at the edge where HandList puts it. */
  .zones {
    margin-bottom: 0.75rem;
    border-left: 3px solid transparent;
    padding-left: 0.5rem;
  }
  h3 {
    margin: 0 0 0.25rem;
    font-size: var(--t-12);
    font-weight: 600;
    color: var(--ink-dim);
  }
  .zone-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    padding: 0.15rem 0;
    border: 0;
    background: none;
    font-size: var(--t-12);
    color: var(--ink-inst);
    text-align: left;
    cursor: pointer;
  }
  .zone-toggle:hover .affordance {
    color: var(--ink-inst);
  }
  .zone-row.empty {
    opacity: 0.45;
    cursor: default;
  }
  .zone-name {
    text-transform: capitalize;
  }
  /* Counts are values: mono, like every other figure on the rail. */
  .count {
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    color: var(--ink-faint);
  }
  .affordance {
    margin-left: auto;
    color: var(--ink-faint);
    font-size: 0.625rem;
  }
  .zone-cards {
    margin: 0 0 0.35rem;
    padding: 0.25rem 0.5rem;
    list-style: none;
    background: var(--instrument-raised);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    max-height: 10rem;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .zone-cards li {
    display: flex;
    justify-content: space-between;
    gap: 0.5rem;
    font-size: var(--t-12);
    line-height: 1.35;
  }
  .card-name {
    color: var(--ink-inst);
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .types {
    color: var(--ink-faint);
    font-size: 0.6875rem;
    white-space: nowrap;
    flex: none;
    margin-left: auto;
  }
</style>
