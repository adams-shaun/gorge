<script lang="ts">
  import type { EventClient } from '../../lib/draftevent.svelte';
  import { cardView, factsOf, openedBy, passDirection, rarityLabel, seatLabel, COLOURS, tint } from '../../lib/draftcards';
  import CardImage from '../CardImage.svelte';
  import MvColumns from './MvColumns.svelte';
  import PodStatus from './PodStatus.svelte';
  import PickClock from './PickClock.svelte';
  import Attribution from './Attribution.svelte';
  import CardDetail from '../CardDetail.svelte';
  import { DraftHover } from '../../lib/drafthover.svelte';
  import { identityColours } from '../../lib/commanderdraft';
  import type { AdviceMark, DraftAssist } from '../../lib/draftassist.svelte';

  /**
   * The pick screen (mockup 3b). Click or tap a card to inspect it;
   * double-click it, press Enter on the selected card, or use "Take card" to
   * pick it. The pick is an intent — (round, pick_no, id) — and the screen
   * then shows whatever pack the server says the seat holds.
   *
   * When a seat takes more than one card per pass (a Commander draft: the
   * pack's picks_per_pass / pass_pick), the server hands back the SAME pack
   * minus the card just taken; the header then says which pick of the pass
   * this is and shows the card(s) already taken from this pack (the tail of
   * the pool, which the server keeps in pick order). Commander-draft cards
   * also wear their colour identity and a commander / partner tag.
   *
   * `assist` is the page's bot-assist switch (off by default). While on, the
   * seat's own drafter is asked about each new pack and its picks wear a
   * numbered badge with the drafter's short reason; the plan's colour
   * identity shows in the rail. A drafter that gives no advice (409)
   * withdraws the switch.
   */
  let { client, rated = true, assist = null }: { client: EventClient; rated?: boolean; assist?: DraftAssist | null } = $props();

  const pack = $derived(client.pack);
  const ev = $derived(client.state);
  const you = $derived(client.you);
  const cards = $derived(pack && !pack.waiting ? pack.pack : []);
  const pool = $derived(pack?.pool ?? []);
  const selected = $derived(cards.find((c) => c.id === client.selected) ?? null);
  const selectedFacts = $derived(selected ? factsOf(selected) : null);
  const round = $derived(pack && !pack.waiting ? pack.round : (client.progress?.round ?? 0));
  const dir = $derived(passDirection(round || 1));
  const perPass = $derived(Math.max(pack?.picks_per_pass ?? ev?.picks_per_pass ?? 1, 1));
  const passPick = $derived(pack && !pack.waiting && perPass > 1 ? Math.min(Math.max(pack.pass_pick ?? 1, 1), perPass) : 1);
  // The cards already taken from this pack this pass: the pool's tail.
  const takenThisPass = $derived(passPick > 1 ? pool.slice(-(passPick - 1)) : []);
  const assisting = $derived(assist !== null && assist.on && !assist.unavailable);
  const advice = $derived(assisting ? (assist?.forPack(pack) ?? null) : null);
  const marks = $derived(assisting && assist ? assist.marks(pack) : new Map<string, AdviceMark>());
  // Ask about each new pack while the switch is on; the answer for a pack
  // that has already moved on is dropped inside DraftAssist.
  $effect(() => {
    const p = pack;
    const token = client.token;
    if (assisting && assist && p && !p.waiting && token) void assist.fetch(client.id, token, p);
  });
  const opener = $derived(
    pack && !pack.waiting && you !== null ? openedBy(you, pack.round, pack.pick_no, ev?.seats.length || 4, perPass) : null,
  );

  const colourCounts = $derived.by(() => {
    const out: Record<string, number> = { W: 0, U: 0, B: 0, R: 0, G: 0 };
    for (const c of pool) for (const col of factsOf(c).colours) out[col]++;
    return out;
  });

  // Hovering a pack card opens the same zoomed CardDetail panel the table
  // uses; a picked card leaves the pack under the pointer, so supervise
  // closes a panel whose card is gone.
  const detail = new DraftHover();
  $effect(() => detail.supervise(cards));

  function onkey(e: KeyboardEvent, id: string) {
    detail.keydown(e);
    if (e.key !== 'Enter' && e.key !== ' ') return;
    e.preventDefault();
    if (client.selected === id) void client.pick(id);
    else client.select(id);
  }
</script>

