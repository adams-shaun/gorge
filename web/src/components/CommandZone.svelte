<script lang="ts">
  import type { PlayerView, StackView } from '../protocol';
  import { commandZoneOf, nextCastCost, stackIdsOf } from '../lib/commander';
  import ManaSymbols from './ManaSymbols.svelte';

  /**
   * CommandZone is one seat's command zone on the rail, for every viewer —
   * ZCommand is a public zone and commander identity is the premise of the
   * format, so a seat-scoped view carries it exactly as a spectator's does.
   * One line per roster commander, followed by the facts the reader is
   * deciding with: where it is, and — only while it sits in the command
   * zone — what the next cast costs. The printed cost renders as its own
   * pips and the CR 903.8 tax as an explicit "+{2} × casts" chip: the
   * derived number the reader pays over the printed cost, never the raw
   * cast count, and the total is spelled out in the line's aria-label.
   * `data-tax`/`data-casts` carry the structured numbers for tests and for
   * the design pass.
   *
   * The in-zone/away split is the "distinguishable from the battlefield"
   * contract: a commander in the command zone renders with the command-zone
   * marker and its cast cost; one that has left the zone renders with its
   * current zone (battlefield, graveyard, exile, stack, hand, library) and
   * no cost, because there is no command-zone cast to price. That marker is
   * data, never label text: `data-cmd-zone` carries the structured zone the
   * reader keyed off.
   */
  let { player, colour, stack = [] }: { player: PlayerView; colour: string; stack?: StackView[] } = $props();

  const commanders = $derived(commandZoneOf(player, stackIdsOf(stack)));
</script>

<section class="command" data-command-zone data-seat={player.seat} style:border-left-color={colour}>
  <h3>{player.name}'s command</h3>
  {#if commanders.length === 0}
    <p class="empty" data-command-empty>No commanders</p>
  {:else}
    <ul>
      {#each commanders as c (c.commander.id)}
        <li
          data-commander={c.index}
          data-next-cost={nextCastCost(c.commander.mana_cost, c.casts)}
          aria-label={c.inZone
            ? `Next commander cast from the command zone: ${c.commander.name}${c.tax > 0 ? `, plus ${c.tax} generic commander tax` : ', no commander tax'}`
            : `${c.commander.name} is in the ${c.zone}`}
        >
          <span class="name">{c.commander.name}</span>
          {#if c.inZone}
            <span class="tag in-zone" data-cmd-zone="command" title="In the command zone — castable">command zone</span>
            <ManaSymbols cost={c.commander.mana_cost ?? ''} />
            {#if c.tax > 0}
              <span class="tax data" data-tax={c.tax} data-casts={c.casts} title="Commander tax (CR 903.8): {c.tax} generic for {c.casts} prior cast{c.casts === 1 ? '' : 's'}">+{c.tax}</span>
            {/if}
          {:else}
            <span class="tag away" data-cmd-zone={c.zone} title="Not in the command zone — cannot be cast from here">{c.zone}</span>
          {/if}
        </li>
      {/each}
    </ul>
    {#if !commanders.some((c) => c.inZone)}
      <p class="empty" data-command-zone-empty>The command zone is empty</p>
    {/if}
  {/if}
</section>

<style>
  /* The instrument register, like the rest of the rail: hairlines and flat
     raised panels. The seat colour is the only saturated thing, and it stays
     at the edge where HandList puts it. */
  .command {
    margin-bottom: var(--sp-3);
    border-left: 3px solid transparent;
    padding-left: var(--sp-2);
  }
  h3 {
    margin: 0;
    font-size: var(--t-12);
    font-weight: 600;
    color: var(--ink-inst);
    line-height: 1.3;
  }
  ul {
    list-style: none;
    margin: var(--sp-1) 0 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  li {
    display: flex;
    align-items: baseline;
    gap: var(--sp-2);
    padding: 1px var(--sp-1);
    font-size: var(--t-12);
    line-height: 1.5;
    min-width: 0;
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--ink-inst);
  }
  .tag {
    flex: none;
    font-size: 0.6875rem;
    line-height: 1.6;
    padding: 0 0.4em;
    border-radius: 2px;
  }
  /* The command-zone marker is the one that means "castable": it carries the
     initiative colour the engine reserves for what the game is waiting on. */
  .tag.in-zone {
    background: color-mix(in srgb, var(--initiative) 18%, transparent);
    color: var(--initiative);
  }
  .tag.away {
    background: var(--instrument-raised);
    color: var(--ink-faint);
  }
  .tax {
    flex: none;
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    font-size: 0.6875rem;
    color: var(--ink-faint);
    white-space: nowrap;
  }
  .empty {
    margin: var(--sp-1) 0 0;
    font-size: var(--t-12);
    color: var(--ink-faint);
  }
</style>
