<script lang="ts">
  import { onMount } from 'svelte';
  import '../components/event/event.css';
  import type { BotPolicyInfo, EventInvite, EventSetInfo } from '../protocol';
  import { fetchBotPolicies } from '../lib/api';
  import { createEvent, listEventPersonas, listEventSets } from '../lib/eventapi';
  import { navigateTo } from '../lib/router';
  import { message } from '../lib/draftevent.svelte';
  import {
    DEFAULT_PERSONA,
    FORMATS,
    PICK_TIMERS,
    RATINGS_17LANDS,
    commanderLabel,
    commanderSets,
    createRequest,
    defaultBestOf,
    defaultRebuild,
    defaultSeats,
    eventPolicies,
    humanCount,
    inviteURL,
    maskToken,
    sealedFormat,
    seriesFormat,
    setNote,
    timerOffered,
    type EventFormat,
    type PickTimer,
    type SeatKind,
  } from '../lib/newevent';
  import Attribution from '../components/event/Attribution.svelte';

  /**
   * /events/new (mockup 3a): a 4-player pod, pick one. Each seat is a bot
   * (with its game policy and pickbot persona) or a human who gets an
   * invite link; seat 1 starts as you. The Mode segment picks the supply —
   * a booster draft, a sealed pool, or a prerelease pool plus one promo —
   * and each format resets Matches and Deck changes to its server default.
   * A sealed format has no picks, so it hides the pick timer and names the
   * persona a builder. A Commander draft is offered only when the server
   * reports a set it hosts one for (EventSetInfo.commander): it narrows the
   * set list to those sets, and drops Matches and Deck changes (its rounds
   * are free-for-all pod games) and the bot drafter choice (the server seats
   * its Commander-draft drafter). Everything offered is the server's: the sets
   * (GET /api/events/sets), the pickbot personas (GET /api/events/personas)
   * and the bot game policies (/api/bot-policies, which never lists a
   * policy that reads opponents' decklists).
   *
   * With one human, creating the event sends you to your own invite link.
   * With more, the server's invite links are shown once (it keeps only
   * their hashes), in an Invites panel with a Copy button per seat.
   *
   * sets0 and personas0 seed the two catalogues for the SSR test harness,
   * where onMount never runs; production passes neither and fetches both.
   */
  let { sets0 = null, personas0 = null }: { sets0?: EventSetInfo[] | null; personas0?: string[] | null } = $props();

  // svelte-ignore state_referenced_locally
  let sets = $state<EventSetInfo[] | null>(sets0);
  // svelte-ignore state_referenced_locally
  let personas = $state<string[]>(personas0 ?? []);
  let setsError = $state<string | null>(null);
  // svelte-ignore state_referenced_locally
  let set = $state(sets0?.[0]?.code ?? '');
  let policies = $state<BotPolicyInfo[]>([]);
  let defaultPolicy = $state('');
  let seats = $state(defaultSeats());
  let bestOf = $state<1 | 3>(1);
  let format = $state<EventFormat>('draft');
  let rebuild = $state(false);
  let pickTimer = $state<PickTimer>('');
  let busy = $state(false);
  let error = $state<string | null>(null);
  /** invites is the create reply's links, kept only while this page shows them (never stored). */
  let invites = $state<EventInvite[] | null>(null);
  let copied = $state<number | null>(null);
  let revealed = $state<number[]>([]);

  const offered = $derived(eventPolicies(policies, format));
  const cmdSets = $derived(commanderSets(sets));
  const commander = $derived(format === 'commander');
  const series = $derived(seriesFormat(format));
  // A Commander draft drafts only the sets the server hosts one for.
  const shownSets = $derived(commander ? cmdSets : sets);
  const modes = $derived(cmdSets.length > 0 ? [...FORMATS, { format: 'commander' as const, label: commanderLabel(sets) }] : FORMATS);
  const chosen = $derived(sets?.find((s) => s.code === set) ?? null);
  const defaultLabel = $derived(policies.find((p) => p.name === defaultPolicy)?.label ?? defaultPolicy);
  const humans = $derived(humanCount(seats));
  const timed = $derived(timerOffered(seats));
  const sealed = $derived(sealedFormat(format));
  // The segment shown as on: Off when the server will not time this pod;
  // otherwise the player's choice, else the server's default (standard).
  const timerShown = $derived(!timed ? 'off' : pickTimer || 'standard');

  onMount(() => {
    fetchBotPolicies()
      .then((l) => {
        policies = l.policies ?? [];
        defaultPolicy = l.default;
      })
      .catch(() => (policies = []));
    // The lobby's Commander-draft button arrives with ?mode=commander.
    const wantCommander = typeof location !== 'undefined' && new URLSearchParams(location.search).get('mode') === 'commander';
    listEventSets()
      .then((l) => {
        sets = l ?? [];
        if (!sets.some((s) => s.code === set)) set = sets[0]?.code ?? '';
        if (wantCommander && commanderSets(sets).length > 0) chooseFormat('commander');
      })
      .catch((e) => {
        sets = [];
        setsError = message(e);
      });
    listEventPersonas()
      .then((p) => (personas = p.drafters ?? []))
      .catch(() => (personas = []));
  });

  function policyLabel(p: BotPolicyInfo): string {
    return p.tier && p.tier !== 'production' ? `${p.label} (${p.tier})` : p.label;
  }

  function setKind(i: number, kind: SeatKind) {
    seats[i].kind = kind;
  }

  // chooseFormat applies the server's defaults for the chosen supply; the
  // player can still change Matches and Deck changes afterwards.
  function chooseFormat(f: EventFormat) {
    format = f;
    bestOf = defaultBestOf(f);
    rebuild = defaultRebuild(f);
    if (f === 'commander' && !cmdSets.some((s) => s.code === set)) set = cmdSets[0]?.code ?? '';
    // A bot's game policy must play the new format's games.
    const plays = new Set(eventPolicies(policies, f).map((p) => p.name));
    for (const seat of seats) if (seat.policy && !plays.has(seat.policy)) seat.policy = '';
  }

  function seatTitle(i: number): string {
    if (seats[i].kind === 'bot') return 'Bot';
    return seats.findIndex((s) => s.kind === 'human') === i ? 'You' : 'Human';
  }

  async function open() {
    if (!set) return;
    busy = true;
    error = null;
    try {
      const res = await createEvent(createRequest(set, seats, { bestOf, pickTimer, format, rebuild }));
      const links = res.invites ?? [];
      if (links.length > 1) {
        invites = links;
        busy = false;
        return;
      }
      const mine = links[0];
      navigateTo(mine ? mine.join : `/e/${encodeURIComponent(res.event)}`);
    } catch (e) {
      error = message(e);
      busy = false;
    }
  }

  function link(inv: EventInvite): string {
    return inviteURL(typeof location === 'undefined' ? '' : location.origin, inv.join);
  }

  // copy puts one invite link on the clipboard. Where the Clipboard API is
  // missing or refused (an insecure origin), it falls back to a selected
  // off-screen textarea; failing both, the link is revealed for a manual copy.
  async function copy(inv: EventInvite) {
    const url = link(inv);
    try {
      await navigator.clipboard.writeText(url);
      copied = inv.seat;
      return;
    } catch {
      /* fall through */
    }
    const ta = document.createElement('textarea');
    ta.value = url;
    ta.setAttribute('readonly', '');
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.append(ta);
    ta.select();
    let ok: boolean;
    try {
      ok = document.execCommand('copy');
    } catch {
      ok = false;
    }
    ta.remove();
    if (ok) copied = inv.seat;
    else reveal(inv.seat);
  }

  function reveal(seat: number) {
    if (!revealed.includes(seat)) revealed = [...revealed, seat];
  }
