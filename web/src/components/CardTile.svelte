<script lang="ts">
  import type { CardView } from '../protocol';
  import CardImage from './CardImage.svelte';
  import CardDetail from './CardDetail.svelte';
  import { HoverCard, type AnchorRect } from '../lib/carddetail.svelte';

  /**
   * CardTile is the battlefield/stack/strip face of one object. It has no
   * rules knowledge: CardImage draws the face (art, or the typeset blank with
   * name/mana symbols/types/P-T); this wrapper only adds per-instance state
   * that is not part of the card's printed identity — tapped, attacking,
   * damage, counters, summoning sickness, attachments — plus the data-obj
   * anchor arrows use, and the hover/focus detail: a pointer dwell (~250ms)
   * or keyboard focus opens the large CardDetail panel, pointer leave / blur
   * / Escape close it.
   *
   * SIGNAL HIERARCHY. Four things compete for a 90px box, so each gets one
   * corner and one meaning, learned once and never moved:
   *
   *   the face      the card itself, and nothing is allowed to cover the art
   *                 except at its edges
   *   top left      keyword marks, at most three, then "+n" — what the object
   *                 CAN do, and the only band that is about the printed card
   *   bottom edge   what has HAPPENED to it: counters at one end, damage and
   *                 current power/toughness at the other, on one band. These
   *                 three were in three different corners, which is the wrong
   *                 grouping — "5/5, 2 damage, two +1/+1 counters" is one
   *                 reading, and it is the reading a combat is scanned for.
   *                 P/T is drawn even when the art loads, because the art
   *                 shows the PRINTED numbers and the wire's are current, and
   *                 the blank's own corner numbers are turned off so the two
   *                 never print on top of each other.
   *
   * A red rim means attacking (XMage's convention, survey #16), so the
   * relationship the arrow draws is also readable on the permanent itself.
   *
   * Tapped is a real 90° turn, the physical convention, and the slot changes
   * shape with it so a tapped permanent takes the table space it actually
   * takes instead of overlapping its neighbours. The badges do not turn with
   * the card: a tapped creature's power still has to be readable.
   *
   * `keywords` on the wire are DERIVED (view.go reads ch.Keywords), so a
   * granted ability shows here exactly like a printed one. The abbreviations
   * below are display only; nothing here decides what a keyword does.
   */
  // hover and anchor are injectable so the repo's SSR test harness (no DOM,
  // no pointer events, no $effect) can drive the panel's lifecycle through
  // the same HoverCard the component owns; production renders never pass them
  // and the defaults are exactly what the component built for itself before.
  let { card, size = 'tile', attachments = [], hover = new HoverCard(), anchor: anchorProp = null, tileOptions = null, open0 = false }: {
    card: CardView;
    size?: 'tile' | 'large';
    attachments?: CardView[];
    hover?: HoverCard;
    anchor?: AnchorRect | null;
    /** tileOptions is the pending decision's offers for THIS object (see
     *  lib/cardoptions.ts): null when the decision offers this card nothing,
     *  in which case the tile carries no options affordance and no mark. It
     *  is the SAME fact behind 'highlight cards with valid options' and
     *  'highlight valid targets', so a card that is a valid target and a card
     *  you may cast both get it — no kind is special-cased (R-E4-2). */
    tileOptions?: import('../lib/cardoptions').TileOptions | null;
    /** open0 seeds the menu's open/closed state, injectable for the repo's
     *  SSR test harness just as `hover`/`anchor` are: this environment has no
     *  DOM and no pointer events, so a test cannot click the badge to open
     *  the menu, and the options affordance's own state is what a test drives
     *  by hand the way it drives the detail panel. Production never passes it
     *  and the default is closed. */
    open0?: boolean;
  } = $props();

  // The options menu is this tile's own open/closed state: nothing on the
  // wire drives it and nothing outside reads it. The badge that opens it is
  // a real button in the tab order (keyboard reachable), the menu items are
  // the server's own option labels, and clicking one posts that option's own
  // index through tileOptions.post — which is the seat panel (R-E4-1: the
  // index, never a position in a rebuilt list).
  // svelte-ignore state_referenced_locally
  let open = $state(open0);

  // Ability shorthand for the keywords players scan for during combat. A
  // keyword with no shorthand is deliberately NOT drawn as a mark: an
  // unexplained two-letter code is worse than no code, and the full list is
  // one hover away on the card itself. Parameterised keywords ("Protection
  // from red") keep their head word, so they match on the head.
  const MARKS: Record<string, string> = {
    flying: 'FL', deathtouch: 'DT', 'double strike': 'DS', 'first strike': 'FS',
    lifelink: 'LL', trample: 'TR', vigilance: 'VG', menace: 'MN',
    reach: 'RC', hexproof: 'HX', indestructible: 'ID', haste: 'HA',
    protection: 'PRO', ward: 'WD',
  };
  const marks = $derived(
    (card.keywords ?? [])
      .map((k) => {
        const lower = k.toLowerCase();
        return MARKS[lower] ?? MARKS[lower.split(' ')[0]];
      })
      .filter((m): m is string => m !== undefined),
  );
  // Three marks is what fits across a 90px face at a countable size. Past
  // that the tile says how many more there are and the inspector spells them
  // all out; six unreadable codes stacked over the art help nobody.
  const MARK_CAP = 3;
  const shownMarks = $derived(marks.slice(0, MARK_CAP));
  const hiddenMarks = $derived(marks.length - shownMarks.length);
  const allMarks = $derived((card.keywords ?? []).join(', '));

  // Counters, sorted so a map's iteration order can never reach the rendered
  // output (determinism contract). Two letters, not one: P1P1 and a poison
  // counter must not both read as "P".
  const counters = $derived(
    Object.entries(card.counters ?? {}).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)),
  );

  const isCreature = $derived(card.types.includes('Creature'));
  const showStats = $derived(isCreature || card.damage > 0);

  // One hover state per tile: pointer dwell or keyboard focus opens the
  // detail; leave/blur/Escape close it. onOpen captures this tile's rect so
  // the fixed panel anchors where the card is. The tile is a tooltip
  // trigger in the ARIA sense (reveals extra info on hover/focus), which is
  // why the div carries role="button" + tabindex + aria-describedby; there
  // is deliberately no click activation.
  let root = $state<HTMLElement | null>(null);
  // The panel's anchor. In production only capture() sets it (from the tile's
  // rect once a pointer event arrives); a test injects one so the open panel
  // can be rendered without a DOM. initialAnchor reads the prop once, at
  // mount/SSR time, so the compiler never sees a prop read in a reactive
  // position — the injected anchor is a seed, not a stream.
  const initialAnchor = () => anchorProp ?? null;
  let anchor = $state<AnchorRect | null>(initialAnchor());

  // PANEL LIFETIME FOLLOWS THE OBJECT, NOT THE MOUNTING. When this tile's
  // object no longer exists (destroyed, bounced, merged into a stacked group)
  // the tile leaves the DOM under the pointer and pointerleave never fires
  // — and worse, when the board re-renders, Svelte can hand this same
  // instance a DIFFERENT card with no pointer event at all, silently
  // re-targeting the open panel to a card the reader never asked for (the
  // exact reported bug). So on every card change the tile re-feeds the id it
  // is now rendering and HoverCard closes a panel opened for another object
  // or re-points an armed dwell. superviseRendering is the rendering half of
  // the contract; supervise ("the object left the visible set") still serves
  // HandList, whose triggers are keyed per card and never re-used.
  $effect(() => {
    hover.superviseRendering(card.id);
  });

  function capture(): void {
    const r = root?.getBoundingClientRect();
    anchor = r ? { left: r.left, top: r.top, right: r.right } : null;
  }
