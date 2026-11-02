<script lang="ts">
  import { untrack } from 'svelte';
  import type { Advice, Decision, PairedBotStatus } from '../protocol';
  import { fetchAdvice, fetchPairedBotStatus } from '../lib/api';
  import { adviceRows, differedNote, loadPairedBotOn, pairedRowLabel, savePairedBotOn, shownRows, winChance, type PlayerAnswer } from '../lib/pairedbot';
  import type { ExplainNote, ExplainSearch } from '../lib/review';
  import { signed, sourceBadge } from '../lib/review';
  import type { SeatCtx } from '../lib/seat';

  /**
   * PairedBotPanel shows a seated player what a bot's search makes of the
   * decision in front of them, and says so when they answer differently. The
   * server computes it from what the seat can see; nothing here can answer the
   * decision. It renders nothing on a server without the option, and asks only
   * while the player's toggle is on.
   */
  let {
    table,
    match,
    ctx,
    decision,
    answered = null,
    onDiffered = () => {},
  }: {
    table: string;
    match: number;
    ctx: SeatCtx;
    decision: Decision | null;
    /** The seat's last accepted answer. */
    answered?: PlayerAnswer | null;
    /** Called with the note when that answer was not the search's recommendation. */
    onDiffered?: (text: string) => void;
  } = $props();

  /** How long a decision must stay in front of the player before the bot is asked: an auto-passed one never costs a search. */
  const SETTLE_MS = 200;
  /** How many candidates the panel lists; the rest are counted. */
  const ROWS = 5;
  /** Searches kept for comparing an answer: the answer's acknowledgement can arrive after the next decision. */
  const KEPT = 4;

  let status = $state<PairedBotStatus | null>(null);
  let on = $state(loadPairedBotOn());
  let advice = $state<Advice | null>(null);
  let failed = $state<string | null>(null);
  let attempt = $state(0);
  let differed = $state<string | null>(null);
  let advised: { seq: number; decision: Decision; search: ExplainSearch }[] = [];

  $effect(() => {
    const t = table;
    let live = true;
    fetchPairedBotStatus(t, ctx)
      .then((s) => {
        if (live) status = s;
      })
      .catch(() => {
        if (live) status = null;
      });
    return () => {
      live = false;
    };
  });

  // The seq this panel may show advice for: the player's own pending decision.
  const seq = $derived(decision !== null && decision.player === ctx.seat ? decision.seq : null);

  $effect(() => {
    const s = seq;
    void attempt;
    advice = null;
    failed = null;
    if (!on || !status?.offered || s === null) return;
    const asked = untrack(() => decision);
    const hangup = new AbortController();
    const timer = setTimeout(() => {
      fetchAdvice(table, match, s, ctx, hangup.signal)
        .then((a) => {
          if (hangup.signal.aborted || a.ask_seq !== s) return;
          advice = a;
          const search = a.state === 'ready' ? (a.explain as ExplainNote).search : undefined;
          if (search && asked) {
            advised = [...advised.filter((x) => x.seq !== s).slice(1 - KEPT), { seq: s, decision: asked, search }];
          }
        })
        .catch((e: unknown) => {
          if (!hangup.signal.aborted) failed = e instanceof Error ? e.message : String(e);
        });
    }, SETTLE_MS);
    return () => {
      clearTimeout(timer);
      hangup.abort();
    };
  });

  $effect(() => {
    const a = answered;
    if (a === null) return;
    untrack(() => {
      // An auto-pass after the player's answer asked nothing, and leaves the note up.
      const seen = advised.find((x) => x.seq === a.seq);
      if (!seen) return;
      advised = advised.filter((x) => x !== seen);
      differed = differedNote(seen.decision, seen.search, a);
      if (differed) onDiffered(differed);
    });
  });

  function toggle() {
    on = !on;
    savePairedBotOn(on);
  }

  const shown = $derived(advice !== null && advice.ask_seq === seq ? advice : null);
  const note = $derived(shown?.state === 'ready' ? (shown.explain as ExplainNote) : null);
  const badge = $derived(note ? sourceBadge(note) : null);
  const pick = $derived(note?.bot && decision && note.bot.length > 0 ? pairedRowLabel(decision, { choices: note.bot }) : null);
  const rows = $derived(note?.search && decision ? adviceRows(decision, note.search) : []);
  const list = $derived(shownRows(rows, ROWS, note?.search?.dropped ?? 0));
  const maxVisits = $derived(Math.max(1, ...rows.map((r) => r.visits)));
  const standing = $derived(note?.search ? winChance({ visits: note.search.sims ?? 0, q: note.search.root_value }) : null);
</script>

