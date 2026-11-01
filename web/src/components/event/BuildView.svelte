<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { EventState } from '../../protocol';
  import { basicsTotal, STEPPER_MAX, type DeckBuilder, type Zone } from '../../lib/deckbuild.svelte';
  import { BASIC_NAMES, COLOUR_FILTERS, COLOURS, factsOf, passesFilter, seatLabel } from '../../lib/draftcards';
  import MvColumns from './MvColumns.svelte';
  import Attribution from './Attribution.svelte';

  /**
   * The deckbuilder (mockup 3c). Two zones, Deck and Pool · sideboard: drag
   * a card between them, or click/tap it to move it to the other zone (the
   * phone path — touch browsers do not start HTML5 drags). Basics are free;
   * set them with the steppers or let "Auto" ask the server for its split.
   *
   * The count, validity and error shown are the server's DeckCheck for the
   * current working copy, never a client-side verdict.
   *
   * `actions` replaces the Ready block: the between-games sheet
   * edits a sideboarding working copy with its own Keep / Submit buttons.
   */
  let { builder, ev, rated = true, actions }: { builder: DeckBuilder; ev: EventState | null; rated?: boolean; actions?: Snippet } = $props();

  const you = $derived(ev?.you ?? null);
  const deckCards = $derived(builder.deckCards);
  const sideCards = $derived(builder.sideCards.filter((c) => passesFilter(builder.filter, factsOf(c))));
  const check = $derived(builder.check);
  const spells = $derived(deckCards.filter((c) => !factsOf(c).land).length);
  const lands = $derived(deckCards.length - spells + basicsTotal(builder.basics));

  const curve = $derived.by(() => {
    const out = [0, 0, 0, 0, 0, 0, 0];
    for (const c of deckCards) {
      const f = factsOf(c);
      if (f.land || f.mv === null) continue;
      out[Math.min(f.mv, 6)]++;
    }
    return out;
  });
  const curveMax = $derived(Math.max(1, ...curve));

  const pips = $derived.by(() => {
    const out = [0, 0, 0, 0, 0];
    for (const c of deckCards) {
      const f = factsOf(c);
      if (f.land) continue;
      for (const tok of f.cost.split(' ')) for (const ch of tok) {
        const i = COLOURS.indexOf(ch as (typeof COLOURS)[number]);
        if (i >= 0) out[i]++;
      }
    }
    return out;
  });

  let over = $state<Zone | null>(null);

  function dragover(e: DragEvent, z: Zone) {
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
    over = z;
  }
  function dragleave(e: DragEvent, z: Zone) {
    const next = e.relatedTarget as Node | null;
    if (next && (e.currentTarget as HTMLElement).contains(next)) return;
    if (over === z) over = null;
  }
  function drop(e: DragEvent, z: Zone) {
    e.preventDefault();
    over = null;
    const id = e.dataTransfer?.getData('text/plain');
    if (id) builder.move(id, z);
  }

  const others = $derived((ev?.seats ?? []).filter((s) => s.seat !== you));
  const readyLabel = $derived(
    builder.busy ? 'Saving…' : builder.ready && !builder.dirty ? 'Ready ✓' : builder.ready ? 'Ready again with these changes' : 'Ready',
  );
</script>

