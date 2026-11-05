<script lang="ts">
  import { onMount } from 'svelte';
  import type { EventSeat, EventSeries, EventState } from '../../protocol';
  import { Sideboard } from '../../lib/sideboard.svelte';
  import { basicsTotal, STEPPER_MAX } from '../../lib/deckbuild.svelte';
  import { BASIC_NAMES, COLOURS, factsOf, seatLabel } from '../../lib/draftcards';
  import { opponentOf, previousResult, waitingOn } from '../../lib/series';
  import type { EventApi } from '../../lib/eventapi';
  import BuildView from './BuildView.svelte';
  import './event.css';

  /**
   * The between-games panel (mockup 3e): between two games of a Bo3
   * series, it says how the last game went, who chooses play or draw for
   * the next one, whether the opponent has locked their deck, and offers a
   * compact sideboard over the seat's working copy — the deck it played
   * last game against the rest of its pool, basics +/-, the server's live
   * deck check — with Keep and Submit deck, and "Open full builder" for
   * the same working copy in the BUILD screen. In a rebuild event's window
   * before game 1 of a round-2-or-later series it is the same panel without
   * a previous game: the headline names the round and the rebuild choice.
   *
   * Everything shown is the server's: the series row (score, games,
   * chooser, waiting), the pool and the deck check. The play/draw choice
   * itself is made in the next game (its starting_player decision).
   *
   * `api` is the test seam (production uses eventApi); `onstate` adopts the
   * EventState a submit answers with; `onstale` refetches after a 409.
   */
  let {
    event,
    token,
    you,
    series,
    seats = [],
    onstate = null,
    onstale = null,
    api = undefined,
  }: {
    event: string;
    token: string;
    you: number;
    series: EventSeries;
    seats?: EventSeat[];
    onstate?: ((st: EventState) => void) | null;
    onstale?: (() => void | Promise<void>) | null;
    api?: EventApi;
  } = $props();

  // The page keys this component by (event, round, game), so binding to
  // the first values is the intent: a new game is a new working copy.
  // svelte-ignore state_referenced_locally
  const sb = new Sideboard(event, token, api);
  const b = sb.builder;
  let full = $state(false);

  onMount(() => {
    sb.onstate = (st) => onstate?.(st);
    sb.onstale = () => onstale?.();
    void sb.load();
  });

  const game = $derived(series.game ?? 0);
  const opp = $derived(opponentOf(series, you));
  const oppName = $derived(seatLabel(opp, seats[opp]?.name, you));
  const last = $derived(previousResult(series, you));
  const mineWaiting = $derived(waitingOn(series, you) && !sb.submitted);
  const oppWaiting = $derived(waitingOn(series, opp));
  const myScore = $derived(series.a === you ? series.score[0] : series.score[1]);
  const oppScore = $derived(series.a === you ? series.score[1] : series.score[0]);
  const check = $derived(b.check);
  const spells = $derived(b.deckCards.filter((c) => !factsOf(c).land));
  const deckNonBasics = $derived(b.deckCards);
  const side = $derived(b.sideCards);

  const headline = $derived(
    game === 1
      ? `Round ${series.round}: before your match, keep your deck or rebuild it from your pool`
      : `Game ${game} of ${series.best_of || 3}` +
          (last === null ? '' : last === 'drawn' ? ` — game ${game - 1} was drawn` : ` — you ${last} game ${game - 1}`),
  );

  function chooserLine(): string | null {
    const c = series.chooser;
    if (c === null || c === undefined) return null;
    if (c === you) return 'You will choose play or draw';
    return `${seatLabel(c, seats[c]?.name, you)} chooses play or draw`;
  }
</script>

