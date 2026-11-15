<script lang="ts">
  import type { StandingRow } from '../../lib/standings';

  /**
   * The standings table: the server's rows in the server's order (MTR
   * Appendix C — match points, then OMW%, GW%, OGW%, with the 33% floor).
   * Nothing here re-ranks or recomputes a percentage. The server sends no
   * rows until it has logged the first result (results log in pairing
   * order, so a finished series can wait on its round-mate), and until then
   * this says so instead of drawing an empty (or all-zero) table.
   * A Commander draft (`pod`) ranks free-for-all games, which have no
   * opponents' or game-win percentages: those columns are dropped.
   */
  let { rows, final = false, pod = false }: { rows: StandingRow[]; final?: boolean; pod?: boolean } = $props();
</script>

{#if rows.length === 0}
  <h3 class="ev-h5 cap">{final ? 'Final standings' : 'Standings'}</h3>
  <p class="empty" data-testid="standings-empty">No results yet. Standings appear once the server records the first result.</p>
{:else}
<div class="tablebox">
  <table data-testid="standings">
    <caption class="ev-h5">{final ? 'Final standings' : 'Standings'}</caption>
    <thead>
      <tr>
        <th scope="col">#</th>
        <th scope="col">Seat</th>
        <th scope="col" title="Match points">Pts</th>
        <th scope="col" title={pod ? 'Pod games won and lost' : 'Match record (wins-losses)'}>W-L</th>
        {#if !pod}
          <th scope="col" title="Opponents' match-win percentage">OMW%</th>
          <th scope="col" title="Game-win percentage">GW%</th>
          <th scope="col" title="Opponents' game-win percentage">OGW%</th>
        {/if}
      </tr>
    </thead>
    <tbody>
      {#each rows as r (r.seat)}
        <tr class:you={r.you} data-seat={r.seat}>
          <td class="data">{r.rank}</td>
          <td class="name">{#if r.you}<b>{r.label}</b>{:else}{r.label}{/if}</td>
          <td class="data">{r.points}</td>
          <td class="data">{r.record}</td>
          {#if !pod}
            <td class="data">{r.omw}</td>
            <td class="data">{r.gw}</td>
            <td class="data">{r.ogw}</td>
          {/if}
        </tr>
      {/each}
    </tbody>
  </table>
</div>
{#if pod}
  <p class="note">Points and order are the server's: points for each free-for-all win.</p>
{:else}
  <p class="note">Order: match points, then OMW%, GW%, OGW% (MTR Appendix C, with the 33% floor).</p>
{/if}
{/if}

<style>
  .tablebox {
    overflow-x: auto;
    max-width: 100%;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--t-12);
  }
  caption {
    text-align: left;
    padding-bottom: var(--sp-2);
  }
  th {
    text-align: left;
    font: 500 var(--t-10) var(--font-data);
    letter-spacing: 0.06em;
    color: var(--ink-faint);
    border-bottom: 1px solid var(--edge-inst);
    padding: 0.3rem 0.4rem;
  }
  th,
  td.data {
    white-space: nowrap;
  }
  td {
    padding: 0.35rem 0.4rem;
    border-bottom: 1px solid color-mix(in srgb, var(--edge-inst) 50%, transparent);
    color: var(--ink-inst);
  }
  .name {
    overflow-wrap: anywhere;
  }
  tr.you td {
    color: var(--ink);
  }
  tr.you td:first-child {
    box-shadow: inset 2px 0 0 var(--gilt);
  }
  .cap {
    margin: 0 0 var(--sp-2);
  }
  .empty {
    margin: 0;
    font-size: var(--t-12);
    color: var(--ink-dim);
  }
  .note {
    margin: var(--sp-1) 0 0;
    font-size: var(--t-11);
    color: var(--ink-dim);
  }
</style>
