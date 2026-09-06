<script lang="ts">
  import type { CardView, PlayerView } from '../protocol';
  import { visibleHand } from '../lib/board';
  import ManaSymbols from './ManaSymbols.svelte';
  import CardDetail from './CardDetail.svelte';
  import { HoverCard, type AnchorRect } from '../lib/carddetail.svelte';

  /**
   * HandList is a text list of one player's revealed hand: one line per card, name plus mana symbols, headed with that seat's colour and deck. Text, not card images, because four hands only fit in a rail as text (survey #24) and a reader who knows the format only needs the name. Rail only mounts this when visibleHand(player) is non-null, but it guards independently too, in case that ever changes. Each line is also a hover/focus target for the same CardDetail panel the board tiles show — one shared hover state, since only one line can be under the pointer or focused at a time.
   */
  let { player, deck, colour }: { player: PlayerView; deck?: string; colour: string } = $props();

  const hand = $derived(visibleHand(player) ?? []);
  // The deck line is dropped when it would only repeat the player's own name,
  // which is what the local fixture's seats do.
  const subtitle = $derived(deck && deck !== player.name ? deck : null);

  const hover = new HoverCard();
  let hovered = $state<CardView | null>(null);
  let anchor = $state<AnchorRect | null>(null);

  function rectOf(el: HTMLElement): AnchorRect {
    const r = el.getBoundingClientRect();
    return { left: r.left, top: r.top, right: r.right };
  }
  function armFor(c: CardView, el: HTMLElement) {
    hovered = c;
    hover.arm(() => {
      anchor = rectOf(el);
    });
  }
  function openFor(c: CardView, el: HTMLElement) {
    hovered = c;
    hover.open(() => {
      anchor = rectOf(el);
    });
  }
</script>

<section class="hand" style:border-left-color={colour}>
  <h3>{player.name}'s hand</h3>
  {#if subtitle}<p class="deck">{subtitle}</p>{/if}
  {#if hand.length === 0}
    <p class="empty">No cards</p>
  {:else}
    <ul>
      {#each hand as c (c.id)}
        <li data-obj={c.id}>
          <span
            class="hand-trigger"
            role="button"
            tabindex="0"
            onpointerenter={(e) => armFor(c, e.currentTarget)}
            onpointerleave={() => hover.close()}
            onfocus={(e) => openFor(c, e.currentTarget)}
            onblur={() => hover.close()}
            onkeydown={(e) => hover.keydown(e)}
            aria-describedby={hover.show && hovered === c ? `card-detail-${c.id}` : undefined}
          >
            <span class="name">{c.name}</span>
            {#if c.mana_cost}<ManaSymbols cost={c.mana_cost} />{/if}
          </span>
        </li>
      {/each}
    </ul>
    {#if hover.show && hovered && anchor}
      <CardDetail card={hovered} anchor={anchor} />
    {/if}
  {/if}
</section>

<style>
  .hand {
    margin-bottom: var(--sp-3);
    border-left: 3px solid transparent;
    padding-left: var(--sp-2);
  }
  /* Whose hand, then which deck, in the same two-line shape the identity bar
     uses for the same two facts — one vocabulary, learned once. The deck was
     previously joined on with a middle dot, which the design system does not
     use and which read as "· eldrazi-stompy" with no space when the name ran
     up against it. */
  h3 {
    margin: 0;
    font-size: var(--t-12);
    font-weight: 600;
    color: var(--ink-inst);
    line-height: 1.3;
  }
  .deck {
    margin: 0;
    font-size: var(--t-10);
    color: var(--ink-faint);
  }
  ul {
    list-style: none;
    margin: var(--sp-1) 0 0;
    padding: 0;
    display: flex;
    flex-direction: column;
  }
  li {
    display: flex;
  }
  /* The focusable trigger lives inside the li so the li itself stays a pure
     data-obj anchor for arrows. tooltip-trigger role: reveals extra info on
     hover/focus, no click activation. Hovering a line opens a panel the size
     of a card, so the line has to look like something that answers a pointer
     — it previously gave no feedback at all. */
  .hand-trigger {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--sp-2);
    width: 100%;
    padding: 1px var(--sp-1);
    font-size: var(--t-12);
    line-height: 1.5;
    border-radius: var(--radius);
    cursor: default;
  }
  .hand-trigger:hover,
  .hand-trigger:focus-visible {
    background: var(--instrument-raised);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .hand-trigger:focus-visible {
    outline: 2px solid var(--initiative);
    outline-offset: 1px;
  }
  .empty {
    margin: var(--sp-1) 0 0;
    font-size: var(--t-12);
    color: var(--ink-faint);
  }
</style>
