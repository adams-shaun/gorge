<script lang="ts">
  import { onMount } from 'svelte';
  import type { MatchInfo } from '../protocol';
  import { fetchMatches } from '../lib/api';
  import { href, navigate } from '../lib/router';

  /** MatchList is a table's match history — every finished (or in-progress) match, in host order, linking to its replay. */
  let { table }: { table: string } = $props();

  let matches = $state<MatchInfo[]>([]);
  let loaded = $state(false);

  onMount(() => {
    void fetchMatches(table).then((m) => { matches = m; loaded = true; }).catch(() => { loaded = true; });
  });

  function winnerLabel(m: MatchInfo): string {
    if (m.winner === null || m.winner === undefined) return m.result ?? (m.state === 'live' ? 'in progress' : '—');
    return m.seats[m.winner]?.name ?? `seat ${m.winner}`;
  }

  function open(e: MouseEvent, k: number) {
    e.preventDefault();
    navigate({ kind: 'match', table, match: k });
  }
</script>

<div class="matches">
  <h2>Matches — {table}</h2>
  {#if !loaded}
    <p class="empty">Loading…</p>
  {:else if matches.length === 0}
    <p class="empty">No matches yet.</p>
  {:else}
    <table>
      <thead>
        <tr><th>#</th><th>State</th><th>Result</th><th>Turns</th><th>Events</th></tr>
      </thead>
      <tbody>
        {#each matches as m (m.match)}
          <tr>
            <td>
              <a href={href({ kind: 'match', table, match: m.match })} onclick={(e) => open(e, m.match)}>{m.match}</a>
            </td>
            <td>{m.state}</td>
            <td>{winnerLabel(m)}</td>
            <td>{m.turns}</td>
            <td>{m.events}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

<style>
  .matches { padding: 1rem; color: #ddd; }
  h2 { font-size: 1rem; opacity: .8; margin: 0 0 .75rem; }
  table { width: 100%; border-collapse: collapse; font-variant-numeric: tabular-nums; }
  th, td { text-align: left; padding: .35rem .6rem; border-bottom: 1px solid #333; }
  th { opacity: .6; font-weight: 500; font-size: .8rem; }
  a { color: #6cf; }
  .empty { opacity: .5; }
</style>
