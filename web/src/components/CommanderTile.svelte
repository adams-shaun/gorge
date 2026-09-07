<script lang="ts">
  import type { CommanderStatus } from '../lib/commander';
  import { nextCastCost } from '../lib/commander';
  import CardImage from './CardImage.svelte';
  import CardDetail from './CardDetail.svelte';
  import { HoverCard, type AnchorRect } from '../lib/carddetail.svelte';

  /**
   * One commander, drawn as a card in its seat's CREATURES row (CZ2) — a
   * commander is a creature, so it is drawn at the same --card-w the row
   * scales every other creature to, and reads the ambient `--card-w`
   * cascading from `.row.creatures` in Quadrant.svelte rather than setting a
   * scale of its own. Nothing here decides the number; it only inherits it.
   *
   * Commander identity is the premise of the format and a line of text is not
   * how anyone recognises a commander — you recognise it by its art. So this
   * is a card face, not a row: CardImage draws the Scryfall art when it
   * resolves and the typeset blank when it does not, and the blank is the
   * NORMAL state (cmd/gorged ships no catalog, and an offline box resolves
   * nothing). The name and the state therefore sit OUTSIDE the face, under
   * it, at full ink: they read the same whichever half CardImage drew, and
   * they survive the two dimmed states, where the face — the blank's own
   * typeset name included — is deliberately faded.
   *
   * THE TAX SITS ON THE FACE, in the corner a printed mana cost occupies —
   * by explicit request, in place of an earlier design that deliberately
   * kept it OFF the face (a derived value in the printed cost's own spot can
   * read as though it IS the printed cost). That risk is real, so the tile
   * marks it as computed rather than printed: parenthesised, `(+N)` rather
   * than a bare number the way a real pip would be, in the initiative colour
   * (not the blank's own text colour) and inside its own translucent chip
   * rather than CardImage's opaque pip disc. It overlays BOTH halves
   * CardImage can draw — the Scryfall art has no DOM hook of its own to
   * anchor a corner value to, so this tile draws its own corner independent
   * of which half rendered.
   *
   * THREE STATES, and every one of them is inspectable. The state is a
   * property of where the card currently is (lib/commander.ts, presenceOf):
   *
   *   command      full strength, plus the CR 903.8 next-cast tax when there
   *                is one — {2} per prior command-zone cast, the DERIVED
   *                number, never the raw count. It is here rather than on the
   *                rail because this is where the cast is decided from.
   *   battlefield  ghosted, and marked "on the battlefield". The real
   *                permanent is drawn in the creatures row of this same
   *                quadrant; two full-colour copies of one card in one
   *                quadrant is a lie about how many there are.
   *   away         greyed, and named with the zone it is in. A commander in a
   *                zone this viewer cannot browse resolves to "library" and
   *                is drawn exactly the same — not being able to see into a
   *                zone is not an error state.
   *
   * Hover dwell or keyboard focus opens the existing CardDetail inspector, in
   * EVERY state including the greyed one: an opponent's commander must be
   * readable at any time, including while it sits in a zone nobody can
   * otherwise browse, and that is the reason this feature is on the board
   * rather than in a rail. The wiring is CardTile's — one HoverCard, an
   * anchor rect captured on open, and superviseRendering keeping the panel's
   * lifetime tied to the object it was opened for.
   */
  // hover and anchor are injectable for the repo's SSR test harness, exactly
  // as CardTile takes them (CD1): that environment has no DOM, so no pointer
  // event can ever open the panel and $effect never runs. Production renders
  // never pass them and the defaults are what this component would have built
  // for itself.
  //
  // `seat` is carried only as a data attribute for the board test that counts
  // "one command area per commander seat" now that the tiles have no
  // wrapping element of their own (CZ2) — nothing here reads it.
  let { status, player, seat, hover = new HoverCard(), anchor: anchorProp = null }: {
    status: CommanderStatus;
    player: string;
    seat: number;
    hover?: HoverCard;
    anchor?: AnchorRect | null;
  } = $props();

  const card = $derived(status.commander);

  // What the state band says. The zone's own name doubles as the label for
  // `away`, because "graveyard" is more use to the reader than "elsewhere".
  //
  // The printed cost is deliberately NOT re-typeset here: with no art the
  // blank already sets it on the face's title line, so a second copy under
  // the tile would print the same cost twice on a creature-scale card; with
  // art it is one hover away in the inspector, which sets it in the panel
  // header. The tax IS drawn on the face (in the mana-cost corner, see the
  // component doc comment) but as its own marked-derived `(+N)` chip, never
  // as a rewrite of the printed cost itself.
  const label = $derived(
    status.presence === 'command' ? 'command zone' : status.presence === 'battlefield' ? 'on the battlefield' : status.zone,
  );
  const description = $derived(
    status.presence === 'command'
      ? `${player} — ${card.name} is in the command zone${status.tax > 0 ? `, next cast pays ${status.tax} generic commander tax (CR 903.8) over its printed cost` : ', no commander tax'}`
      : status.presence === 'battlefield'
        ? `${player} — ${card.name} is on the battlefield`
        : `${player} — ${card.name} is in the ${status.zone}`,
  );

  let root = $state<HTMLElement | null>(null);
  // The anchor the fixed panel is placed against. In production only
  // capture() sets it, from this tile's rect once a pointer or focus event
  // arrives; the injected one is a seed for the SSR harness, read once at
  // mount so the compiler never sees a prop read in a reactive position.
  const initialAnchor = () => anchorProp ?? null;
  let anchor = $state<AnchorRect | null>(initialAnchor());

  // Same lifecycle tie as CardTile (CD1): the panel's lifetime follows the
  // OBJECT it was opened for, not the mounting. These tiles are keyed by the
  // commander's id so Svelte will not hand one instance a different
  // commander, but the id is re-fed on every card change anyway — the guard
  // is the contract, not an assumption about the keying above it.
  $effect(() => {
    hover.superviseRendering(card.id);
  });

  function capture(): void {
    const r = root?.getBoundingClientRect();
    anchor = r ? { left: r.left, top: r.top, right: r.right } : null;
  }
