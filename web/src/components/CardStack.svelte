<script lang="ts">
  import type { CardView } from '../protocol';
  import { stackFaces, type CardStackGroup } from '../lib/board';
  import CardTile from './CardTile.svelte';

  /**
   * CardStack renders one stackIdentical group: a bare CardTile for a group
   * of one — pixel-for-pixel what the quadrant rendered before stacking — or
   * a single tile with a shallow two-layer fan and a count badge for a group
   * of many. Activating a stacked group expands it in place to every member;
   * activating again collapses. That state is this component's alone: nothing
   * on the wire drives it, and nothing outside reads it. Attachments only
   * ever reach a group of one — a permanent with an attachment is individual
   * by definition and never stacks — so the fan has no riders to compose.
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
    {#if !expanded}
      <span class="count" data-stack-count aria-hidden="true">x{group.cards.length}</span>
    {/if}
  </button>
{/if}

<style>
  /* The whole group is one focusable control; the first tile inside it is
     still the data-obj anchor CardTile already drew. The count badge sits
     bottom-right, clear of the damage badge (top-right) and keyword marks
     (top-left). */
  .stacked {
    position: relative;
    display: inline-flex;
    align-items: flex-start;
    gap: var(--sp-2);
    padding: 0;
    border: none;
    border-radius: var(--radius-card);
    background: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }
  /* Ghosts mirror the face's own aspect ratio and the containing block's
     width, so the fan is the same shape whether the tile is 'tile' or
     'large' without duplicating either size's dimensions here. */
  .ghost {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    aspect-ratio: 63 / 88;
    background: var(--felt-sunk);
    border: 1px solid var(--edge-felt);
    border-radius: var(--radius-card);
  }
  .ghost--1 {
    transform: translate(-0.35em, 0.3em) rotate(-6deg);
  }
  .ghost--2 {
    transform: translate(0.2em, 0.15em) rotate(7deg);
  }
  .count {
    position: absolute;
    right: -0.4em;
    bottom: -0.4em;
    background: var(--felt-sunk);
    color: var(--ink);
    border: 1px solid var(--edge-felt);
    border-radius: 2px;
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    font-size: 0.625rem;
    font-weight: 500;
    line-height: 1.45;
    padding: 0.05em 0.4em;
  }
</style>