{#snippet buttons()}
  <div class="btns">
    <button type="button" class="ev-btn" data-testid="sb-keep" disabled={sb.busy || !mineWaiting} onclick={() => void sb.keep(game)}>Keep</button>
    <button
      type="button"
      class="ev-btn primary"
      data-testid="sb-submit"
      disabled={sb.busy || !mineWaiting || !sb.changed || !check?.valid}
      title={sb.changed ? undefined : 'Nothing changed since last game: Keep locks that deck.'}
      onclick={() => void sb.submit(game)}
    >{sb.busy ? 'Sending…' : 'Submit deck'}</button>
  </div>
{/snippet}

<section class="bg" id="between-games" data-testid="between-games" aria-label="Between games">
  <header class="head">
    <h3>{headline}</h3>
    <span class="score data" data-testid="sb-score">You {myScore} – {oppScore} {oppName}</span>
  </header>
  <div class="facts">
    {#if chooserLine()}<p class="chooser" data-testid="sb-chooser">{chooserLine()}</p>{/if}
    <p class="data dim" data-testid="sb-opponent">{oppWaiting ? 'opponent: sideboarding…' : 'opponent ready'}</p>
  </div>

  {#if sb.notice}<p class="ev-notice" role="status">{sb.notice}</p>{/if}
  {#if sb.error}<p class="ev-error" role="alert" data-testid="sb-error">{sb.error}</p>{/if}

  {#if !mineWaiting}
    <p class="waiting" role="status" data-testid="sb-waiting">
      {oppWaiting ? `Waiting for ${oppName}…` : `Both decks are in — game ${game} starts shortly.`}
    </p>
  {:else if !b.loaded}
    <p class="dim">Loading your pool…</p>
  {:else}
    <div class="swap">
      <div class="col" aria-label="Your deck">
        <h4 class="ev-h5">Deck · {deckNonBasics.length} cards · {spells.length} spells</h4>
        <ul>
          {#each deckNonBasics as c (c.id)}
            <li><button type="button" class="card" data-id={c.id} data-zone="deck" title="Move to the sideboard" onclick={() => b.move(c.id, 'side')}>
              <span class="nm">{c.name}</span><span class="data dim">{c.mana_cost ?? ''}</span><span class="arrow">→</span>
            </button></li>
          {/each}
        </ul>
      </div>
      <div class="col" aria-label="Your sideboard">
        <h4 class="ev-h5">Sideboard · {side.length}</h4>
        <ul>
          {#each side as c (c.id)}
            <li><button type="button" class="card" data-id={c.id} data-zone="side" title="Move into the deck" onclick={() => b.move(c.id, 'deck')}>
              <span class="arrow">←</span><span class="nm">{c.name}</span><span class="data dim">{c.mana_cost ?? ''}</span>
            </button></li>
          {/each}
        </ul>
      </div>
    </div>
    <div class="lands">
      {#each BASIC_NAMES as name, i (name)}
        <span class="land">
          <span class="ev-pip {COLOURS[i]}">{COLOURS[i]}</span>
          <button type="button" aria-label={`fewer ${name}`} disabled={b.basics[i] <= 0} onclick={() => b.step(i, -1)}>−</button>
          <span class="data" data-testid={`sb-basic-${COLOURS[i]}`}>{b.basics[i]}</span>
          <button type="button" aria-label={`more ${name}`} disabled={b.basics[i] >= STEPPER_MAX} onclick={() => b.step(i, 1)}>+</button>
        </span>
      {/each}
      <span class="data dim">{basicsTotal(b.basics)} basics</span>
    </div>
    <div class="foot">
      <span class="check data" class:ok={check?.valid} data-testid="sb-check">
        {#if check}{check.total} cards · {check.valid ? 'the server accepts this deck' : (check.error ?? 'not a legal deck')}{:else}checking…{/if}
      </span>
      <span class="sp"></span>
      <button type="button" class="ev-btn sm" data-testid="sb-full" onclick={() => (full = true)}>Open full builder</button>
      {@render buttons()}
    </div>
  {/if}
</section>

{#if full}
  <div class="sheet" role="dialog" aria-modal="true" aria-label="Full deck builder" data-testid="sb-sheet">
    <div class="sheet-bar">
      <b>Your deck for game {game}</b>
      <span class="sp"></span>
      <button type="button" class="ev-btn sm" onclick={() => (full = false)}>Close</button>
    </div>
    <div class="sheet-body">
      <BuildView builder={b} ev={null} rated={false}>
        {#snippet actions()}
          {#if sb.error}<p class="ev-error" role="alert">{sb.error}</p>{/if}
          {#if mineWaiting}{@render buttons()}{:else}<p class="dim">Deck locked for game {game}.</p>{/if}
        {/snippet}
      </BuildView>
    </div>
  </div>
{/if}

<style>
  .bg {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    padding: var(--sp-3);
    background: var(--instrument);
    color: var(--ink-inst);
    border: 1px solid var(--gilt);
    border-radius: var(--radius);
    min-width: 0;
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--sp-2) var(--sp-3);
    justify-content: space-between;
  }
  h3 {
    margin: 0;
    font: 600 1.2rem var(--font-serif);
    color: var(--ink);
  }
  .score {
    color: var(--ink);
  }
  .facts {
    display: flex;
    flex-wrap: wrap;
    gap: var(--sp-1) var(--sp-4);
    align-items: baseline;
  }
  .facts p,
  .waiting {
    margin: 0;
  }
  .chooser {
    color: var(--gilt);
    font-weight: 600;
  }
  .dim {
    color: var(--ink-dim);
  }
  .waiting {
    padding: var(--sp-2) 0;
    color: var(--ink);
  }
  .swap {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--sp-2);
  }
  @media (max-width: 560px) {
    .swap {
      grid-template-columns: minmax(0, 1fr);
    }
  }
  .col {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .col ul {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 14rem;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .card {
    width: 100%;
    display: flex;
    gap: var(--sp-2);
    align-items: center;
    text-align: left;
    padding: 0.2rem 0.45rem;
    background: var(--instrument-raised);
    border: 1px solid var(--edge-inst);
    border-radius: 6px;
    color: var(--ink-inst);
    font: 500 var(--t-12) var(--font-ui);
    cursor: pointer;
  }
  .card:hover {
    border-color: var(--gilt);
    color: var(--ink);
  }
  .nm {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .arrow {
    color: var(--ink-faint);
  }
  .lands {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--sp-1) var(--sp-3);
    font-size: var(--t-12);
  }
  .land {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .land button {
    width: 1.5rem;
    height: 1.5rem;
    border-radius: 4px;
    border: 1px solid var(--edge-inst);
    background: var(--instrument-raised);
    color: var(--ink);
    cursor: pointer;
  }
  .land button[disabled] {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .foot {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--sp-2);
  }
  .check {
    font-size: var(--t-12);
    color: var(--ember);
  }
  .check.ok {
    color: var(--verdigris);
  }
  .sp {
    flex: 1;
  }
  .btns {
    display: inline-flex;
    gap: var(--sp-2);
    flex-wrap: wrap;
  }
  .sheet {
    position: fixed;
    inset: 2vh 2vw;
    z-index: 60;
    display: flex;
    flex-direction: column;
    background: var(--felt);
    border: 1px solid var(--gilt);
    border-radius: var(--radius);
    box-shadow: 0 1rem 3rem rgb(0 0 0 / 0.6);
    overflow: hidden;
  }
  .sheet-bar {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    padding: var(--sp-2) var(--sp-3);
    background: var(--instrument);
    border-bottom: 1px solid var(--edge-inst);
    color: var(--ink);
  }
  .sheet-body {
    flex: 1;
    min-height: 0;
    overflow: auto;
    display: flex;
  }
  .sheet-body :global(.ev-split) {
    flex: 1;
  }
</style>