</script>

<div class="tile-wrap">
<div
  class="card-tile card-tile--{size}"
  class:tapped={card.tapped}
  class:sick={card.summon_sick}
  class:attacking={card.attacking}
  data-obj={card.id}
  bind:this={root}
  tabindex="0"
  role="button"
  onpointerenter={() => hover.arm(card.id, capture)}
  onpointerleave={() => hover.close()}
  onfocus={() => hover.open(card.id, capture)}
  onblur={() => hover.close()}
  onkeydown={(e) => hover.keydown(e)}
  aria-describedby={hover.show ? `card-detail-${card.id}` : undefined}
>
  <div class="slot">
    <div class="face"><CardImage {card} {size} pt={false} /></div>

    {#if shownMarks.length}
      <div class="marks data" title={allMarks}>
        {#each shownMarks as m (m)}<span class="mark">{m}</span>{/each}
        {#if hiddenMarks > 0}<span class="mark mark--more">+{hiddenMarks}</span>{/if}
      </div>
    {/if}

    {#if counters.length || showStats}
      <div class="band">
        <div class="counters">
          {#each counters as [kind, n] (kind)}
            <span class="chip" title="{n} {kind}"><span class="chip__n data">{n}</span>{kind.slice(0, 2).toUpperCase()}</span>
          {/each}
        </div>
        {#if showStats}
          <div class="stats">
            {#if card.damage > 0}<span class="stats__dmg data" title="damage marked">{card.damage}</span>{/if}
            {#if isCreature}<span class="stats__pt data" title="current power/toughness">{card.power}/{card.toughness}</span>{/if}
          </div>
        {/if}
      </div>
    {/if}
  </div>

  {#if attachments.length}
    <div class="attached">
      {#each attachments as a (a.id)}
        <span class="rider" data-obj={a.id} title={a.name}>{a.name}</span>
      {/each}
    </div>
  {/if}
</div>

{#if tileOptions}
  <!-- The options affordance sits OUTSIDE the role="button" tile so a
       real button is never nested inside one. It is anchored to the tile's
       top-right corner — the one corner with no meaning yet (top-left is
       keyword marks, the bottom band is state), so the tile's signal
       hierarchy stays intact. -->
  <div class="tile-actions">
    <button
      class="badge"
      type="button"
      aria-haspopup="menu"
      aria-expanded={open}
      aria-label="{tileOptions.list.length} {tileOptions.list.length === 1 ? 'action' : 'actions'} for {card.name}"
      title="Options for {card.name}"
      onclick={() => (open = !open)}
    >
      <span class="badge__n data">{tileOptions.list.length}</span>
    </button>
    {#if open}
      <ul class="menu" role="menu" aria-label="Options for {card.name}">
        {#each tileOptions.list as opt (opt.index)}
          <li role="none">
            <button class="menu__item" type="button" role="menuitem" onclick={() => tileOptions.post(opt.index)}>
              {opt.label}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}
</div>

{#if hover.show && anchor}
  <CardDetail {card} anchor={anchor} />
{/if}

<style>
  /* The wrapper is one flow item in the row (it was the card-tile before:
     inline-block, so a row of cards is a row of tiles). The card-tile and
     any attached riders pack inside it exactly as they did, and the options
     affordance anchors to the wrapper so it can overlay the face without
     becoming a child of the role="button" tile. */
  .tile-wrap {
    position: relative;
    display: inline-block;
  }
  .card-tile {
    position: relative;
    display: inline-block;
  }
  .card-tile--tile {
    --w: var(--card-w, 90px);
  }
  .card-tile--large {
    --w: var(--card-w-large, 220px);
  }

  /* The slot is the space the permanent occupies on the table. Untapped that
     is the card; tapped it is the card turned a quarter, which is wider and
     shorter — so the slot changes shape too and a tapped permanent stops
     colliding with whatever is beside it. */
  .slot {
    position: relative;
    width: var(--w);
    aspect-ratio: 63 / 88;
    transition: width 0.14s ease-out;
  }
  .card-tile.tapped .slot {
    width: calc(var(--w) * 88 / 63);
    aspect-ratio: 88 / 63;
  }
  .face {
    position: absolute;
    top: 50%;
    left: 50%;
    line-height: 0;
    transform: translate(-50%, -50%) rotate(0deg);
    transform-origin: center;
    /* Motion answers the action: a permanent turns when it taps. */
    transition: transform 0.14s ease-out;
  }
  .card-tile.tapped .face {
    transform: translate(-50%, -50%) rotate(90deg);
  }
  /* Summoning sickness dims the card, not its numbers — the whole reason to
     look at a sick creature is to check whether it can attack yet. */
  .card-tile.sick .face {
    opacity: 0.78;
  }
  /* Attacking is a red rim on the permanent itself, the same relationship the
     red arrow draws (survey #16). */
  .card-tile.attacking .slot::after {
    content: '';
    position: absolute;
    inset: 0;
    border-radius: var(--radius-card);
    box-shadow: inset 0 0 0 2px var(--danger);
    pointer-events: none;
  }

  /* Top left: keyword marks, as one band rather than scattered ink. */
  .marks {
    position: absolute;
    top: 2px;
    left: 2px;
    display: flex;
    flex-wrap: wrap;
    gap: 1px;
    max-width: calc(100% - 4px);
  }
  .mark {
    background: color-mix(in srgb, var(--felt-sunk) 86%, transparent);
    color: var(--ink-dim);
    font-size: var(--t-10);
    line-height: 1.35;
    letter-spacing: -0.03em;
    padding: 0 2px;
    border-radius: 2px;
  }
  .mark--more {
    color: var(--ink-faint);
  }

  /* The bottom band: everything that has happened to this object, on one
     line, so a combat is one glance and not four. */
  .band {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 2px;
  }
  /* A counter is a modification the player must SEE, not a number silently
     folded into power/toughness, so it is the one filled saturated mark on
     the face. */
  .counters {
    display: flex;
    flex-wrap: wrap;
    gap: 1px;
    padding: 0 0 2px 2px;
  }
  .chip {
    display: inline-flex;
    align-items: baseline;
    gap: 0.3em;
    background: var(--initiative);
    color: var(--felt-sunk);
    font-size: var(--t-10);
    font-weight: 600;
    line-height: 1.35;
    letter-spacing: -0.03em;
    border-radius: 2px;
    padding: 0 2px;
  }
  .chip__n {
    font-size: inherit;
    font-weight: 600;
  }

  .stats {
    flex: none;
    display: flex;
    align-items: stretch;
    background: color-mix(in srgb, var(--felt-sunk) 90%, transparent);
    border-top: 1px solid var(--edge-felt);
    border-left: 1px solid var(--edge-felt);
    border-radius: var(--radius) 0 var(--radius-card) 0;
    overflow: hidden;
    font-size: var(--t-11);
    line-height: 1.45;
  }
  .stats__dmg {
    font-size: inherit;
    background: var(--danger);
    color: var(--mana-w);
    font-weight: 500;
    padding: 0 0.35em;
  }
  .stats__pt {
    font-size: inherit;
    color: var(--ink);
    font-weight: 500;
    padding: 0 0.35em;
  }

  /* An attachment rides UNDER its host, overlapping slightly, so the pair
     reads as one object without hiding either. */
  .attached {
    display: flex;
    flex-direction: column;
    gap: 1px;
    margin-top: -0.35em;
    padding-left: var(--sp-2);
  }
  .rider {
    background: var(--felt-sunk);
    border-left: 2px solid var(--edge-felt);
    color: var(--ink-dim);
    font-size: var(--t-10);
    line-height: 1.6;
    padding: 0 0.35em;
    max-width: var(--w);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* An option affordance: a small plate on the tile's free top-right corner
     carrying the number of things the pending decision lets you do here,
     and a menu of the server's own option labels. The plate is a real
     button — keyboard reachable, and it names the card it belongs to — and
     the menu items are the option labels verbatim. The tone rule on the
     plate's inner edge is applied by the board-marking layer (see the
     marking commit); its neutral state is the instrument ink. */
  .tile-actions {
    position: absolute;
    top: 1px;
    right: 1px;
    z-index: 3;
    line-height: 1;
  }
  .badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 1.1rem;
    height: 1.1rem;
    padding: 0 0.25rem;
    border-radius: 3px;
    border: 1px solid var(--edge-inst);
    background: var(--instrument);
    color: var(--ink);
    font-family: var(--font-data);
    font-size: var(--t-10);
    font-weight: 600;
    cursor: pointer;
  }
  .badge__n {
    font-size: inherit;
  }
  .badge:hover,
  .badge[aria-expanded='true'] {
    border-color: var(--ink-dim);
    color: var(--ink);
  }
  .menu {
    position: absolute;
    top: calc(100% + 3px);
    right: 0;
    z-index: 6;
    margin: 0;
    padding: 2px;
    list-style: none;
    min-width: 9rem;
    max-width: 14rem;
    max-height: 12rem;
    overflow-y: auto;
    background: var(--instrument);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    box-shadow: var(--shadow-lift);
  }
  .menu__item {
    display: block;
    width: 100%;
    text-align: left;
    background: none;
    border: 0;
    border-left: 2px solid transparent;
    border-radius: 0;
    color: var(--ink-inst);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    line-height: 1.35;
    padding: var(--sp-1) var(--sp-2);
    cursor: pointer;
  }
  .menu__item:hover,
  .menu__item:focus-visible {
    background: color-mix(in srgb, var(--ink) 7%, var(--instrument));
    border-left-color: var(--ink-dim);
    color: var(--ink);
  }
</style>