<div class="ev-split build">
  <section class="ev-main">
    {#if builder.error}<p class="ev-error" role="alert" data-testid="build-error">{builder.error}</p>{/if}
    <div
      class="zone"
      class:over={over === 'deck'}
      data-zone="deck"
      data-testid="zone-deck"
      role="region"
      aria-label="Deck"
      ondragover={(e) => dragover(e, 'deck')}
      ondragleave={(e) => dragleave(e, 'deck')}
      ondrop={(e) => drop(e, 'deck')}
    >
      <div class="zh">
        <h3>Deck</h3>
        <span class="data dim">{spells} spells · {lands} lands</span>
      </div>
      {#if deckCards.length === 0}<p class="dim small">Drag cards here, or tap one in the pool.</p>{/if}
      <MvColumns cards={deckCards} draggable onactivate={(id) => builder.move(id, 'side')} label="Deck cards" />
    </div>
    <div
      class="zone"
      class:over={over === 'side'}
      data-zone="side"
      data-testid="zone-side"
      role="region"
      aria-label="Pool and sideboard"
      ondragover={(e) => dragover(e, 'side')}
      ondragleave={(e) => dragleave(e, 'side')}
      ondrop={(e) => drop(e, 'side')}
    >
      <div class="zh">
        <h3>Pool · sideboard</h3>
        <div class="chips" role="group" aria-label="Colour filter">
          {#each COLOUR_FILTERS as f (f.key)}
            <button type="button" class="ev-chip" class:on={builder.filter === f.key} aria-pressed={builder.filter === f.key} onclick={() => builder.setFilter(f.key)}>{f.label}</button>
          {/each}
        </div>
      </div>
      <MvColumns cards={sideCards} draggable onactivate={(id) => builder.move(id, 'deck')} label="Pool cards" />
    </div>
    {#if rated}<Attribution what="Pick ratings and the suggested deck" />{/if}
  </section>

  <aside class="ev-rail">
    <div class="block">
      <h3 class="ev-h5">Deck size</h3>
      <div class="count" class:ok={check?.valid} data-testid="deck-count">
        {check ? check.total : '…'} <small>/ 40 minimum</small>
      </div>
      {#if check && !check.valid && check.error}
        <p class="check-error" role="status" data-testid="check-error">{check.error}</p>
      {:else if check?.valid}
        <p class="check-ok small">The server accepts this deck.</p>
      {/if}
    </div>

    <div class="block">
      <div class="between">
        <h3 class="ev-h5">Basic lands · free</h3>
        <button type="button" class="ev-btn sm" data-testid="auto-lands" onclick={() => void builder.autoLands()}>Auto</button>
      </div>
      <div class="lands">
        {#each BASIC_NAMES as name, i (name)}
          <div class="landrow">
            <span class="ev-pip {COLOURS[i]}">{COLOURS[i]}</span>
            <span>{name}</span>
            <span class="stepper">
              <button type="button" aria-label={`fewer ${name}`} disabled={builder.basics[i] <= 0} onclick={() => builder.step(i, -1)}>−</button>
              <span class="data" data-testid={`basic-${COLOURS[i]}`}>{builder.basics[i]}</span>
              <button type="button" aria-label={`more ${name}`} disabled={builder.basics[i] >= STEPPER_MAX} onclick={() => builder.step(i, 1)}>+</button>
            </span>
          </div>
        {/each}
      </div>
      {#if check}
        <p class="dim small">Auto would add: {BASIC_NAMES.map((n, i) => (check.auto_lands[i] ? `${check.auto_lands[i]} ${n}` : '')).filter(Boolean).join(', ') || 'no basics'}</p>
      {/if}
    </div>

    <div class="block">
      <h3 class="ev-h5">Curve · spells</h3>
      <div class="curve" aria-label="Mana curve">
        {#each curve as n, i (i)}
          <div class="b"><em>{n || ''}</em><i style:height={`${(n / curveMax) * 70}%`}></i><span>{i === 6 ? '6+' : i}</span></div>
        {/each}
      </div>
    </div>

    <div class="block">
      <h3 class="ev-h5">Pips vs basics</h3>
      {#each COLOURS as col, i (col)}
        {#if pips[i] || builder.basics[i]}
          <div class="piprow data"><span class="ev-pip {col}">{col}</span><span>{pips[i]} pips · {builder.basics[i]} {BASIC_NAMES[i]}</span></div>
        {/if}
      {/each}
    </div>

    <div class="block">
      <button type="button" class="ev-btn sm" onclick={() => void builder.resetToSuggestion()} disabled={!builder.suggestion}>Reset to bot suggestion</button>
    </div>

    {#if actions}
      <div class="block grow">{@render actions()}</div>
    {:else}
    <div class="block grow">
      <div class="readyrow">
        {#each others as s (s.seat)}
          <div><span>{seatLabel(s.seat, s.name, you)}</span><span class={s.ready ? 'ok' : 'wait'}>{s.ready ? 'ready' : 'building…'}</span></div>
        {/each}
      </div>
      <button type="button" class="ev-btn primary" data-testid="ready" disabled={builder.busy || !check?.valid} onclick={() => void builder.readyUp()}>{readyLabel}</button>
      {#if builder.ready && !builder.dirty}<p class="dim small">Waiting for the pod. Change anything and Ready again to resubmit.</p>{/if}
    </div>
    {/if}
  </aside>
</div>

<style>
  .dim {
    color: var(--ink-dim);
  }
  .small {
    font-size: var(--t-12);
    margin: 0;
  }
  .zone {
    border: 1px dashed var(--edge-felt);
    border-radius: var(--radius);
    padding: var(--sp-2);
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    min-height: 8rem;
    transition: background 0.1s, border-color 0.1s;
  }
  .zone.over {
    border-color: var(--verdigris);
    background: color-mix(in srgb, var(--verdigris) 6%, transparent);
  }
  .zh {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--sp-2);
    flex-wrap: wrap;
  }
  .zh h3 {
    margin: 0;
    font: 600 1.05rem var(--font-serif);
    color: var(--ink);
  }
  .chips {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }
  .block {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
  }
  .grow {
    margin-top: auto;
  }
  .between {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .count {
    font: 500 1.6rem var(--font-data);
    color: var(--ink);
  }
  .count small {
    font-size: var(--t-12);
    color: var(--ink-dim);
  }
  .count.ok {
    color: var(--verdigris);
  }
  .check-error {
    margin: 0;
    font-size: var(--t-12);
    color: var(--ember);
    overflow-wrap: anywhere;
  }
  .check-ok {
    color: var(--verdigris);
  }
  .lands {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .landrow {
    display: grid;
    grid-template-columns: 1.1rem 1fr auto;
    gap: var(--sp-2);
    align-items: center;
    font-size: var(--t-14);
  }
  .stepper {
    display: inline-flex;
    align-items: center;
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    overflow: hidden;
  }
  .stepper button {
    background: var(--instrument-raised);
    border: 0;
    color: var(--ink-inst);
    width: 1.9rem;
    height: 1.9rem;
    cursor: pointer;
    font: 500 var(--t-14) var(--font-data);
  }
  .stepper button:hover:not([disabled]) {
    color: var(--ink);
    background: var(--edge-inst);
  }
  .stepper button[disabled] {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .stepper span {
    min-width: 1.8rem;
    text-align: center;
    color: var(--ink);
  }
  .curve {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 4px;
    align-items: end;
    height: 5rem;
  }
  .curve .b {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: flex-end;
    gap: 2px;
    height: 100%;
  }
  .curve .b i {
    display: block;
    width: 100%;
    background: var(--ink-dim);
    border-radius: 2px 2px 0 0;
    min-height: 1px;
  }
  .curve .b span {
    font: var(--t-10) var(--font-data);
    color: var(--ink-faint);
  }
  .curve .b em {
    font: normal var(--t-10) var(--font-data);
    color: var(--ink-inst);
  }
  .piprow {
    display: flex;
    gap: var(--sp-2);
    align-items: center;
    color: var(--ink-inst);
  }
  .readyrow {
    display: flex;
    flex-direction: column;
    gap: 5px;
    font-size: var(--t-12);
  }
  .readyrow div {
    display: flex;
    justify-content: space-between;
    gap: var(--sp-2);
  }
  .ok {
    color: var(--verdigris);
  }
  .wait {
    color: var(--ink-faint);
  }
</style>
