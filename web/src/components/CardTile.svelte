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
  let { card, size = 'tile', attachments = [] }: { card: CardView; size?: 'tile' | 'large'; attachments?: CardView[] } = $props();

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
  const hover = new HoverCard();
  let root = $state<HTMLElement | null>(null);
  let anchor = $state<AnchorRect | null>(null);

  function capture(): void {
    const r = root?.getBoundingClientRect();
    anchor = r ? { left: r.left, top: r.top, right: r.right } : null;
  }
</script>

<div
  class="card-tile card-tile--{size}"
  class:tapped={card.tapped}
  class:sick={card.summon_sick}
  class:attacking={card.attacking}
  data-obj={card.id}
  bind:this={root}
  tabindex="0"
  role="button"
  onpointerenter={() => hover.arm(capture)}
  onpointerleave={() => hover.close()}
  onfocus={() => hover.open(capture)}
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

{#if hover.show && anchor}
  <CardDetail {card} anchor={anchor} />
{/if}

<style>
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
</style>
