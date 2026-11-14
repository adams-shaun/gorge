<script lang="ts">
  import { onMount } from 'svelte';
  import '../components/event/event.css';
  import { session } from '../lib/session.svelte';
  import { EventClient, isRound } from '../lib/draftevent.svelte';
  import { DeckBuilder } from '../lib/deckbuild.svelte';
  import { DraftAssist } from '../lib/draftassist.svelte';
  import { eventSeatFor, forgetEventSeat, rememberName, rememberedName } from '../lib/eventseat';
  import { eventApi, listEventSets } from '../lib/eventapi';
  import { commanderFormat, ratedBy17Lands, sealedFormat } from '../lib/newevent';
  import { seatLabel } from '../lib/draftcards';
  import PhaseRail from '../components/event/PhaseRail.svelte';
  import LobbyView from '../components/event/LobbyView.svelte';
  import DraftView from '../components/event/DraftView.svelte';
  import BuildView from '../components/event/BuildView.svelte';
  import HubView from '../components/event/HubView.svelte';
  import PodStatus from '../components/event/PodStatus.svelte';
  import type { EventSetInfo, Frame } from '../protocol';

  /**
   * /e/{id}: one draft event, whatever its phase. The viewer's seat comes
   * from the invite link's ?seat=&token= (or the one remembered for this
   * event); the SERVER decides which seat that token is (EventState.you).
   * Live updates arrive as event_state / draft_pack / pod_progress /
   * series_update frames once the page's stream is subscribed to the event;
   * every reconnect (a new hello) refetches, so a missed frame never
   * strands the page.
   */
  let { event }: { event: string } = $props();

  // App keys this page by event id, so binding to the first value is the intent.
  // svelte-ignore state_referenced_locally
  const seat = eventSeatFor(event, typeof location === 'undefined' ? '' : location.search);
  // svelte-ignore state_referenced_locally
  const client = new EventClient(event, seat);
  // The set catalogue says whether this set's bot ratings are 17Lands' (the
  // CC BY credit) or the card heuristic; until it loads the credit stays.
  let sets = $state<EventSetInfo[] | null>(null);
  let builder = $state<DeckBuilder | null>(null);
  let log = $state<unknown[] | null>(null);
  let name = $state(rememberedName());
  // The bot-assist switch (off by default), shared by the draft and build screens.
  const assist = new DraftAssist();

  const st = $derived(client.state);
  const you = $derived(client.you);
  const phase = $derived(client.phase);
  const rated = $derived(ratedBy17Lands(sets, st?.set));

  onMount(() => {
    let hellos = 0;
    const off = session.stream.onFrame((f: Frame) => {
      if (f.t === 'hello') {
        // The first hello is this page's own connect; any later one is a
        // reconnect the server could not resume: refetch what may be missed.
        if (hellos++ > 0) {
          void client.load();
          void builder?.recheck();
        }
        return;
      }
      client.handleFrame(f);
    });
    listEventSets()
      .then((l) => (sets = l ?? []))
      .catch(() => {});
    void client.load().then(() => {
      if (client.unauthorised) forgetEventSeat(event);
      void session.watchEvent(event, client.token);
    });
    return () => {
      off();
      session.unwatchEvent(event);
    };
  });

  // BUILD: one working copy per page, created when the seat first reaches it.
  $effect(() => {
    if (phase === 'BUILD' && you !== null && client.token && !builder) {
      const b = new DeckBuilder(event, client.token);
      b.commanderMode = commanderFormat(client.state?.format);
      b.onstate = (s) => (client.state = s);
      builder = b;
      void b.load();
    }
  });

  // COMPLETE: the server reveals every seat's packs, picks and decks.
  $effect(() => {
    if (phase === 'COMPLETE' && log === null) {
      log = [];
      eventApi
        .log(event, client.token)
        .then((l) => (log = l.entries ?? []))
        .catch(() => (log = []));
    }
  });

  async function join(n: string) {
    if (await client.join(n)) rememberName(n);
  }

  const commander = $derived(commanderFormat(st?.format));
  // A Commander draft's rounds are pod games, one per round (EventState.pod).
  const podGames = $derived(Math.max(st?.pod?.length ?? 0, st?.round ?? 0, 1));
  const phaseLabel = $derived(
    phase === 'LOBBY'
      ? 'lobby'
      : phase === 'DRAFT'
        ? sealedFormat(st?.format)
          ? 'opening packs'
          : 'draft'
        : phase === 'BUILD'
          ? 'build'
          : isRound(phase) && commander
            ? podGames > 1
              ? `pod game ${st?.round ?? ''} of ${podGames}`
              : 'pod game'
          : isRound(phase)
            ? `round ${st?.round ?? ''} of 3 · Bo${st?.best_of === 3 ? 3 : 1}`
            : phase === 'COMPLETE'
              ? 'results'
              : '',
  );