</script>

<div
  class="cmd-tile cmd-tile--{status.presence}"
  data-commander={status.index}
  data-obj={card.id}
  data-cmd-state={status.presence}
  data-cmd-zone={status.zone}
  data-seat={seat}
  data-next-cost={nextCastCost(card.mana_cost, status.casts)}
  bind:this={root}
  tabindex="0"
  role="button"
  title={description}
  aria-label={description}
  onpointerenter={() => hover.arm(card.id, capture)}
  onpointerleave={() => hover.close()}
  onfocus={() => hover.open(card.id, capture)}
  onblur={() => hover.close()}
  onkeydown={(e) => hover.keydown(e)}
  aria-describedby={hover.show ? `card-detail-${card.id}` : undefined}
>
  <div class="face">
    <CardImage {card} pt={false} />
    {#if status.presence === 'command' && status.tax > 0}
      <span
        class="tax data"
        data-tax={status.tax}
        data-casts={status.casts}
        title="Commander tax (CR 903.8): {status.tax} generic on the next cast, for {status.casts} prior cast{status.casts === 1 ? '' : 's'}"
      >(+{status.tax})</span>
    {/if}
  </div>
  <div class="who">{card.name}</div>
  <div class="band">
    <span class="state">{label}</span>
  </div>
</div>

{#if hover.show && anchor}
  <CardDetail {card} anchor={anchor} />
{/if}

<style>
  /* A card lying in its own spot on the felt, with a state band under it
     rather than over it: the band is the one thing that must stay readable
     when the face is a ghost or a grey, so it never sits on the art. */
  /* Width comes straight from the row's own --card-w (104px in the creatures
     row) — a commander is a creature, drawn at creature scale, not at a
     private scale of its own (CZ2). No fallback is set here: outside a row
     that defines --card-w, CardImage's own 90px default applies, which only
     ever happens in a test render. */
  .cmd-tile {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    width: var(--card-w, 104px);
    flex: none;
  }
  /* position:relative gives the tax chip below a corner to anchor to — the
     same corner CardImage's own blank puts a printed mana cost in — that
     works over BOTH halves CardImage can draw: the blank (a DOM title line
     we could otherwise have shared) and the Scryfall art (a bare <img> with
     no cost hook of its own to anchor to). */
  .face {
    position: relative;
    line-height: 0;
  }
  /* Already on the battlefield: the permanent itself is drawn in the
     creatures row of this quadrant, so the tile is a placeholder for a card
     that is not here — a ghost of the face, not a second copy of it. */
  .cmd-tile--battlefield .face {
    opacity: 0.4;
  }
  /* Anywhere else: greyed, not hidden. Desaturated rather than faded to
     nothing, because it still has to be recognisable enough to be worth
     hovering — which is the whole point of drawing it at all. */
  .cmd-tile--away .face {
    filter: grayscale(1);
    opacity: 0.5;
  }
  .cmd-tile:focus-visible {
    outline: 2px solid var(--initiative);
    outline-offset: 2px;
  }

  /* The name, OUTSIDE the face and at full ink. It is not a duplicate of the
     blank's own title line: the blank sets the name beside the mana pips and
     clips it to a few characters even at creature scale, and in the two
     dimmed states the whole face — typeset name included — is drawn at
     40-50%, which is exactly when the reader most needs to know which
     commander the ghost is. One line, clipped, with the full name (and where
     it is) in the tile's title and accessible name. */
  .who {
    max-width: 100%;
    font-size: var(--t-10);
    line-height: 1.3;
    font-weight: 600;
    color: var(--ink);
    text-align: center;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* The band wraps rather than clips: "on the battlefield" is three words
     against a narrow card and half of it ("on the battlef…") is not a state
     anyone can read. A second line costs 13px at the rim, where there is
     room; an ellipsis costs the meaning. */
  .band {
    display: flex;
    align-items: baseline;
    justify-content: center;
    flex-wrap: wrap;
    gap: 0 0.35em;
    max-width: 100%;
    font-size: var(--t-10);
    line-height: 1.3;
    text-align: center;
  }
  .state {
    color: var(--ink-faint);
    min-width: 0;
  }
  /* The one castable state carries the initiative colour the client reserves
     for what can be acted on. */
  .cmd-tile--command .state {
    color: var(--initiative);
  }
  /* The tax sits where a printed mana cost lives — the corner the reader
     already scans for "what does this cost" — but it is not typeset like a
     printed pip, because it is not one: CardImage's own pips are opaque
     colour discs, printed values, one per card, permanent. This is a
     parenthesised, computed, CHANGING number, so it gets its own translucent
     chip, its own colour (the initiative hue this tile already reserves for
     "castable now" on the state band below), and parentheses no printed
     cost would ever carry — three signals that this is what the NEXT cast
     costs on top of the print, not the print itself. `line-height:0` on
     `.face` would otherwise collapse this to nothing, so it sets its own. */
  .tax {
    position: absolute;
    top: 2px;
    right: 2px;
    line-height: 1.3;
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    font-size: var(--t-10);
    font-weight: 600;
    color: var(--initiative);
    background: color-mix(in srgb, var(--felt-sunk) 85%, transparent);
    border-radius: 2px;
    padding: 0 3px;
  }
</style>