{#if status?.offered || status?.reason}
  <section class="paired" aria-label="Paired bot" data-paired-panel data-state={!status.offered ? 'unavailable' : !on ? 'off' : seq === null ? 'idle' : shown ? shown.state : failed ? 'failed' : 'working'}>
    <header>
      <h3>Paired bot</h3>
      {#if status.offered}
        <button type="button" class="switch" role="switch" aria-checked={on} onclick={toggle}>{on ? 'On' : 'Off'}</button>
      {/if}
    </header>
    {#if !status.offered}
      <p class="quiet">{status.reason}</p>
    {:else}
      {#if status.shared}
        <p class="shared">Any player at this table may be asking a paired bot.</p>
      {/if}
      {#if !on}
        <p class="quiet">Turn it on to see gorge's search look ahead on each of your decisions.</p>
      {:else}
        {#if seq === null}
          <p class="quiet">Nothing to weigh until your next decision.</p>
        {:else if failed}
          <p class="refused">The bot did not answer: {failed}</p>
          <button type="button" class="again" onclick={() => attempt++}>Ask again</button>
        {:else if !shown}
          <p class="quiet" role="status">Search is looking ahead…</p>
        {:else if shown.state !== 'ready'}
          <p class="refused">{shown.reason}</p>
        {:else if note && decision}
          {#if note.search}
            <p class="summary" data-paired-summary title="Each line plays on from here with a fresh guess at the cards you can't see. Estimates are the average of where those lines end up, not a measured win rate.">Search looked at {note.search.sims} lines{#if standing !== null}. You stand at about {standing}%{/if}</p>
            <table class="advice" aria-label="Search">
              <thead>
                <tr><th>Option</th><th class="c-vis">Looked at</th><th class="num">Est. win</th></tr>
              </thead>
              <tbody>
                {#each list.rows as r (r.label)}
                  <tr class:best={r.recommended}>
                    <td class="c-opt"><span class="opt">{r.label}</span>{#if r.recommended}<span class="tag">Recommended</span>{/if}</td>
                    <td class="c-vis"><span class="vis"><span class="track"><span class="bar" style:width={`${(r.visits / maxVisits) * 100}%`}></span></span><span class="vn">{r.visits}</span></span></td>
                    <td class="num">{r.chance === null ? '-' : `${r.chance}%`}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
            {#if list.more}<p class="quiet">and {list.more} more</p>{/if}
          {:else if note.source === 'forced'}
            <p class="pick" title={badge?.tip}>Nothing to weigh here{#if pick}: <b>{pick}</b>{/if}</p>
          {:else}
            <p class="pick" title={badge?.tip}>Search doesn't weigh this one.{#if pick} The bot's rules pick <b>{pick}</b>{/if}</p>
          {/if}
          {#if note.net}<p class="quiet">Net value {signed(note.net.value)}</p>{/if}
          {#if shown.reason}<p class="quiet">{shown.reason}</p>{/if}
        {/if}
        {#if differed}<p class="differed" data-paired-differed>{differed}</p>{/if}
        <p class="quiet fine">Advice only, from what you can see. It guesses your opponent's cards from a set of decks, and you still answer.</p>
      {/if}
    {/if}
  </section>
{/if}

<style>
  .paired {
    container-type: inline-size;
    flex: 0 1 auto;
    max-height: 45%;
    overflow-y: auto;
    padding: var(--sp-2) var(--sp-3) var(--sp-3);
    border-bottom: 1px solid var(--edge-inst);
    background: var(--instrument);
    color: var(--ink-inst);
    font-family: var(--font-ui);
    font-size: var(--t-12);
  }
  header { display: flex; align-items: baseline; justify-content: space-between; gap: var(--sp-2); }
  h3 { margin: 0; font-family: var(--font-serif); font-size: var(--t-14, var(--t-12)); font-weight: 600; color: var(--ink); }
  button {
    font: inherit;
    font-size: var(--t-11);
    color: var(--ink-inst);
    background: transparent;
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    padding: 2px var(--sp-2);
    cursor: pointer;
  }
  button:hover { background: var(--instrument-raised); }
  button:focus-visible { outline: 2px solid var(--gilt); outline-offset: 2px; }
  .switch[aria-checked='true'] { color: var(--gilt); border-color: color-mix(in srgb, var(--gilt) 55%, transparent); }
  .again { margin-top: var(--sp-1); }
  p { margin: var(--sp-1) 0 0; overflow-wrap: anywhere; }
  .quiet { color: var(--ink-faint); }
  .shared { color: var(--ember); }
  .refused { color: var(--ink-dim); }
  .pick { color: var(--ink-dim); }
  .summary { color: var(--ink); }
  .differed { color: var(--ember); }
  .fine { font-size: var(--t-11); }

  .advice { width: 100%; margin-top: var(--sp-2); border-collapse: collapse; table-layout: fixed; font-variant-numeric: tabular-nums; }
  .advice th { text-align: left; font-weight: 400; color: var(--ink-faint); padding: 0 var(--sp-1) 2px 0; border-bottom: 1px solid var(--edge-inst); }
  .advice td { padding: 3px var(--sp-1) 3px 0; vertical-align: baseline; border-bottom: 1px solid color-mix(in srgb, var(--edge-inst) 45%, transparent); }
  .advice .c-vis { width: 28%; }
  .advice .num { width: 17%; text-align: right; }
  .advice tr.best .opt { color: var(--ink); font-weight: 600; }
  .advice tr.best td:first-child { box-shadow: inset 2px 0 0 var(--gilt); padding-left: var(--sp-1); }
  .tag { display: block; font-size: var(--t-11); color: var(--gilt); }
  .opt { overflow-wrap: anywhere; }
  .vis { display: flex; align-items: center; gap: var(--sp-1); }
  .track { flex: 1; min-width: 0; height: 0.5rem; }
  .bar { display: block; height: 100%; min-width: 1px; border-radius: 1px; background: color-mix(in srgb, var(--ink-inst) 30%, transparent); }
  .advice tr.best .bar { background: color-mix(in srgb, var(--gilt) 70%, transparent); }
  .vn { flex: none; min-width: 2em; text-align: right; color: var(--ink-inst); }

  /* A narrow rail keeps the option and its estimate. */
  @container (max-width: 15rem) {
    .advice .c-vis { display: none; }
    .advice .num { width: 28%; }
  }
  .pick b { font-weight: 600; color: var(--ink); }
</style>
