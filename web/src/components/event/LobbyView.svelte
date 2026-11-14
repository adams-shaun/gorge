<script lang="ts">
  import type { EventState } from '../../protocol';
  import { seatLabel } from '../../lib/draftcards';
  import { commanderFormat, timerLabel } from '../../lib/newevent';

  /**
   * LOBBY: the pod waits for every human seat to type a display name. The
   * viewer's own unjoined seat gets the name form; the server validates the
   * name (1–32 printable characters) and opens the packs the moment the last
   * human joins.
   */
  let {
    ev,
    canJoin,
    busy = false,
    error = null,
    name = $bindable(''),
    onjoin,
  }: {
    ev: EventState;
    canJoin: boolean;
    busy?: boolean;
    error?: string | null;
    name?: string;
    onjoin: (name: string) => void;
  } = $props();

  const you = $derived(ev.you ?? null);
  const picks = $derived(ev.picks_per_pass ?? 1);
  // The summary's middle: how cards are taken and what the rounds are.
  const shape = $derived(
    commanderFormat(ev.format)
      ? `Commander draft · ${ev.seats.length} seats · ${picks === 1 ? 'pick one' : `take ${picks} from each pack`} · ${ev.deck_size ? `${ev.deck_size}-card decks` : 'decks'} around drafted commanders · one free-for-all`
      : `draft · ${ev.seats.length} seats · pick one · best of ${ev.best_of === 3 ? 3 : 1}`,
  );

  function submit(e: SubmitEvent) {
    e.preventDefault();
    if (name.trim() !== '') onjoin(name.trim());
  }
</script>

<section class="lobby ev-main">
  <h2>Waiting for the pod</h2>
  <p class="dim" data-testid="lobby-summary">{ev.set} {shape} · {timerLabel(ev.pick_timer)}. The packs open once every human seat has joined.</p>
  {#if canJoin}
    <form class="join" onsubmit={submit} data-testid="join-form">
      <label>
        <span class="ev-h5">Your name</span>
        <input type="text" bind:value={name} maxlength="32" autocomplete="nickname" required aria-label="Your name" />
      </label>
      <button type="submit" class="ev-btn primary" disabled={busy || name.trim() === ''}>{busy ? 'Joining…' : 'Join the pod'}</button>
    </form>
  {/if}
  {#if error}<p class="ev-error" role="alert">{error}</p>{/if}
  <ul class="seats">
    {#each ev.seats as s (s.seat)}
      <li class:me={s.seat === you}>
        <span class="data idx">{s.seat + 1}</span>
        <b>{s.kind === 'human' && !s.joined ? (s.seat === you ? 'You' : 'Open human seat') : seatLabel(s.seat, s.name, you)}</b>
        <span class="data dim">{s.kind === 'bot' ? `bot · plays ${s.policy ?? 'default'}` : s.joined ? 'joined' : 'waiting to join'}</span>
      </li>
    {/each}
  </ul>
</section>

<style>
  .lobby {
    max-width: 40rem;
  }
  h2 {
    margin: 0;
    font: 600 1.5rem var(--font-serif);
  }
  .dim {
    color: var(--ink-dim);
  }
  p {
    margin: 0;
  }
  .join {
    display: flex;
    align-items: flex-end;
    gap: var(--sp-2);
    flex-wrap: wrap;
  }
  .join label {
    display: flex;
    flex-direction: column;
    gap: var(--sp-1);
    flex: 1 1 12rem;
  }
  .seats {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .seats li {
    display: grid;
    grid-template-columns: 1.5rem minmax(0, 1fr) auto;
    gap: var(--sp-2);
    align-items: center;
    padding: 0.45rem 0.6rem;
    background: var(--felt-raised);
    border: 1px solid var(--edge-felt);
    border-radius: var(--radius);
    font-size: var(--t-14);
  }
  .seats li.me {
    border-color: var(--gilt);
  }
  .seats b {
    overflow-wrap: anywhere;
  }
  .idx {
    color: var(--ink-faint);
  }
</style>
