<script lang="ts">
  import type { CardView } from '../protocol';
  import { oracle, type OracleCard } from '../lib/oracle';
  import { placePanel, PANEL_WIDTH, type AnchorRect } from '../lib/carddetail.svelte';
  import CardImage from './CardImage.svelte';
  import ManaSymbols from './ManaSymbols.svelte';

  /**
   * CardDetail is the large hover panel for one object. It always renders
   * what the WIRE knows — name, mana cost, types, CURRENT power/toughness,
   * damage marked, every counter, every derived keyword, #id, controller
   * seat, tapped/summoning-sick — then the large art, then the printed
   * oracle text only when the pluggable resolver returns it. No catalog
   * (the default for cmd/gorged) means the panel is complete from the wire
   * alone and shows no oracle block at all: not an error, not a spinner
   * shell. The component has no rules knowledge: the wire's P/T are current
   * values and every modifier is displayed, never folded into a base.
   */
  type CardResolver = (name: string) => OracleCard | null | Promise<OracleCard | null>;

  let { card, anchor, resolver = oracle.text }: { card: CardView; anchor: AnchorRect; resolver?: CardResolver } = $props();

  const isCreature = $derived(card.types.includes('Creature'));

  // Every counter, sorted so a map's iteration order can never reach the
  // rendered output (determinism contract).
  const counterChips = $derived(Object.entries(card.counters ?? {}).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)));

  // Oracle text is best-effort: the default resolver is the async catalog,
  // which with no meta tag resolves null without a request. The panel
  // renders no oracle block until the resolver returns one.
  let orc = $state<OracleCard | null>(initialOracle());

  // A resolver may answer synchronously (the server-render tests use one to
  // exercise the oracle block without a DOM; the production resolver is
  // async). The initializer covers that path: server render never runs the
  // effect below, so its answer must be in place from the first render.
  function initialOracle(): OracleCard | null {
    const v = resolver(card.name);
    if (v !== null && typeof (v as Promise<OracleCard | null>).then !== 'function') return v as OracleCard;
    return null;
  }

  $effect(() => {
    const name = card.name;
    const v = resolver(name);
    if (v === null) {
      orc = null;
      return;
    }
    if (typeof (v as Promise<OracleCard | null>).then === 'function') {
      let cancelled = false;
      (v as Promise<OracleCard | null>).then((c) => {
        if (!cancelled) orc = c;
      });
      return () => {
        cancelled = true;
      };
    }
    orc = v as OracleCard;
  });

  // When the catalog's PRINTED power/toughness disagrees with the wire's
  // current values, say so plainly. Provenance hook, deliberately naive:
  // the wire carries only current P/T (view.go reads the live object), so we
  // can state current vs printed but not HOW MUCH of the delta is counters,
  // auras or a base change — omitting that attribution is a known engine gap
  // (capability registry); this stays a plain statement and no fake base is
  // invented. Remove the naive compare when provenance is on the wire.
  const ptNote = $derived.by(() => {
    if (!isCreature || !orc) return null;
    const differs =
      (orc.power !== undefined && String(card.power) !== orc.power) ||
      (orc.toughness !== undefined && String(card.toughness) !== orc.toughness);
    if (!differs) return null;
    const p = orc.power ?? String(card.power);
    const t = orc.toughness ?? String(card.toughness);
    return `Shown P/T is current; the printed card reads ${p}/${t}`;
  });

  const placement = $derived(placePanel(anchor, typeof window === 'undefined' ? 0 : window.innerWidth, typeof window === 'undefined' ? 0 : window.innerHeight));
</script>

<div
  class="card-detail"
  id="card-detail-{card.id}"
  role="tooltip"
  style:left="{placement.x}px"
  style:top="{placement.y}px"
  style:width="{PANEL_WIDTH}px"
  style:max-height="{placement.maxHeight}px"
>
  <header class="card-detail__head">
    <span class="card-detail__name">{card.name}</span>
    {#if card.mana_cost}<ManaSymbols cost={card.mana_cost} />{/if}
  </header>
  <p class="card-detail__types">{card.types}</p>

  <CardImage {card} size="large" />

  {#if isCreature}
    <p class="card-detail__row" title="power/toughness — current engine values, not printed">
      P/T <span class="data">{card.power}/{card.toughness}</span>
    </p>
  {/if}
  {#if card.damage > 0}
    <p class="card-detail__row"><span class="damage">damage marked <span class="data">{card.damage}</span></span></p>
  {/if}
  {#if counterChips.length}
    <div class="card-detail__chips">
      {#each counterChips as [kind, n] (kind)}
        <span class="chip" title="{n} {kind}"><span class="data">{n}</span>× {kind}</span>
      {/each}
    </div>
  {/if}
  {#if card.keywords?.length}
    <div class="card-detail__chips">
      {#each card.keywords as kw (kw)}
        <span class="kw">{kw}</span>
      {/each}
    </div>
  {/if}
  <p class="card-detail__meta data">
    #{card.id} · seat {card.controller}{#if card.tapped} · tapped{/if}{#if card.summon_sick} · summoning sick{/if}
  </p>

  {#if ptNote}<p class="card-detail__note">{ptNote}</p>{/if}
  {#if orc?.oracle_text}<p class="card-detail__oracle">{orc.oracle_text}</p>{/if}
</div>

<style>
  /* A floating read-only panel: instrument register (cool, dense — where the
     engine explains what it did), pointer-events none so no underlying
     interaction is ever blocked by it. z-index 6 sits above the board's
     other overlays (RecentStrip uses 4) but BELOW the seat panel's option
     sheet (SeatPanel uses 8): the sheet is real clickable UI and must always
     stack above this read-only panel, and because the panel never takes
     pointer events the number is about visual order, not click capture. */
  .card-detail {
    position: fixed;
    z-index: 6;
    pointer-events: none;
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    padding: var(--sp-3);
    box-sizing: border-box;
    background: var(--instrument-raised);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius-card);
    box-shadow: 0 10px 28px rgb(0 0 0 / 0.5);
    overflow-y: auto;
    color: var(--ink-inst);
  }
  .card-detail__head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--sp-2);
  }
  .card-detail__name {
    font-weight: 600;
    font-size: var(--t-16);
    line-height: 1.25;
  }
  .card-detail__types {
    margin: 0;
    font-size: var(--t-12);
    opacity: 0.75;
  }
  .card-detail__row {
    margin: 0;
    display: flex;
    align-items: baseline;
    gap: var(--sp-2);
    font-size: var(--t-12);
    opacity: 0.9;
  }
  .damage {
    color: var(--danger);
  }
  /* Counters are modifications the player must SEE; keyword words are their
     own full text here — the whole point of the hover panel is reading the
     card, so nothing is abbreviated. */
  .card-detail__chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--sp-1);
  }
  .chip {
    background: var(--initiative);
    color: var(--felt-sunk);
    border-radius: 3px;
    padding: 0 var(--sp-2);
    font-size: var(--t-12);
    line-height: 1.5;
  }
  .kw {
    background: var(--edge-inst);
    color: var(--ink-inst);
    border-radius: 3px;
    padding: 0 var(--sp-2);
    font-size: var(--t-12);
    line-height: 1.5;
  }
  .card-detail__meta {
    font-size: var(--t-12);
    opacity: 0.6;
  }
  .card-detail__note {
    margin: 0;
    font-size: var(--t-12);
    color: var(--initiative);
  }
  .card-detail__oracle {
    margin: 0;
    font-size: var(--t-12);
    line-height: 1.5;
    opacity: 0.95;
    white-space: pre-wrap;
  }
</style>
