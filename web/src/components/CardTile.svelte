<script lang="ts">
  import type { CardView } from '../protocol';
  import CardImage from './CardImage.svelte';

  /**
   * CardTile is the battlefield/stack/strip face of one object. It has no
   * rules knowledge: CardImage draws the face (art, or the text fallback with
   * name/mana symbols/types/P-T); this wrapper only adds per-instance state
   * that is not part of the card's printed identity — tapped, damage,
   * counters, summoning sickness, attachments — plus the data-obj anchor
   * arrows use.
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
</script>

<div class="card-tile" class:tapped={card.tapped} class:sick={card.summon_sick} data-obj={card.id}>
  <CardImage {card} {size} />
  {#if card.damage > 0}<span class="damage" title="damage marked">{card.damage}</span>{/if}
  {#if marks.length}
    <div class="marks">
      {#each marks as m (m)}<span class="mark">{m}</span>{/each}
    </div>
  {/if}
  {#if card.counters && Object.keys(card.counters).length}
    <div class="counters">
      {#each Object.entries(card.counters) as [kind, n] (kind)}
        <span class="chip" title="{n} {kind}">{n}{kind.slice(0, 1)}</span>
      {/each}
    </div>
  {/if}
  {#if attachments.length}
    <div class="attached">
      {#each attachments as a (a.id)}
        <span class="rider" data-obj={a.id} title={a.name}>{a.name}</span>
      {/each}
    </div>
  {/if}
</div>

<style>
  .card-tile {
    position: relative;
    display: inline-block;
  }
  /* Tapped reads as tapped: a real rotation, not a nudge. The slot keeps its
     footprint so a board does not reflow every time something taps. */
  .card-tile.tapped {
    transform: rotate(28deg);
    transition: transform 0.12s ease-out;
  }
  .card-tile.sick {
    opacity: 0.62;
  }
  .damage {
    position: absolute;
    top: -0.4em;
    right: -0.4em;
    background: var(--danger);
    color: var(--ink);
    border-radius: 999px;
    font-family: var(--font-data);
    font-size: 0.625rem;
    font-weight: 500;
    padding: 0.1em 0.4em;
    line-height: 1.3;
  }
  /* Keyword marks sit top-left, where they do not collide with damage. Two
     letters is enough to scan a combat; the title carries the full word. */
  .marks {
    position: absolute;
    top: 0.15em;
    left: 0.15em;
    display: flex;
    flex-wrap: wrap;
    gap: 1px;
    max-width: 70%;
  }
  .mark {
    background: color-mix(in srgb, var(--felt-sunk) 82%, transparent);
    color: var(--ink-dim);
    font-family: var(--font-data);
    font-size: 0.5625rem;
    line-height: 1.4;
    padding: 0 0.25em;
    border-radius: 2px;
  }
  .counters {
    position: absolute;
    bottom: -0.35em;
    left: 0.25em;
    display: flex;
    gap: 0.15em;
  }
  /* A counter is a modification the player must SEE, not a number silently
     folded into power/toughness — so it is marked in the initiative colour
     rather than in neutral chrome. */
  .chip {
    background: var(--initiative);
    color: var(--felt-sunk);
    font-family: var(--font-data);
    font-size: 0.5625rem;
    font-weight: 500;
    border-radius: 2px;
    padding: 0 0.3em;
    line-height: 1.45;
  }
  /* An attachment rides UNDER its host, overlapping slightly, so the pair
     reads as one object without hiding either. */
  .attached {
    display: flex;
    flex-direction: column;
    gap: 1px;
    margin-top: -0.35em;
    padding-left: 0.5em;
  }
  .rider {
    background: var(--felt-sunk);
    border-left: 2px solid var(--mana-c);
    color: var(--ink-dim);
    font-size: 0.625rem;
    line-height: 1.5;
    padding: 0 0.35em;
    max-width: 7rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
