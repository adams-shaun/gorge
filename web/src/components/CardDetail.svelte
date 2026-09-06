<script lang="ts">
  import type { CardView } from '../protocol';
  import { oracle, type OracleCard } from '../lib/oracle';
  import { placePanel, PANEL_WIDTH, type AnchorRect } from '../lib/carddetail.svelte';
  import CardImage from './CardImage.svelte';
  import ManaSymbols from './ManaSymbols.svelte';

  /**
   * CardDetail is the object inspector: the large panel shown while a card is
   * hovered or focused, anywhere in the client.
   *
   * A card has two halves of truth and this panel is built on the seam
   * between them. The PRINTED card — art, cost, type line, oracle text — is
   * what every other client shows you. The ENGINE'S OBJECT — current power
   * and toughness, damage marked, every counter, every derived keyword, the
   * id, the controller, tapped and attacking — is what gorge knows and no
   * rival displays. So the printed card is the plate at the top and the
   * engine's object is a ledger beneath it, in the instrument's register:
   * hairline rows, labels in the interface face, values right-aligned in the
   * data face. The ledger is the reason the panel exists, and it is the part
   * that is loud.
   *
   * The ledger prints only rows that have something to say. A vanilla
   * creature's ledger is one row. That discipline is what separates an
   * inspector from a dump of every field on the struct.
   *
   * NO ART AND NO ORACLE IS THE NORMAL CASE, not an error: cmd/gorged injects
   * no catalog meta tag, so the resolver answers null without a request, and
   * a machine with no route to Scryfall resolves no image. The panel is
   * therefore designed from that state outward — with nothing resolved it is
   * a complete typeset record of the object, and the art plate and the oracle
   * block are additions to it rather than holes in it. In particular the art
   * plate is suppressed rather than replaced by CardImage's blank, because
   * the blank would only restate the header directly above it.
   *
   * The component has no rules knowledge: the wire's P/T are current values
   * and every modifier is displayed, never folded into a base.
   */
  type CardResolver = (name: string) => OracleCard | null | Promise<OracleCard | null>;

  let { card, anchor, resolver = oracle.text }: { card: CardView; anchor: AnchorRect; resolver?: CardResolver } = $props();

  const isCreature = $derived(card.types.includes('Creature'));

  // Every counter, sorted so a map's iteration order can never reach the
  // rendered output (determinism contract).
  const counterChips = $derived(Object.entries(card.counters ?? {}).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)));

  // Keywords arrive from the engine in two shapes and typesetting them the
  // same way is what made this panel read as a debug dump: "Flying" is a word
  // a player knows, "etbCounter:CHARGE:X" is a script parameter. The split is
  // lexical — a keyword carrying notation characters is engine notation —
  // and nothing here decides what either kind DOES. Neither is hidden: this
  // client shows its working. They are just not the same voice, so the words
  // are set as words and the notation is set as a value, in the data face.
  const isNotation = (k: string) => /[:<>$|]/.test(k);
  const abilities = $derived((card.keywords ?? []).filter((k) => !isNotation(k)));
  const engineKeywords = $derived((card.keywords ?? []).filter(isNotation));

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
  // current values, say so in the same row rather than as a warning below:
  // "4/3, printed 2/1" is one fact about one object, and splitting it into a
  // number and a sentence somewhere else is what makes a player hunt.
  // Provenance hook, deliberately naive: the wire carries only current P/T
  // (view.go reads the live object), so we can state current vs printed but
  // not HOW MUCH of the delta is counters, auras or a base change — omitting
  // that attribution is a known engine gap (capability registry); no fake
  // base is invented. Remove the naive compare when provenance is on the wire.
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

  const states = $derived(
    [
      card.tapped ? 'tapped' : null,
      card.attacking ? 'attacking' : null,
      card.summon_sick ? 'summoning sick' : null,
    ].filter((s): s is string => s !== null),
  );

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
  <!-- The printed card, full bleed, or nothing at all. -->
  <div class="plate"><CardImage {card} size="large" fallback="none" /></div>

  <header class="head">
    <h2 class="name">{card.name}</h2>
    {#if card.mana_cost}<ManaSymbols cost={card.mana_cost} />{/if}
  </header>
  <p class="types">{card.types}</p>

  <!-- The engine's object. Rows appear only when they carry something. -->
  <dl class="ledger">
    {#if isCreature}
      <div class="row">
        <dt>Power / toughness</dt>
        <dd>
          <span class="data value">{card.power}/{card.toughness}</span>
          {#if ptNote}<span class="printed">{ptNote}</span>{/if}
        </dd>
      </div>
    {/if}
    {#if card.damage > 0}
      <div class="row">
        <dt>Damage marked</dt>
        <dd><span class="data value value--danger" title="{card.damage} damage marked">{card.damage}</span></dd>
      </div>
    {/if}
    {#if counterChips.length}
      <div class="row row--set">
        <dt>Counters</dt>
        <dd class="chips">
          {#each counterChips as [kind, n] (kind)}
            <span class="counter" title="{n} {kind}"><span class="data">{n}</span> × {kind}</span>
          {/each}
        </dd>
      </div>
    {/if}
    {#if abilities.length}
      <div class="row row--set">
        <dt>Abilities</dt>
        <dd class="chips">
          {#each abilities as kw (kw)}<span class="kw">{kw}</span>{/each}
        </dd>
      </div>
    {/if}
    {#if engineKeywords.length}
      <div class="row row--set">
        <dt>Engine keywords</dt>
        <dd class="chips">
          {#each engineKeywords as kw (kw)}<span class="kw kw--notation data">{kw}</span>{/each}
        </dd>
      </div>
    {/if}
  </dl>

  {#if orc?.oracle_text}<p class="card-detail__oracle">{orc.oracle_text}</p>{/if}

  <footer class="stamp">
    <span class="data">#{card.id}</span>
    <span class="data">seat {card.controller}</span>
    {#each states as s (s)}<span class="state">{s}</span>{/each}
  </footer>
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
    box-shadow: var(--shadow-lift);
    overflow-y: auto;
    color: var(--ink-inst);
  }

  /* The art runs to the panel's edges. A card is an image with a border
     printed on it; adding a second border around it makes it a thumbnail. */
  .plate {
    display: none;
  }
  /* Only when an image actually resolved. With nothing to show the plate is
     not an empty frame or a spinner shell — it is simply absent, and the
     panel below closes over the space. */
  .plate:has(:global(.card-image)) {
    display: block;
    --card-w-large: 100%;
    --card-radius: 0;
    margin: calc(var(--sp-3) * -1) calc(var(--sp-3) * -1) 0;
    line-height: 0;
    border-bottom: 1px solid var(--edge-inst);
    overflow: hidden;
    border-radius: var(--radius) var(--radius) 0 0;
  }

  .head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--sp-2);
  }
  .name {
    margin: 0;
    font-size: var(--t-14);
    font-weight: 600;
    line-height: 1.25;
    color: var(--ink);
    letter-spacing: -0.005em;
  }
  .types {
    margin: 0;
    font-size: var(--t-11);
    color: var(--ink-dim);
  }

  /* The ledger. Hairline rows, label left in the interface face, value right
     in the data face — the instrument's own grammar, used here because these
     are readings taken off a running machine. */
  .ledger {
    margin: 0;
    display: grid;
    gap: 0;
  }
  .row {
    display: grid;
    grid-template-columns: auto 1fr;
    align-items: baseline;
    gap: var(--sp-3);
    padding: var(--sp-1) 0;
    border-top: 1px solid var(--edge-inst);
  }
  dt {
    font-size: var(--t-11);
    color: var(--ink-faint);
    white-space: nowrap;
  }
  dd {
    margin: 0;
    text-align: right;
    font-size: var(--t-12);
  }
  .value {
    font-size: var(--t-14);
    font-weight: 500;
    color: var(--ink);
  }
  .value--danger {
    color: var(--danger);
  }
  /* Provenance sits with the number it qualifies, not as a warning at the
     foot of the panel: "3/2, and the printed card reads 1/2" is one fact
     about one object. Left aligned because it is a sentence. */
  .printed {
    display: block;
    margin-top: var(--sp-1);
    text-align: left;
    font-size: var(--t-10);
    line-height: 1.45;
    color: var(--ink-faint);
  }

  /* A reading is right-aligned against its label; a SET is a list, and a list
     that wraps must not be right-aligned or the eye loses the left edge it
     scans down. So the ledger has two row shapes: scalar readings across, sets
     stacked under their label. */
  .row--set {
    grid-template-columns: 1fr;
    gap: var(--sp-1);
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-start;
    gap: var(--sp-1);
    text-align: left;
  }
  /* Counters are outlined, not filled: they are already the only saturated
     thing in the panel and a row of solid blocks shouts louder than the
     numbers they annotate. */
  .counter {
    border: 1px solid var(--initiative);
    color: var(--initiative);
    border-radius: var(--radius);
    padding: 0 var(--sp-1);
    font-size: var(--t-11);
    line-height: 1.5;
  }
  .kw {
    background: var(--edge-inst);
    color: var(--ink-inst);
    border-radius: var(--radius);
    padding: 0 var(--sp-2);
    font-size: var(--t-11);
    line-height: 1.6;
  }
  /* Engine notation breaks at its own separators, never mid-word. */
  .kw--notation {
    background: none;
    padding: 0;
    color: var(--ink-faint);
    font-size: var(--t-10);
    line-height: 1.5;
    overflow-wrap: anywhere;
  }

  /* The printed text, when a catalog is configured. Full ink: this is the
     card talking, not the panel annotating it. */
  .card-detail__oracle {
    margin: 0;
    padding-top: var(--sp-2);
    border-top: 1px solid var(--edge-inst);
    font-size: var(--t-12);
    line-height: 1.55;
    color: var(--ink);
    white-space: pre-wrap;
  }

  /* Identity last, and spaced rather than punctuated: the design system has
     no middle-dot meta strings. */
  .stamp {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--sp-1) var(--sp-3);
    padding-top: var(--sp-1);
    border-top: 1px solid var(--edge-inst);
    color: var(--ink-faint);
  }
  .state {
    font-size: var(--t-11);
    color: var(--ink-dim);
  }
</style>
