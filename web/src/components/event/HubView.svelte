<script lang="ts">
  import type { EventSeries, EventState } from '../../protocol';
  import { withBase } from '../../lib/basepath';
  import { seatLabel } from '../../lib/draftcards';
  import { standingRows } from '../../lib/standings';
  import { countNames, pickHistory, revealDecks } from '../../lib/eventlog';
  import { gameDots, inPlay, podStatus, seriesScore, seriesStatus } from '../../lib/series';
  import { commanderFormat } from '../../lib/newevent';
  import Standings from './Standings.svelte';
  import BetweenGames from './BetweenGames.svelte';

  /**
   * The event hub (mockup 3d) for ROUND1..3, and the results page for
   * COMPLETE. Pairings, scores and standings are the server's; the viewer's
   * own live game carries the server's seat-tokenised join path, every
   * other game a spectate link to its table.
   *
   * Between two games of the viewer's own Bo3 series the row carries the
   * between-games panel — the place to sideboard when the table page
   * is gone (a restart moves the series to a new table). `token` is the
   * viewer's event seat token; `onstate` adopts a submit's EventState and
   * `onstale` refetches after a 409.
   *
   * A Commander draft has no series: each round is one free-for-all pod
   * game (EventState.pod) with every seat at one table. Its row names the
   * seats, the winner once it ends, and the viewer's own join link (the
   * server's seat-tokenised table path) while it is live.
   */
  let {
    ev,
    log = null,
    token = null,
    onstate = null,
    onstale = null,
  }: {
    ev: EventState;
    log?: unknown[] | null;
    token?: string | null;
    onstate?: ((st: EventState) => void) | null;
    onstale?: (() => void | Promise<void>) | null;
  } = $props();

  const you = $derived(ev.you ?? null);
  const complete = $derived(ev.phase === 'COMPLETE');
  const rounds = $derived.by(() => {
    const out: { round: number; series: EventSeries[] }[] = [];
    for (const s of ev.series ?? []) {
      let r = out.find((x) => x.round === s.round);
      if (!r) out.push((r = { round: s.round, series: [] }));
      r.series.push(s);
    }
    // the current round first, then the finished ones newest first, then the rest
    return out.sort((a, b) => rank(a.round) - rank(b.round));
  });
  const pod = $derived(commanderFormat(ev.format));
  const podGames = $derived([...(ev.pod ?? [])].sort((a, b) => rank(a.round) - rank(b.round)));
  const rows = $derived(standingRows(ev.standings ?? [], ev.seats ?? [], ev.series ?? [], you, pod ? (ev.pod ?? []) : null));
  const decks = $derived(complete && log ? revealDecks(log) : []);

  function rank(r: number): number {
    if (r === ev.round && !complete) return 0;
    if (r < ev.round || complete) return 10 - r;
    return 20 + r;
  }

  function label(seat: number): string {
    return seatLabel(seat, ev.seats?.[seat]?.name, you);
  }

  function roundStatus(series: EventSeries[]): string {
    if (series.every((s) => s.state === 'ended')) return 'complete';
    if (series.some(inPlay)) return 'playing';
    return 'upcoming';
  }

  // A row's game dots read from the viewer's side when it is their series,
  // else from the left-hand seat's.
  function side(s: EventSeries): number {
    return you !== null && s.b === you ? s.b : s.a;
  }

  const DOT_WORD = { w: 'won', l: 'lost', d: 'drawn' } as const;

  function outcome(s: EventSeries, seat: number): 'w' | 'l' | 'd' | '' {
    if (s.state !== 'ended') return '';
    if (s.winner === null || s.winner === undefined) return 'd';
    return s.winner === seat ? 'w' : 'l';
  }
</script>

