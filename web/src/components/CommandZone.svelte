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
   * U2/U4: this is a GROUP of rows inside the rail's one "Commanders"
   * section, not a panel with a heading of its own. Four "…'s command"
   * headings are four lines saying what the seat's colour rule already says,
   * and the measured cost of that repetition was the command zone falling
   * below the fold on the only format it exists for. Which seats get a group
   * at all is the rail's decision (commanderSeats): a constructed table draws
   * no section.
   *
   * The seat is therefore carried by the colour rule — the same rule, in the
   * same colour, that the seat table directly above uses, in the same seat
   * order, so identity carries across without being spelled twice — and by
   * every row's own title and accessible name, which do spell it.
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

<div class="command" data-command-zone data-seat={player.seat} style:border-left-color={colour}>
  {#if commanders.length === 0}
    <p class="empty" data-command-empty>No commanders</p>
  {:else}
    <ul>
      {#each commanders as c (c.commander.id)}
        <li
          data-commander={c.index}
          data-next-cost={nextCastCost(c.commander.mana_cost, c.casts)}
          title={c.inZone
            ? `${player.name} — ${c.commander.name} is in the command zone`
            : `${player.name} — ${c.commander.name} is in the ${c.zone}`}
          aria-label={c.inZone
            ? `${player.name} — next commander cast from the command zone: ${c.commander.name}${c.tax > 0 ? `, plus ${c.tax} generic commander tax` : ', no commander tax'}`
            : `${player.name} — ${c.commander.name} is in the ${c.zone}`}
        >
          <span class="name">{c.commander.name}</span>
          {#if c.inZone}
            <span class="tag in-zone" data-cmd-zone="command" title="In the command zone — castable">in zone</span>
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
      <p class="empty" data-command-zone-empty>{player.name}'s command zone is empty</p>
    {/if}
  {/if}
</div>

<style>
  /* The instrument register, like the rest of the rail: hairlines and flat
     raised panels. The seat colour is the only saturated thing, and it stays
     at the edge where HandList puts it. */
  .command {
    border-left: 3px solid transparent;
    padding-left: var(--sp-2);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
  }
  /* The row wraps rather than clips: the rail is ~250px of content width and
     a long commander name beside a marker and a cost will not always fit on
     one line. Wrapping costs a line only when it must. */
  li {
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: 0 var(--sp-2);
    padding: 0 var(--sp-1);
    font-size: var(--t-11);
    line-height: 1.4;
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
    margin: 0;
    padding: 0 var(--sp-1);
    font-size: var(--t-11);
    line-height: 1.5;
    color: var(--ink-faint);
  }
</style>