</script>

<main class="ev" data-testid="new-event">
  <header class="ev-topbar">
    <a class="brand" href="/">gorge</a>
    <span class="crumbs">Events / New</span>
    <span class="sp"></span>
    <a class="ev-btn sm" href="/">{invites ? 'Done' : 'Cancel'}</a>
  </header>
  {#if invites}
    <section class="ev-main invites" data-testid="invites">
      <h2>Invites</h2>
      <p class="warn" role="note">These links are shown once. Copy them now.</p>
      <p class="dim small">Each human seat joins with its own link. The server keeps no copy it can show again.</p>
      <ul class="invlist">
        {#each invites as inv, j (inv.seat)}
          {@const url = link(inv)}
          <li class="invrow" data-testid="invite-row">
            <span class="idx data">{inv.seat + 1}</span>
            <b>{j === 0 ? 'You' : `Seat ${inv.seat + 1}`}</b>
            {#if revealed.includes(inv.seat)}
              <input class="link data" type="text" readonly value={url} aria-label={`Seat ${inv.seat + 1} invite link`} onfocus={(e) => e.currentTarget.select()} />
            {:else}
              <span class="link data dim" title="The seat token is hidden; Copy copies the full link.">{maskToken(url)}</span>
            {/if}
            <span class="btns">
              <button type="button" class="ev-btn sm" data-testid="copy-invite" onclick={() => void copy(inv)}>{copied === inv.seat ? 'Copied ✓' : 'Copy'}</button>
              {#if !revealed.includes(inv.seat)}
                <button type="button" class="ev-btn sm" onclick={() => reveal(inv.seat)}>Show</button>
              {/if}
            </span>
          </li>
        {/each}
      </ul>
      <div>
        <button type="button" class="ev-btn primary" data-testid="go-to-seat" onclick={() => invites && navigateTo(invites[0].join)}>Go to my seat</button>
      </div>
    </section>
  {:else}
  <div class="ev-split create">
    <section class="ev-main form">
      <div class="field">
        <span class="ev-h5">Mode</span>
        <div class="seg" role="group" aria-label="Mode">
          {#each modes as f (f.format)}
            <button
              type="button"
              class:on={format === f.format}
              aria-pressed={format === f.format}
              data-testid={`format-${f.format}`}
              onclick={() => chooseFormat(f.format)}
            >{f.label}</button>
          {/each}
        </div>
        {#if format === 'sealed'}
          <p class="dim small">Six Play Boosters per player, opened straight into your pool. Build a 40-card deck with any number of basic lands.</p>
        {:else if format === 'prerelease'}
          <p class="dim small">Six Play Boosters and one rare or mythic promo per player, opened straight into your pool. Build a 40-card deck with any number of basic lands.</p>
        {:else if commander}
          <p class="dim small" data-testid="commander-note">
            Draft {chosen && chosen.pack_size > 0 ? `${chosen.pack_size}-card packs` : 'packs'}, taking more than one card from each pack before passing it.
            Build a deck around one drafted commander, or two that can partner, with every card inside their colour identity and free basic lands.
            The whole pod then plays one Commander free-for-all.
          </p>
        {/if}
      </div>
      <div class="field">
        <span class="ev-h5">Seats · pod of 4 · {humans} {humans === 1 ? 'human' : 'humans'}</span>
        <div class="seats">
          {#each seats as seat, i (i)}
            <div class="seatrow" class:human={seat.kind === 'human'} data-testid={`seat-${i + 1}`}>
              <span class="idx data">{i + 1}</span>
              <div class="seg kindseg" role="group" aria-label={`Seat ${i + 1} kind`}>
                <button type="button" class:on={seat.kind === 'human'} aria-pressed={seat.kind === 'human'} data-testid={`seat-${i + 1}-human`} onclick={() => setKind(i, 'human')}>{i === 0 ? 'You' : 'Invite human'}</button>
                <button type="button" class:on={seat.kind === 'bot'} aria-pressed={seat.kind === 'bot'} data-testid={`seat-${i + 1}-bot`} onclick={() => setKind(i, 'bot')}>Bot</button>
              </div>
              {#if seat.kind === 'human'}
                <span class="data dim humannote">
                  {seatTitle(i) === 'You' ? 'human · you type your name next' : 'human · gets an invite link once the pod opens'}
                </span>
              {:else if commander}
                <span class="data dim humannote">bot · drafts and builds as the server's Commander drafter</span>
              {:else}
                <span class="persona">
                  <select aria-label={`Seat ${i + 1} ${sealed ? 'builds' : 'drafts'} as`} bind:value={seat.drafter} data-testid={`seat-${i + 1}-drafter`}>
                    <option value="">{sealed ? 'Builder' : 'Drafter'}: server default ({DEFAULT_PERSONA})</option>
                    {#each personas as name (name)}
                      <option value={name}>{sealed ? 'Builder' : 'Drafter'}: {name}</option>
                    {/each}
                  </select>
                  <span class="data dim">Builder: {seat.drafter || DEFAULT_PERSONA}</span>
                </span>
              {/if}
              {#if seat.kind === 'bot'}
                <label class="plays">
                  <span class="sr">Seat {i + 1} plays</span>
                  <select aria-label={`Seat ${i + 1} plays`} bind:value={seat.policy} data-testid={`seat-${i + 1}-policy`}>
                    <option value="">{defaultLabel ? `Plays: server default (${defaultLabel})` : 'Plays: server default'}</option>
                    {#each offered as p (p.name)}
                      <option value={p.name}>Plays: {policyLabel(p)}</option>
                    {/each}
                  </select>
                </label>
              {/if}
            </div>
          {/each}
        </div>
        <p class="dim small">Only bots that cannot see your decklist are offered. Every human seat after yours gets its own invite link.</p>
      </div>
      {#if series}
      <div class="field">
        <span class="ev-h5">Matches</span>
        <div class="seg" role="group" aria-label="Matches">
          <button type="button" class:on={bestOf === 1} aria-pressed={bestOf === 1} data-testid="best-of-1" onclick={() => (bestOf = 1)}>Best of 1</button>
          <button type="button" class:on={bestOf === 3} aria-pressed={bestOf === 3} data-testid="best-of-3" onclick={() => (bestOf = 3)}>Best of 3</button>
        </div>
        <p class="dim small">
          Round robin: three rounds, everyone plays everyone once{bestOf === 3 ? ', each pairing a best-of-three series with sideboarding between games' : ''}.
        </p>
      </div>
      <div class="field">
        <span class="ev-h5">Deck changes between rounds</span>
        <div class="seg" role="group" aria-label="Deck changes between rounds">
          <button type="button" class:on={rebuild} aria-pressed={rebuild} data-testid="rebuild-on" onclick={() => (rebuild = true)}>On</button>
          <button type="button" class:on={!rebuild} aria-pressed={!rebuild} data-testid="rebuild-off" onclick={() => (rebuild = false)}>Off</button>
        </div>
        <p class="dim small">
          Between rounds, keep your deck or rebuild it from your pool before each match (Magic Tournament Rules 7.3). Bots keep theirs.
        </p>
      </div>
      {/if}
      {#if !sealed}
        <div class="field">
          <span class="ev-h5">Pick timer</span>
          <div class="seg" role="group" aria-label="Pick timer" data-testid="pick-timer">
            {#each PICK_TIMERS as t (t.mode)}
              <button
                type="button"
                class:on={timerShown === t.mode}
                aria-pressed={timerShown === t.mode}
                disabled={!timed}
                data-testid={`timer-${t.mode}`}
                onclick={() => (pickTimer = t.mode)}
              >{t.label}</button>
            {/each}
          </div>
          <p class="dim small">
            {timed
              ? 'Each pick has a time limit (shorter as the pack empties); the server picks for a seat that runs out.'
              : 'A pod with one human drafts without a timer.'}
          </p>
        </div>
      {/if}
    </section>
    <aside class="ev-rail">
      <span class="ev-h5">Set</span>
      <div class="setlist" role="radiogroup" aria-label="Set" data-testid="set-list">
        {#if sets === null}
          <p class="dim small">Loading the sets…</p>
        {:else}
          {#each shownSets ?? [] as s (s.code)}
            <label class="setopt" class:on={set === s.code}>
              <input type="radio" name="event-set" value={s.code} bind:group={set} />
              <span class="code data">{s.code}</span>
              <span>{s.name || s.code}<small>{setNote(s)}</small></span>
            </label>
          {:else}
            <p class="dim small" role="status">{setsError ? `Could not load the sets: ${setsError}` : 'This server offers no draftable sets.'}</p>
          {/each}
        {/if}
      </div>
      <p class="dim small">Only sets where every booster card is fully supported are offered.</p>
      {#if chosen?.ratings === RATINGS_17LANDS}<Attribution what="Bot pick ratings" />{/if}
      <span class="sp"></span>
      {#if error}<p class="ev-error" role="alert">{error}</p>{/if}
      <button type="button" class="ev-btn primary" data-testid="open-pod" disabled={busy || !set} onclick={open}>{busy ? 'Opening…' : 'Open the pod'}</button>
    </aside>
  </div>
  {/if}
</main>

<style>
  .dim {
    color: var(--ink-dim);
  }
  .small {
    font-size: var(--t-12);
    margin: 0;
  }
  .form {
    gap: var(--sp-4);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
  }
  .seg {
    display: inline-flex;
    flex-wrap: wrap;
    align-self: flex-start;
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    overflow: hidden;
  }
  .seg button {
    background: var(--instrument);
    border: 0;
    border-right: 1px solid var(--edge-inst);
    color: var(--ink-dim);
    padding: 0.4rem 0.8rem;
    font: 500 var(--t-14) var(--font-ui);
  }
  .seg button:last-child {
    border-right: 0;
  }
  .seg button.on {
    background: var(--felt-raised);
    color: var(--ink);
    box-shadow: inset 0 -2px 0 var(--gilt);
  }
  .seg button[disabled] {
    color: var(--ink-faint);
    cursor: not-allowed;
  }
  .seats {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .seatrow {
    display: grid;
    grid-template-columns: auto auto minmax(0, 1fr) minmax(0, 15rem);
    gap: var(--sp-2);
    align-items: center;
    padding: 0.45rem 0.6rem;
    background: var(--felt-raised);
    border: 1px solid var(--edge-felt);
    border-radius: var(--radius);
    font-size: var(--t-14);
  }
  .seatrow.human {
    grid-template-columns: auto auto minmax(0, 1fr);
  }
  .kindseg button {
    padding: 0.25rem 0.55rem;
    font-size: var(--t-12);
  }
  .humannote {
    font-size: var(--t-12);
  }
  .persona {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    font-size: var(--t-11);
  }
  .persona select {
    width: 100%;
  }
  .plays select {
    width: 100%;
  }
  .idx {
    color: var(--ink-faint);
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
  }
  /* On a phone a bot row stacks: number and kind, then persona, then its select. */
  @media (max-width: 600px) {
    .seatrow {
      grid-template-columns: 1.5rem minmax(0, 1fr);
    }
    .seatrow.human {
      grid-template-columns: 1.5rem minmax(0, 1fr);
    }
    .persona,
    .plays,
    .humannote {
      grid-column: 2;
    }
  }
  .setlist {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .setopt {
    display: grid;
    grid-template-columns: auto 2.75rem minmax(0, 1fr);
    gap: var(--sp-2);
    align-items: center;
    padding: 0.5rem 0.6rem;
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    background: var(--instrument);
    cursor: pointer;
    font-size: var(--t-14);
  }
  .setopt input {
    accent-color: var(--gilt);
    margin: 0;
  }
  .setopt.on {
    border-color: var(--gilt);
    background: var(--instrument-raised);
  }
  .setopt small {
    display: block;
    color: var(--ink-dim);
    font-size: var(--t-11);
  }
  .code {
    color: var(--ink);
  }
  .sp {
    flex: 1;
  }
  .invites {
    max-width: 52rem;
  }
  .invites h2 {
    margin: 0;
    font: 600 1.5rem var(--font-serif);
  }
  .warn {
    margin: 0;
    padding: var(--sp-2) var(--sp-3);
    border: 1px solid var(--gilt);
    border-radius: var(--radius);
    background: color-mix(in srgb, var(--gilt) 12%, transparent);
    color: var(--ink);
    font-weight: 600;
  }
  .invlist {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .invrow {
    display: grid;
    grid-template-columns: 1.5rem 4rem minmax(0, 1fr) auto;
    gap: var(--sp-2);
    align-items: center;
    padding: 0.45rem 0.6rem;
    background: var(--felt-raised);
    border: 1px solid var(--edge-felt);
    border-radius: var(--radius);
    font-size: var(--t-14);
  }
  .invrow .link {
    min-width: 0;
    overflow-wrap: anywhere;
    font-size: var(--t-12);
  }
  .invrow input.link {
    width: 100%;
    background: var(--instrument);
    color: var(--ink);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    padding: 0.25rem 0.4rem;
  }
  .btns {
    display: inline-flex;
    gap: var(--sp-1);
  }
  @media (max-width: 600px) {
    .invrow {
      grid-template-columns: 1.5rem minmax(0, 1fr);
    }
    .invrow .link,
    .btns {
      grid-column: 1 / -1;
    }
  }
</style>
