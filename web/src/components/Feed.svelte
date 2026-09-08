<script lang="ts">
  import { isRoutineLine, latestPerTable, notableLines, type FeedLine } from '../lib/feed';
  import { SEAT_COLOURS } from '../lib/colours';
  import { parseLogLine } from '../lib/logrender';
  import ManaSymbols from './ManaSymbols.svelte';

  /**
   * Feed lines are the same server-described lines the match transcript uses
   * (widget.last, view/describe.go), so they get the same readable-log
   * rendering (ui9): a card/permanent reference's `#<id>` is suppressed onto
   * a hover title (the id is still load-bearing — two copies of the same card
   * — so it stays reachable), mana symbols render as pips, and a faceless
   * ability reference is italicised. There is no match view at the lobby rail
   * to look a card's colour up from, so card names render uncoloured here.
   *
   * Step lines ("Step: main-1") are clock noise and are suppressed by the
   * feed itself (feed.ts's isStepLine, keyed on view/describe.go's literal
   * "Step: " prefix): they never reach the rail's 'now' row or the notable
   * log. The 'all' toggle reveals them alongside the routine priority lines.
   */

  /**
   * The lobby rail, in two registers (Task L2).
   *
   * Its content is `widget.last`: one line per burst per table, and today
   * essentially every one of them reads "<deck> is asked: priority". A single
   * chronological scroll of that is a heartbeat rendered as prose — it looked
   * like log spam because, as a log, it is. So the same stream is shown twice:
   *
   *   NOW      one stable row per table, newest line, repeats counted. It
   *            never reorders, so a glance finds a table by position.
   *   NOTABLE  the chronological lines that are not routine — the plays,
   *            the combat asks, the mulligans. Usually short. The toggle
   *            widens it to every line when you want the raw stream.
   *
   * `order` comes from the lobby's own section order so the rail's rows and
   * the grid's cells are in the same sequence.
   */
  let { lines, order }: { lines: FeedLine[]; order: string[] } = $props();

  let showAll = $state(false);
  const now = $derived(latestPerTable(lines, order));
  const notable = $derived(notableLines(lines, showAll));

  let log: HTMLDivElement | undefined;

  // Deterministic colour per table id (not a seat), independent of arrival order.
  function tagColour(id: string): string {
    let h = 0;
    for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0;
    return SEAT_COLOURS[h % SEAT_COLOURS.length];
  }

  $effect(() => {
    if (notable.length) log?.scrollTo({ top: log.scrollHeight });
  });
</script>

<div class="rail">
  <section class="now">
    <h2>Now</h2>
    {#each now as n (n.table)}
      <div class="line" class:routine={n.routine} style:--tag={tagColour(n.table)}>
        <span class="tag">{n.table}</span>
        <span class="text">{#if n.line}{#each parseLogLine(n.line) as p, i (i)}{#if p.kind === 'text'}{p.text}{:else if p.kind === 'mana'}<ManaSymbols cost={p.token} />{:else if p.kind === 'card'}<span class="obj card" title="{p.name} #{p.id}">{p.name}</span>{:else if p.kind === 'ability'}<span class="obj ability" title="an ability #{p.id}">{p.name}</span>{/if}{/each}{:else}—{/if}</span>
        {#if n.count > 1}<span class="run">&times;{n.count}</span>{/if}
      </div>
    {/each}
  </section>

  <section class="notable">
    <h2>
      {showAll ? 'Everything' : 'Notable'}
      <button type="button" aria-pressed={showAll} onclick={() => (showAll = !showAll)}>
        {showAll ? 'notable' : 'all'}
      </button>
    </h2>
    <div class="log" bind:this={log}>
      {#each notable as l (`${l.table}:${l.match}:${l.seq}`)}
        <div class="line" class:routine={isRoutineLine(l.line)} style:--tag={tagColour(l.table)}>
          <span class="tag">{l.table}</span>
          <span class="text">{#each parseLogLine(l.line) as p, i (i)}{#if p.kind === 'text'}{p.text}{:else if p.kind === 'mana'}<ManaSymbols cost={p.token} />{:else if p.kind === 'card'}<span class="obj card" title="{p.name} #{p.id}">{p.name}</span>{:else if p.kind === 'ability'}<span class="obj ability" title="an ability #{p.id}">{p.name}</span>{/if}{/each}</span>
        </div>
      {:else}
        <p class="none">Nothing yet but priority.</p>
      {/each}
    </div>
  </section>
</div>

<style>
  /* Instrument register: hairlines, small type, no fills. The rail explains;
     it never competes with the grid. */
  .rail {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .now {
    flex: none;
    border-bottom: 1px solid var(--edge-inst);
  }
  .notable {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  h2 {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--sp-2);
    margin: 0;
    padding: var(--sp-2) var(--sp-3);
    font-size: var(--t-11);
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-faint);
  }
  button {
    background: none;
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    color: var(--ink-inst);
    font: inherit;
    letter-spacing: 0;
    text-transform: none;
    padding: 0 var(--sp-2);
    cursor: pointer;
  }
  button:hover {
    border-color: var(--ink-faint);
  }
  .log {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding-bottom: var(--sp-2);
  }
  /* Table identity is a rule, not a fill — the same rule the life grid uses
     for a seat. Two hundred saturated pills stacked in a rail is decoration;
     a 2px edge is identity. */
  .line {
    display: flex;
    align-items: baseline;
    gap: var(--sp-2);
    padding: 0.15rem var(--sp-3);
    border-left: 2px solid var(--tag);
    font-size: var(--t-12);
    line-height: 1.35;
  }
  /* A routine line is kept, not hidden — you can see the table is ticking —
     but it is dimmed to the floor so a real line stands out beside it. */
  .line.routine .text {
    color: var(--ink-faint);
  }
  .tag {
    flex: none;
    color: var(--tag);
    font-family: var(--font-data);
    font-size: var(--t-10);
    font-weight: 600;
  }
  .text {
    overflow-wrap: anywhere;
  }
  /* Same readable-log rendering as the match transcript (ui9). */
  .obj.card {
    font-weight: 600;
  }
  .obj.ability {
    font-style: italic;
  }
  /* The repeat count is a value, so it reads in the data face. */
  .run {
    flex: none;
    margin-left: auto;
    font-family: var(--font-data);
    font-size: var(--t-10);
    color: var(--ink-faint);
  }
  .none {
    margin: 0;
    padding: var(--sp-2) var(--sp-3);
    font-size: var(--t-12);
    color: var(--ink-faint);
  }
</style>
