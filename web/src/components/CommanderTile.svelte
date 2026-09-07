<script lang="ts">
  import type { CommanderStatus } from '../lib/commander';
  import { nextCastCost } from '../lib/commander';
  import CardImage from './CardImage.svelte';
  import CardDetail from './CardDetail.svelte';
  import { HoverCard, type AnchorRect } from '../lib/carddetail.svelte';

  /**
   * One commander, drawn as a card in its seat's command area.
   *
   * Commander identity is the premise of the format and a line of text is not
   * how anyone recognises a commander — you recognise it by its art. So this
   * is a card face, not a row: CardImage draws the Scryfall art when it
   * resolves and the typeset blank when it does not, and the blank is the
   * NORMAL state (cmd/gorged ships no catalog, and an offline box resolves
   * nothing). The name, the state and the tax therefore sit OUTSIDE the face,
   * under it, at full ink: they read the same whichever half CardImage drew,
   * and they survive the two dimmed states, where the face — the blank's own
   * typeset name included — is deliberately faded.
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
  let { status, player, hover = new HoverCard(), anchor: anchorProp = null }: {
    status: CommanderStatus;
    player: string;
    hover?: HoverCard;
    anchor?: AnchorRect | null;
  } = $props();

  const card = $derived(status.commander);

  // What the state band says. The zone's own name doubles as the label for
  // `away`, because "graveyard" is more use to the reader than "elsewhere".
  //
  // The printed cost is deliberately NOT re-typeset here: with no art the
  // blank already sets it on the face's title line, so a second copy under
  // the tile would print the same cost twice on an 80px card; with art it is
  // one hover away in the inspector, which sets it in the panel header. The
  // tax chip is the number that is NOT on any face — it is derived, it
  // changes, and it is the reason this moved off the rail.
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
  <div class="face"><CardImage {card} pt={false} /></div>
  <div class="who">{card.name}</div>
  <div class="band">
    <span class="state">{label}</span>
    {#if status.presence === 'command' && status.tax > 0}
      <span
        class="tax data"
        data-tax={status.tax}
        data-casts={status.casts}
        title="Commander tax (CR 903.8): {status.tax} generic on the next cast, for {status.casts} prior cast{status.casts === 1 ? '' : 's'}"
      >+{status.tax}</span>
    {/if}
  </div>
</div>

{#if hover.show && anchor}
  <CardDetail {card} anchor={anchor} />
{/if}

<style>
  /* A card lying in its own spot on the felt, with a state band under it
     rather than over it: the band is the one thing that must stay readable
     when the face is a ghost or a grey, so it never sits on the art. */
  .cmd-tile {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    width: var(--cmd-w);
    flex: none;
  }
  .face {
    --card-w: var(--cmd-w);
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
     blank's own title line: at 80px the blank sets the name beside the mana
     pips and clips it to a few characters, and in the two dimmed states the
     whole face — typeset name included — is drawn at 40-50%, which is
     exactly when the reader most needs to know which commander the ghost is.
     One line, clipped, with the full name (and where it is) in the tile's
     title and accessible name. */
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
     against a 72px card and half of it ("on the battlef…") is not a state
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
  /* The tax is a value, so it reads in the data face: +6, never "3 casts". */
  .tax {
    flex: none;
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    color: var(--ink-dim);
  }
</style>
