<script lang="ts">
  import type { Advice, Decision, PairedBotStatus } from '../protocol';
  import { fetchAdvice, fetchPairedBotStatus } from '../lib/api';
  import { loadPairedBotOn, pairedRowLabel, savePairedBotOn } from '../lib/pairedbot';
  import type { ExplainNote } from '../lib/review';
  import { signed, sourceBadge } from '../lib/review';
  import type { SeatCtx } from '../lib/seat';
  import SearchTable from './SearchTable.svelte';

  /**
   * PairedBotPanel shows a seated player what a bot makes of the decision in
   * front of them. The server computes it from what the seat can see; nothing
   * here can answer the decision. It renders nothing on a server without the
   * option, and asks only while the player's toggle is on.
   */
  let { table, match, ctx, decision }: { table: string; match: number; ctx: SeatCtx; decision: Decision | null } = $props();

  /** How long a decision must stay in front of the player before the bot is asked: an auto-passed one never costs a search. */
  const SETTLE_MS = 200;

  let status = $state<PairedBotStatus | null>(null);
  let on = $state(loadPairedBotOn());
  let advice = $state<Advice | null>(null);
  let failed = $state<string | null>(null);
  let attempt = $state(0);

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
    const hangup = new AbortController();
    const timer = setTimeout(() => {
      fetchAdvice(table, match, s, ctx, hangup.signal)
        .then((a) => {
          if (!hangup.signal.aborted && a.ask_seq === s) advice = a;
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

  function toggle() {
    on = !on;
    savePairedBotOn(on);
  }

  const shown = $derived(advice !== null && advice.ask_seq === seq ? advice : null);
  const note = $derived(shown?.state === 'ready' ? (shown.explain as ExplainNote) : null);
  const badge = $derived(note ? sourceBadge(note) : null);
  const pick = $derived(
    note?.bot && decision && note.bot.length > 0 ? pairedRowLabel(decision, { choices: note.bot, visits: 0, prior: 0, q: 0 }) : null,
  );
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
      {#if on}
        <p class="quiet">Advice only, from a bot that can't see hidden cards. It guesses your opponent's cards from a set of decks. You still answer.</p>
        {#if seq === null}
          <p class="quiet">Nothing to evaluate until your next decision.</p>
        {:else if failed}
          <p class="refused">The bot did not answer: {failed}</p>
          <button type="button" class="again" onclick={() => attempt++}>Ask again</button>
        {:else if !shown}
          <p class="quiet" role="status">The bot is thinking…</p>
        {:else if shown.state !== 'ready'}
          <p class="refused">{shown.reason}</p>
        {:else if note && decision}
          {#if note.search}
            <SearchTable search={note.search} chosenTag="its pick" botTag="" label={(r) => pairedRowLabel(decision, r)} />
          {:else}
            <p class="quiet" title={badge?.tip}>The bot answers this one by its rules, without a search.</p>
            {#if pick}<p class="pick">It would pick <b>{pick}</b></p>{/if}
          {/if}
          {#if note.net}<p class="quiet">Net value {signed(note.net.value)}</p>{/if}
          {#if note.search && note.reason}<p class="quiet">{note.reason === 'deadline' ? 'The search ran out of time; these are the simulations it finished.' : note.reason}</p>{/if}
        {/if}
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
  .pick b { font-weight: 600; color: var(--ink); }
</style>