<div class="ev-split draft">
  <section class="ev-main">
    {#if pack && !pack.waiting}
      <div class="packhead">
        <h2>Pack {pack.round} · Pick {pack.pick_no}</h2>
        <span class="data dim">
          {cards.length} {cards.length === 1 ? 'card' : 'cards'} ·
          {opener === null || opener === you ? 'your own pack' : `opened by ${seatLabel(opener, ev?.seats[opener]?.name, you)}`}
        </span>
        <span class="dir data">passing {dir} {dir === 'left' ? '←' : '→'}</span>
        {#if (pack.time_limit_ms ?? 0) > 0}
          <PickClock remainingMs={pack.remaining_ms ?? 0} arrivedAt={client.packAt} />
        {:else}
          <span class="data dim" data-testid="no-pick-timer">no pick timer</span>
        {/if}
        <span class="sp"></span>
        {#if assist && !assist.unavailable}
          <button
            type="button"
            class="ev-btn sm assist"
            role="switch"
            aria-checked={assist.on}
            data-testid="assist-toggle"
            title="Mark the picks your seat's drafter bot would make"
            onclick={() => assist?.toggle()}
          >Bot assist: {assist.on ? 'On' : 'Off'}</button>
        {/if}
        <button type="button" class="ev-btn primary" data-testid="take" disabled={!selected || client.busy} onclick={() => void client.pick()}>
          {client.busy ? 'Taking…' : selected ? `Take ${selected.name}` : 'Take card'}{perPass > 1 ? ` (${passPick} of ${perPass})` : ''}
        </button>
      </div>
      {#if perPass > 1}
        <div class="passbar" class:again={passPick > 1} role="status" data-testid="pass-pick" data-pass-pick={passPick}>
          <b>Pick {passPick} of {perPass} from this pack</b>
          {#if passPick > 1}
            <span>
              You already took
              {#each takenThisPass as t, j (t.id)}<em>{t.name}</em>{j < takenThisPass.length - 1 ? ', ' : ''}{/each}
              from it. Take {perPass - passPick + 1 === 1 ? 'one more card' : `${perPass - passPick + 1} more cards`} before it passes {dir}.
            </span>
          {:else}
            <span>You take {perPass} cards from each pack before it passes {dir}.</span>
          {/if}
        </div>
      {/if}
      {#if client.notice}<p class="ev-notice" role="status">{client.notice}</p>{/if}
      {#if client.error}<p class="ev-error" role="alert">{client.error}</p>{/if}
      <div class="pack" role="listbox" aria-label="Pack" data-testid="pack">
        {#each cards as c (c.id)}
          <button
            type="button"
            role="option"
            class="pcard ev-tint-{tint(factsOf(c))}"
            class:sel={c.id === client.selected}
            aria-selected={c.id === client.selected}
            aria-label={`${c.name}, ${rarityLabel(c.rarity)}`}
            data-id={c.id}
            disabled={client.busy}
            onclick={() => client.select(c.id)}
            ondblclick={() => void client.pick(c.id)}
            onkeydown={(e) => onkey(e, c.id)}
            onpointerenter={(e) => detail.enter(c, e.currentTarget)}
            onpointerleave={() => detail.leave()}
            onfocus={(e) => detail.focus(c, e.currentTarget)}
            onblur={() => detail.leave()}
          >
            <CardImage card={cardView(c)} pt={false} />
            <span class="ev-rar {c.rarity}" title={rarityLabel(c.rarity)}></span>
            {#if marks.has(c.id)}
              {@const m = marks.get(c.id)!}
              <span class="advbadge" data-testid="assist-badge" data-rank={m.rank} title={m.pick.reason}>{m.rank}</span>
              <span class="advreason">{m.pick.reason}</span>
            {/if}
            {#if c.commander || c.partner}
              <span class="cmdtag" title={c.partner ? 'Can be a commander, or half of a commander pair' : 'Can be a commander'}>{c.partner ? 'Partner' : 'Commander'}</span>
            {/if}
            {#if c.identity !== undefined && (c.commander || c.partner)}
              <span class="idtag" aria-label={`Colour identity ${c.identity || 'colourless'}`}>
                {#each identityColours(c.identity) as col (col)}<span class="ev-pip {col}">{col}</span>{/each}
              </span>
            {/if}
          </button>
        {/each}
      </div>
      <p class="hint">Hover a card to zoom it · tap to select · double-click, or Enter twice, to take it.</p>
      {#if detail.show && detail.card && detail.anchor}
        <CardDetail card={cardView(detail.card)} anchor={detail.anchor} />
      {/if}
    {:else}
      <div class="waiting" role="status">
        <h2>Waiting for the next pack</h2>
        <p class="dim">The seat passing to you is still picking. Your pack arrives here the moment it does.</p>
        {#if client.error}<p class="ev-error" role="alert">{client.error}</p>{/if}
      </div>
    {/if}

    <div class="pool">
      <div class="row">
        <h3 class="ev-h5">Your picks · {pool.length}</h3>
        <span class="legend data">
          {#each COLOURS as col (col)}
            {#if colourCounts[col]}<span><span class="ev-pip {col}">{col}</span> {colourCounts[col]}</span>{/if}
          {/each}
        </span>
      </div>
      <MvColumns cards={pool} label="Your picks" />
    </div>
    {#if rated}<Attribution />{/if}
  </section>

  <aside class="ev-rail">
    <div class="block">
      <h3 class="ev-h5">Pod</h3>
      {#if ev}<PodStatus seats={ev.seats} {you} progress={client.progress} />{/if}
    </div>
    <div class="block">
      <h3 class="ev-h5">Inspect</h3>
      {#if assisting}
        <div class="advplan" data-testid="assist-plan">
          <span class="data dim">Bot plan</span>
          {#if advice?.plan}
            {#each identityColours(advice.plan) as col (col)}<span class="ev-pip {col}">{col}</span>{/each}
          {:else if advice}
            <span class="data dim">no commander yet</span>
          {:else}
            <span class="data dim">{assist?.error ?? 'asking…'}</span>
          {/if}
        </div>
      {/if}
      {#if selected}
        <div class="big"><CardImage card={cardView(selected)} size="large" pt={false} /></div>
        <p class="data dim center">
          {selectedFacts?.types || selected.name}
          {#if selectedFacts && selectedFacts.mv !== null}· MV {selectedFacts.mv}{/if}
          · {rarityLabel(selected.rarity)}
        </p>
      {:else}
        <p class="dim small">Tap a card to inspect it. Double-click picks it.</p>
      {/if}
    </div>
  </aside>
</div>

<style>
  .packhead {
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: var(--sp-2) var(--sp-3);
  }
  h2 {
    margin: 0;
    font: 600 1.5rem var(--font-serif);
    color: var(--ink);
  }
  .dim {
    color: var(--ink-dim);
  }
  .small {
    font-size: var(--t-12);
  }
  .center {
    text-align: center;
  }
  .dir {
    color: var(--gilt);
  }
  .sp {
    flex: 1;
  }
  /* The pack: card art at a readable size, as many per row as fit. */
  .pack {
    --card-w: 100%;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(7.25rem, 28vw), 1fr));
    gap: var(--sp-3) var(--sp-2);
  }
  .pcard {
    position: relative;
    display: block;
    padding: 0;
    border: 0;
    background: none;
    border-radius: var(--radius-card);
    cursor: pointer;
    transition: transform 0.12s;
    min-width: 0;
  }
  .pcard :global(.card-image) {
    width: 100%;
  }
  .pcard:hover:not([disabled]) {
    transform: translateY(-3px);
  }
  .pcard.sel {
    outline: 2px solid var(--gilt);
    outline-offset: 2px;
    transform: translateY(-4px);
  }
  .pcard .ev-rar {
    position: absolute;
    left: 0.35rem;
    bottom: 0.35rem;
    box-shadow: 0 0 0 2px rgb(0 0 0 / 0.55);
  }
  .passbar {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--sp-1) var(--sp-3);
    padding: var(--sp-2) var(--sp-3);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    background: var(--instrument);
    font-size: var(--t-14);
    color: var(--ink-inst);
  }
  .passbar b {
    color: var(--ink);
  }
  .passbar.again {
    border-color: var(--gilt);
    background: color-mix(in srgb, var(--gilt) 14%, var(--instrument));
    box-shadow: 0 0 0 1px var(--gilt);
  }
  .passbar.again b {
    color: var(--gilt);
  }
  .passbar em {
    font-style: normal;
    font-weight: 600;
    color: var(--ink);
  }
  .assist[aria-checked='true'] {
    border-color: var(--verdigris);
    color: var(--verdigris);
  }
  .advbadge {
    position: absolute;
    left: -0.35rem;
    top: -0.35rem;
    display: grid;
    place-items: center;
    width: 1.5rem;
    height: 1.5rem;
    border-radius: 50%;
    background: var(--verdigris);
    color: var(--felt-sunk);
    font: 700 var(--t-12) var(--font-data);
    box-shadow: 0 0 0 2px rgb(0 0 0 / 0.55);
    z-index: 1;
  }
  .advreason {
    position: absolute;
    left: 0.25rem;
    right: 0.25rem;
    bottom: 1.6rem;
    padding: 0.1rem 0.3rem;
    border-radius: 3px;
    background: rgb(0 0 0 / 0.78);
    color: var(--ink);
    font: 500 var(--t-10) / 1.25 var(--font-ui);
    overflow-wrap: anywhere;
  }
  .advplan {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .cmdtag {
    position: absolute;
    right: 0.35rem;
    top: 0.35rem;
    padding: 0 0.3rem;
    border-radius: 3px;
    background: rgb(0 0 0 / 0.7);
    color: var(--gilt);
    font: 600 var(--t-10) var(--font-data);
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }
  .idtag {
    position: absolute;
    right: 0.35rem;
    bottom: 0.35rem;
    display: inline-flex;
    gap: 2px;
  }
  .hint {
    margin: 0;
    font-size: var(--t-11);
    color: var(--ink-faint);
  }
  .waiting {
    padding: var(--sp-6) 0;
  }
  .waiting p {
    margin: var(--sp-2) 0 0;
  }
  .pool {
    border-top: 1px solid var(--edge-felt);
    padding-top: var(--sp-2);
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
  }
  .row {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    flex-wrap: wrap;
    gap: var(--sp-2);
  }
  .legend {
    display: flex;
    gap: var(--sp-2);
    flex-wrap: wrap;
    color: var(--ink-dim);
  }
  .legend > span {
    display: inline-flex;
    align-items: center;
    gap: 3px;
  }
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
  }
  .big {
    --card-w-large: min(13rem, 100%);
    display: flex;
    justify-content: center;
  }
</style>