</script>

<main class="ev" data-testid="event-page">
  <header class="ev-topbar">
    <a class="brand" href="/">gorge</a>
    <span class="crumbs">
      {st ? `${st.set} ${commander ? 'Commander draft' : 'draft'} · ${event} · ${phaseLabel}` : `event ${event}`}
      {#if you !== null && st} · seat {you + 1} of {st.seats.length}{:else if st} · spectating{/if}
    </span>
    <span class="sp"></span>
    <a class="ev-btn sm" href="/events/new">New event</a>
  </header>

  {#if client.missing}
    <section class="ev-main"><h2>No such event</h2><p><a href="/">Back to the lobby</a></p></section>
  {:else if !st}
    <section class="ev-main">
      {#if client.error}<p class="ev-error" role="alert">{client.error}</p>{:else}<p class="dim">Loading the event…</p>{/if}
    </section>
  {:else}
    <PhaseRail phase={st.phase} format={st.format} games={commander ? podGames : 0} />
    {#if client.unauthorised}
      <p class="ev-notice banner" role="status">This link's seat token is not valid for this event, so you are watching as a spectator.</p>
    {/if}
    {#if st.error}<p class="ev-error banner" role="alert">{st.error}</p>{/if}

    {#if st.phase === 'LOBBY'}
      <LobbyView ev={st} canJoin={you !== null && !client.joined} busy={client.busy} error={client.error} bind:name onjoin={join} />
    {:else if st.phase === 'DRAFT'}
      {#if you !== null}
        <DraftView {client} {rated} {assist} />
      {:else}
        <section class="ev-main">
          <h2>Drafting</h2>
          <p class="dim">Packs and picks stay hidden until the event is complete.</p>
          <PodStatus seats={st.seats} you={null} progress={client.progress} />
        </section>
      {/if}
    {:else if st.phase === 'BUILD'}
      {#if you !== null && builder}
        <BuildView {builder} ev={st} {rated} {assist} />
      {:else}
        <section class="ev-main">
          <h2>Building decks</h2>
          <ul class="ready">
            {#each st.seats as s (s.seat)}
              <li><span>{seatLabel(s.seat, s.name, you)}</span><span class={s.ready ? 'ok' : 'dim'}>{s.ready ? 'ready' : 'building…'}</span></li>
            {/each}
          </ul>
        </section>
      {/if}
    {:else}
      <HubView ev={st} {log} token={client.token} onstate={(s) => (client.state = s)} onstale={() => client.load()} />
    {/if}
  {/if}
</main>

<style>
  h2 {
    margin: 0;
    font: 600 1.5rem var(--font-serif);
  }
  .dim {
    color: var(--ink-dim);
  }
  .ok {
    color: var(--verdigris);
  }
  .banner {
    margin: var(--sp-2) var(--sp-4) 0;
  }
  .ready {
    list-style: none;
    padding: 0;
    margin: 0;
    max-width: 28rem;
  }
  .ready li {
    display: flex;
    justify-content: space-between;
    gap: var(--sp-2);
    padding: 0.3rem 0;
    border-bottom: 1px solid var(--edge-felt);
  }
</style>
