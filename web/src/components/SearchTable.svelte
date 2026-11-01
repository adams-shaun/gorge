<script lang="ts">
  import type { ExplainRow, ExplainSearch } from '../lib/review';
  import { signed } from '../lib/review';

  /**
   * SearchTable is a bot search's root table and its footer: candidates,
   * visits, prior and Q, the root value and what the search cost. The match
   * reviewer and the paired bot both show it. It reads no game state; label
   * names a row's candidate, and chosenTag and botTag are the words on the row
   * the search picked and the row its heuristic would answer ("" for none).
   * Rows are not controls.
   */
  let {
    search,
    label,
    chosenTag = 'chosen',
    botTag = 'bot',
  }: {
    search: ExplainSearch;
    label: (r: ExplainRow) => string;
    chosenTag?: string;
    botTag?: string;
  } = $props();

  const rows = $derived(search.rows ?? []);
  const maxVisits = $derived(Math.max(1, ...rows.map((r) => r.visits)));
</script>

<table class="search" aria-label="Search">
  <thead>
    <tr><th class="c-ans">Candidate</th><th class="c-vis">Visits</th><th class="num c-prior">Prior</th><th class="num">Q</th></tr>
  </thead>
  <tbody>
    {#each rows as r, i (i)}
      <tr class:chosen={r.chosen} class:bot={r.bot}>
        <td class="c-ans" title={label(r)}>
          <span class="ans"><span class="cand">{label(r)}</span>{#if r.chosen}<span class="tag tag-chosen">{chosenTag}</span>{/if}{#if r.bot && botTag}<span class="tag tag-bot">{botTag}</span>{/if}</span>
        </td>
        <td class="c-vis">
          <span class="vis"><span class="track"><span class="bar" style:width={`${(r.visits / maxVisits) * 100}%`}></span></span><span class="vn">{r.visits}</span></span>
        </td>
        <td class="num c-prior">{r.prior.toFixed(2)}</td>
        <td class="num" class:neg={r.q < 0}>{signed(r.q)}</td>
      </tr>
    {/each}
  </tbody>
</table>
<p class="foot">
  <span>Root <b class:neg={search.root_value < 0}>{signed(search.root_value)}</b></span>
  {#if search.sims}<span>{search.sims} sims</span>{/if}
  {#if search.ms}<span>{Math.round(search.ms)} ms</span>{/if}
  <span>{search.engine}{search.cheap ? ', cheap' : ''}</span>
  {#if search.dropped}<span>{search.dropped} more not listed</span>{/if}
</p>

<style>
  .tag { flex: none; font-size: var(--t-11); }
  .tag-chosen { color: var(--gilt); }
  .tag-bot { color: var(--verdigris); }

  .search { width: 100%; margin-top: var(--sp-3); border-collapse: collapse; table-layout: fixed; font-variant-numeric: tabular-nums; }
  .search th {
    text-align: left;
    font-weight: 400;
    color: var(--ink-faint);
    padding: 0 var(--sp-1) 2px 0;
    border-bottom: 1px solid var(--edge-inst);
  }
  .search td { padding: 3px var(--sp-1) 3px 0; vertical-align: baseline; border-bottom: 1px solid color-mix(in srgb, var(--edge-inst) 45%, transparent); }
  .search .c-ans { width: 46%; }
  .search .c-vis { width: 28%; }
  .search .num { width: 13%; text-align: right; }
  .ans { display: flex; align-items: baseline; gap: var(--sp-1); min-width: 0; }
  .cand { flex: 0 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .search tr.chosen .cand { color: var(--ink); }
  .vis { display: flex; align-items: center; gap: var(--sp-1); }
  .track { flex: 1; min-width: 0; height: 0.5rem; }
  .bar {
    display: block;
    height: 100%;
    min-width: 1px;
    border-radius: 1px;
    background: color-mix(in srgb, var(--ink-inst) 30%, transparent);
  }
  .search tr.chosen .bar { background: color-mix(in srgb, var(--gilt) 70%, transparent); }
  .search tr.bot:not(.chosen) .bar { background: color-mix(in srgb, var(--verdigris) 60%, transparent); }
  .vn { flex: none; min-width: 2.2em; text-align: right; color: var(--ink-inst); }
  .neg { color: var(--ink-dim); }
  .foot { display: flex; flex-wrap: wrap; gap: var(--sp-1) var(--sp-3); margin: var(--sp-2) 0 0; color: var(--ink-faint); }
  .foot b { font-weight: 600; color: var(--ink); }
  .foot b.neg { color: var(--ink-dim); }

  /* A narrow rail keeps the candidate, its visits and Q; the prior goes. */
  @container (max-width: 17rem) {
    .search .c-prior { display: none; }
    .search .c-ans { width: 52%; }
    .search .c-vis { width: 30%; }
    .search .num { width: 18%; }
  }
  /* A sliver of a rail: the candidate and its Q only. */
  @container (max-width: 11rem) {
    .search .c-vis, .search .c-prior { display: none; }
    .search .c-ans { width: 65%; }
    .search .num { width: 35%; }
    .search .tag { display: none; }
    .search tr.chosen td:first-child { box-shadow: inset 2px 0 0 var(--gilt); padding-left: var(--sp-1); }
    .search tr.bot:not(.chosen) td:first-child { box-shadow: inset 2px 0 0 var(--verdigris); padding-left: var(--sp-1); }
  }
</style>
