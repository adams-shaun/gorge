<script lang="ts">
  import { onMount } from 'svelte';
  import type { Frame } from '../../protocol';
  import { session } from '../../lib/session.svelte';
  import { EventClient } from '../../lib/draftevent.svelte';
  import { tableEventSeat } from '../../lib/eventseat';
  import { opponentOf, viewerSeries, waitingOn } from '../../lib/series';
  import { seatLabel } from '../../lib/draftcards';
  import BetweenGames from './BetweenGames.svelte';
  import { withBase } from '../../lib/basepath';

  /**
   * The table page's event layer. A series game's join link carries
   * `&event=<id>`; with a remembered event seat for that event (never the
   * URL's game token), the page watches the event's frames and — while the
   * viewer's series on this table is between games and the server still
   * waits on the viewer's deck — overlays the between-games panel above the
   * finished board. Once the viewer's deck is in, a small status line
   * replaces it; when the next game starts the series is live again and
   * both disappear, leaving the board and its prompt dock alone.
   *
   * Whether the table page shows the finished board or the match list is
   * not this component's business: the table store learns a series table's
   * between-games state from the server (TableInfo.series and the table_state
   * frame), so the board stays up between games whether or not this overlay
   * is mounted.
   *
   * A Commander draft's pod game has no series: once the server reports
   * this table's pod game ended, a chip links back to the event page
   * (standings, the next pod game or the results).
   */
  let { table }: { table: string } = $props();

  const ctx = tableEventSeat(typeof location === 'undefined' ? '' : location.search);
  const client = ctx ? new EventClient(ctx.event, ctx.seat) : null;

  onMount(() => {
    if (!client) return;
    let hellos = 0;
    const off = session.stream.onFrame((f: Frame) => {
      if (f.t === 'hello') {
        if (hellos++ > 0) void client.load();
        return;
      }
      client.handleFrame(f);
    });
    void client.load().then(() => {
      if (client.token) void session.watchEvent(client.id, client.token);
    });
    return () => {
      off();
      session.unwatchEvent(client.id);
    };
  });

  const st = $derived(client?.state ?? null);
  const you = $derived(client?.you ?? null);
  const row = $derived(st ? viewerSeries(st, table) : null);
  const between = $derived(row !== null && row.state === 'between' && you !== null);
  const waiting = $derived(between && row !== null && you !== null && waitingOn(row, you));
  const podEnded = $derived((st?.pod ?? []).some((g) => g.table === table && g.state === 'ended'));
  // The player may fold the panel away to look at the finished board.
  let folded = $state(false);
</script>

{#if client && st && podEnded}
  <p class="ev-overlay__chip" role="status" data-testid="table-pod-ended">
    The pod game is over.
    <a class="ev-btn sm" href={withBase(`/e/${encodeURIComponent(client.id)}`)}>Back to the event</a>
  </p>
{/if}
{#if client && st && row && you !== null && between}
  {#if waiting && folded}
    <p class="ev-overlay__chip" role="status" data-testid="table-between-folded">
      Between games: your deck for game {row.game}.
      <button type="button" class="ev-btn sm" onclick={() => (folded = false)}>Sideboard</button>
    </p>
  {:else if waiting}
    <div class="ev-overlay" data-testid="table-between">
      <div class="ev-overlay__panel">
        <div class="ev-overlay__bar"><button type="button" class="ev-btn sm" onclick={() => (folded = true)}>Show the board</button></div>
        {#key `${row.round}-${row.game}`}
          <BetweenGames
            event={client.id}
            token={client.token ?? ''}
            {you}
            series={row}
            seats={st.seats}
            onstate={(s) => client && (client.state = s)}
            onstale={() => client?.load()}
          />
        {/key}
      </div>
    </div>
  {:else}
    {@const opp = opponentOf(row, you)}
    <p class="ev-overlay__chip" role="status" data-testid="table-between-waiting">
      Deck locked for game {row.game}. {waitingOn(row, opp) ? `Waiting for ${seatLabel(opp, st.seats[opp]?.name, you)}…` : 'The next game starts shortly.'}
    </p>
  {/if}
{/if}

<style>
  .ev-overlay {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: grid;
    place-items: center;
    padding: 4vh 3vw;
    background: rgb(0 0 0 / 0.45);
    overflow-y: auto;
  }
  .ev-overlay__panel {
    width: min(48rem, 100%);
    max-height: 100%;
    overflow-y: auto;
  }
  .ev-overlay__bar {
    display: flex;
    justify-content: flex-end;
    margin-bottom: var(--sp-1);
  }
  .ev-overlay__chip {
    position: fixed;
    left: 50%;
    /* below the board's centred game-over banner */
    top: 6.5rem;
    transform: translateX(-50%);
    z-index: 50;
    margin: 0;
    padding: 0.4rem 0.9rem;
    border: 1px solid var(--gilt);
    border-radius: 999px;
    background: var(--instrument);
    color: var(--ink);
    font-size: var(--t-12);
    max-width: calc(100vw - 2rem);
    display: flex;
    align-items: center;
    gap: var(--sp-2);
  }
</style>