<div class="ev-split hub">
  <section class="ev-main">
    {#if complete}
      <h2 class="title">Event complete</h2>
    {/if}
    {#if pod}
      {#if podGames.length === 0}
        <p class="dim" data-testid="pod-waiting">The pod game is being seated.</p>
      {/if}
      {#each podGames as g (g.round)}
        <h3 class="ev-h5">{podGames.length > 1 || ev.round > 1 ? `Pod game ${g.round}` : 'Pod game'} · {podStatus(g)}</h3>
        <div class="podrow" class:mine={you !== null} data-testid="pod-game" data-state={g.state}>
          <ul class="podseats">
            {#each ev.seats as s (s.seat)}
              <li class:w={g.state === 'ended' && g.winner === s.seat} class:l={g.state === 'ended' && g.winner !== null && g.winner !== s.seat}>
                <b>{label(s.seat)}</b>{#if g.state === 'ended' && g.winner === s.seat}<span class="data won">winner</span>{/if}
              </li>
            {/each}
          </ul>
          <div class="act">
            {#if g.join && g.state === 'live'}
              <a class="ev-btn primary" href={withBase(g.join)} data-testid="join-game">Join game</a>
            {:else if g.table && g.state === 'live'}
              <a class="ev-btn" href={withBase(`/t/${encodeURIComponent(g.table)}`)} data-testid="watch-game">Watch</a>
            {:else if g.table && g.state === 'ended'}
              <a class="ev-btn sm" href={withBase(`/t/${encodeURIComponent(g.table)}`)}>Replay</a>
            {:else if g.join}
              <a class="ev-btn primary" href={withBase(g.join)} data-testid="join-game">Join game</a>
            {:else}
              <span class="data dim">{g.state === 'live' ? 'starting…' : 'waiting for the table'}</span>
            {/if}
          </div>
        </div>
      {/each}
    {/if}
    {#each rounds as r (r.round)}
      <h3 class="ev-h5">Round {r.round} · {roundStatus(r.series)}</h3>
      {#each r.series as s (`${s.round}-${s.a}-${s.b}`)}
        {@const mine = you !== null && (s.a === you || s.b === you)}
        <div class="pair" class:mine data-testid="pairing">
          <div class="who"><b class={outcome(s, s.a)}>{label(s.a)}</b></div>
          <div class="mid">
            <div class="score data">{seriesScore(s)}</div>
            {#if s.best_of > 1 && (s.games?.length ?? 0) > 0}
              {@const dots = gameDots(s, side(s))}
              <div class="dots" data-testid="game-dots" aria-label={`Games from ${label(side(s))}'s side: ${dots.map((d) => DOT_WORD[d]).join(', ')}`}>
                {#each dots as d, g (g)}<i class="dot {d}" title={`Game ${g + 1}: ${DOT_WORD[d]}`}>{d.toUpperCase()}</i>{/each}
              </div>
            {/if}
            <div class="state data">{seriesStatus(s)}</div>
          </div>
          <div class="who r"><b class={outcome(s, s.b)}>{label(s.b)}</b></div>
          <div class="act">
            {#if mine && s.state === 'between' && s.game}
              <a class="ev-btn primary" href={s.join ? withBase(s.join) : '#between-games'} data-testid="your-deck">your deck for game {s.game}</a>
            {:else if s.join}
              <a class="ev-btn primary" href={withBase(s.join)} data-testid="join-game">Play your game</a>
            {:else if s.table && s.state === 'live'}
              <a class="ev-btn" href={withBase(`/t/${encodeURIComponent(s.table)}`)}>Watch</a>
            {:else if s.table && s.state === 'ended'}
              <a class="ev-btn sm" href={withBase(`/t/${encodeURIComponent(s.table)}`)}>Replay</a>
            {:else if mine && s.state === 'live'}
              <span class="data dim">starting…</span>
            {:else if s.table && s.state === 'between'}
              <a class="ev-btn sm" href={withBase(`/t/${encodeURIComponent(s.table)}`)}>Watch</a>
            {/if}
          </div>
        </div>
        {#if mine && you !== null && token && s.state === 'between' && s.game}
          {#key `${ev.event}-${s.round}-${s.game}`}
            <BetweenGames event={ev.event} {token} {you} series={s} seats={ev.seats} {onstate} {onstale} />
          {/key}
        {/if}
      {/each}
    {/each}

    {#if complete}
      <h3 class="ev-h5">Decks</h3>
      {#if decks.length === 0}
        <p class="dim">The revealed decks are loading.</p>
      {/if}
      <div class="decks">
        {#each decks as d (d.seat)}
          <details class="deck" data-testid="revealed-deck">
            <summary><b>{label(d.seat)}</b> <span class="data dim">{d.total} cards</span></summary>
            <ul>
              {#each countNames(d.cards) as c (c.name)}<li><span class="data">{c.count}</span> {c.name}</li>{/each}
              {#each d.basics as b (b.name)}<li><span class="data">{b.count}</span> {b.name}</li>{/each}
            </ul>
            {#if log}
              {@const picks = pickHistory(log, d.seat)}
              {#if picks.length}
                <details>
                  <summary class="dim">Picks</summary>
                  <ol class="picks">
                    {#each picks as p, i (i)}<li><span class="data dim">P{p.round}p{p.pickNo}</span> {p.name}{#if p.auto} <span class="dim">(auto)</span>{/if}</li>{/each}
                  </ol>
                </details>
              {/if}
            {/if}
          </details>
        {/each}
      </div>
    {/if}
  </section>

  <aside class="ev-rail">
    <Standings {rows} final={complete} {pod} />
  </aside>
</div>

<style>
  /* the standings table needs a wider rail than the draft's pod boxes */
  .hub {
    grid-template-columns: minmax(0, 1fr) 22rem;
  }
  @media (max-width: 760px) {
    .hub {
      grid-template-columns: minmax(0, 1fr);
    }
  }
  .title {
    margin: 0;
    font: 600 1.5rem var(--font-serif);
  }
  .dim {
    color: var(--ink-dim);
  }
  .pair {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr) auto;
    gap: var(--sp-3);
    align-items: center;
    padding: var(--sp-2) var(--sp-3);
    background: var(--felt-raised);
    border: 1px solid var(--edge-felt);
    border-radius: var(--radius);
  }
  .pair.mine {
    border-color: var(--gilt);
  }
  .who {
    min-width: 0;
  }
  .who b {
    font: 600 1.1rem var(--font-serif);
    color: var(--ink);
    overflow-wrap: anywhere;
  }
  .who b.w {
    color: var(--verdigris);
  }
  .who b.l {
    color: var(--ink-dim);
  }
  .who.r {
    text-align: right;
  }
  .mid {
    text-align: center;
  }
  .score {
    font-size: 1.1rem;
    color: var(--ink);
  }
  .dots {
    display: flex;
    justify-content: center;
    gap: 3px;
    margin: 2px 0;
  }
  .dot {
    display: inline-grid;
    place-items: center;
    width: 1rem;
    height: 1rem;
    border-radius: 50%;
    font: 600 0.55rem/1 var(--font-data);
    font-style: normal;
    color: var(--felt-sunk);
    background: var(--ink-faint);
  }
  .dot.w {
    background: var(--verdigris);
  }
  .dot.l {
    background: var(--ember);
  }
  .state {
    font-size: var(--t-10);
    color: var(--ink-faint);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .act {
    display: flex;
    justify-content: flex-end;
  }
  .podrow {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: var(--sp-3);
    align-items: center;
    padding: var(--sp-2) var(--sp-3);
    background: var(--felt-raised);
    border: 1px solid var(--edge-felt);
    border-radius: var(--radius);
  }
  .podrow.mine {
    border-color: var(--gilt);
  }
  .podseats {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(10rem, 100%), 1fr));
    gap: var(--sp-1) var(--sp-3);
  }
  .podseats li {
    display: flex;
    align-items: baseline;
    gap: var(--sp-1);
    min-width: 0;
  }
  .podseats b {
    font: 600 1rem var(--font-serif);
    color: var(--ink);
    overflow-wrap: anywhere;
  }
  .podseats li.w b {
    color: var(--verdigris);
  }
  .podseats li.l b {
    color: var(--ink-dim);
  }
  .won {
    font-size: var(--t-10);
    color: var(--verdigris);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  @media (max-width: 520px) {
    .podrow {
      grid-template-columns: minmax(0, 1fr);
    }
  }
  /* On a phone the action drops under the pairing instead of squeezing it. */
  @media (max-width: 520px) {
    .pair {
      grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
    }
    .act {
      grid-column: 1 / -1;
      justify-content: center;
    }
  }
  .decks {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(16rem, 100%), 1fr));
    gap: var(--sp-2);
  }
  .deck {
    background: var(--felt-raised);
    border: 1px solid var(--edge-felt);
    border-radius: var(--radius);
    padding: var(--sp-2) var(--sp-3);
    font-size: var(--t-12);
  }
  .deck summary {
    cursor: pointer;
  }
  .deck ul,
  .deck ol {
    margin: var(--sp-2) 0 0;
    padding-left: 0;
    list-style: none;
  }
  .picks li {
    padding: 1px 0;
  }
</style>
