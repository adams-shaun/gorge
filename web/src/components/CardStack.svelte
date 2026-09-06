<script lang="ts">
  import type { CardView } from '../protocol';
  import { stackFaces, type CardStackGroup } from '../lib/board';
  import CardTile from './CardTile.svelte';

  /**
   * CardStack renders one stackIdentical group: a bare CardTile for a group
   * of one — pixel-for-pixel what the quadrant rendered before stacking — or
   * a single tile with a shallow fan and a count badge for a group of many.
   * Activating a stacked group expands it in place to every member;
   * activating again collapses. That state is this component's alone: nothing
   * on the wire drives it, and nothing outside reads it. Attachments only
   * ever reach a group of one — a permanent with an attachment is individual
   * by definition and never stacks — so the fan has no riders to compose.
   *
   * THE PILE. Six identical Islands are one pile of cards on a table, and a
   * pile of cards is square: offset, not splayed. The two ghost layers step
   * up and to the left by a fixed amount, exposing a spine along the left
   * edge, and the count sits ON that spine like the tab on a pile you have
   * squared up — not floating in a corner as a badge. Which is also why the
   * spine is on the left: every other corner of the tile already means
   * something (keyword marks, counters, the combat numbers), and the gutter
   * outside the card's left edge is the only place left that is the pile's
   * own rather than the card's.
   *
   * THE AFFORDANCE. Pointing at the pile pushes the ghosts further out and
   * warms the tab: the gesture the tile is about to make, made small first.
   * That is the discoverability — motion answering a pointer, not an ambient
   * animation. Deliberately NOT a title tooltip: pointing at the pile already
   * opens the card inspector, and a second floating box over it is noise.
   *
   * The first card keeps its data-obj anchor (on the CardTile itself), and
   * the group's every member id is on `data-obj-group` so a later task can
   * draw arrows to each member of a stacked tile.
   */
  let { group, size = 'tile', attachments = [] }: { group: CardStackGroup; size?: 'tile' | 'large'; attachments?: CardView[] } = $props();

  let expanded = $state(false);
  const faces = $derived(stackFaces(group, expanded));
  const accessibleLabel = $derived(
    expanded
      ? `${group.cards.length} copies of ${group.cards[0].name}, expanded`
      : `${group.cards.length} copies of ${group.cards[0].name}`,
  );
  const memberIds = $derived(group.cards.map((c) => c.id).join(','));
</script>

{#if group.cards.length === 1}
  <CardTile card={group.cards[0]} {size} {attachments} />
{:else}
  <button
    type="button"
    class="stacked"
    class:expanded
    data-obj-group={memberIds}
    aria-expanded={expanded}
    aria-label={accessibleLabel}
    onclick={() => (expanded = !expanded)}
  >
    {#if !expanded}
      <!-- the fan is exactly two ghost layers, never one per card: a fan of
           40 tiles is noise, and two edges say "stack" in one glance. -->
      <span class="ghost ghost--2" aria-hidden="true"></span>
      <span class="ghost ghost--1" aria-hidden="true"></span>
    {/if}
    {#each faces as c (c.id)}
      <CardTile card={c} {size} />
    {/each}
    <span class="count" data-stack-count aria-hidden="true">x{group.cards.length}</span>
  </button>
{/if}

<style>
  /* The whole group is one focusable control; the first tile inside it is
     still the data-obj anchor CardTile already drew. */
  .stacked {
    --spine: 5px;
    position: relative;
    display: inline-flex;
    align-items: flex-start;
    gap: var(--sp-2);
    /* room for the spine and for the tab that hangs off it, so the pile never
       leans into whatever is beside it in the row */
    margin-left: var(--sp-6);
    padding: 0;
    border: none;
    border-radius: var(--radius-card);
    background: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }
  /* Ghosts trace the face's own box in both states — the CardTile slot
     changes shape when a permanent is tapped, and inset:0 follows it — so a
     pile of tapped lands is still a pile and not two loose rectangles. */
  .ghost {
    position: absolute;
    inset: 0;
    background: var(--felt-sunk);
    /* The lit edges are the ones the pile exposes, so the step reads even
       when the top card is a blank in nearly the same colour. */
    border: 1px solid var(--edge-felt);
    border-top-color: var(--ink-faint);
    border-left-color: var(--ink-faint);
    border-radius: var(--radius-card);
    transition: transform 0.12s ease-out;
  }
  .ghost--1 {
    transform: translate(calc(var(--spine) * -1), calc(var(--spine) * -0.75));
  }
  .ghost--2 {
    background: var(--felt);
    border-top-color: var(--ink-dim);
    border-left-color: var(--ink-dim);
    transform: translate(calc(var(--spine) * -2), calc(var(--spine) * -1.5));
  }
  .stacked:hover .ghost--1,
  .stacked:focus-visible .ghost--1 {
    transform: translate(calc(var(--spine) * -1.75), calc(var(--spine) * -1.5));
  }
  .stacked:hover .ghost--2,
  .stacked:focus-visible .ghost--2 {
    transform: translate(calc(var(--spine) * -3.5), calc(var(--spine) * -3));
  }

  /* Open, the members are a group rather than N loose tiles that happen to
     match, so the set keeps one rule around it and the tab stays put. */
  .stacked.expanded {
    padding: var(--sp-1);
    margin-left: var(--sp-6);
    box-shadow: inset 0 0 0 1px var(--edge-felt);
  }

  /* The tab: a real plate on the spine, in the instrument's voice because a
     count is a value. It is the only thing on the pile that is not a card. */
  .count {
    position: absolute;
    /* Hung entirely in the gutter, off the pile's exposed edge: the card's
       own bottom-left corner belongs to the counters band. translateX keeps
       it outside whatever width the number needs. */
    left: calc(var(--spine) * -2);
    bottom: var(--sp-2);
    transform: translateX(-100%);
    background: var(--felt-sunk);
    color: var(--ink);
    border: 1px solid var(--edge-felt);
    border-radius: 2px;
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    font-size: var(--t-11);
    font-weight: 500;
    line-height: 1.4;
    letter-spacing: -0.02em;
    padding: 0 0.35em;
    transition: border-color 0.12s ease-out, color 0.12s ease-out;
  }
  .stacked.expanded .count {
    left: calc(var(--sp-1) * -1);
    bottom: auto;
    top: calc(var(--sp-1) * -1);
  }
  .stacked:hover .count,
  .stacked:focus-visible .count {
    border-color: var(--initiative);
    color: var(--initiative);
  }
</style>
