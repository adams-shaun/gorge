<script lang="ts">
  import type { CardView, PlayerView } from '../protocol';
  import { visibleHand } from '../lib/board';
  import ManaSymbols from './ManaSymbols.svelte';
  import CardDetail from './CardDetail.svelte';
  import { HoverCard, type AnchorRect } from '../lib/carddetail.svelte';

  /**
   * HandList is a text list of one player's revealed hand: one line per card, name plus mana symbols, headed with that seat's colour and deck. Rail only mounts this when visibleHand(player) is non-null, but it guards independently too, in case that ever changes. Each line is also a hover/focus target for the same CardDetail panel the board tiles show — one shared hover state, since only one line can be under the pointer or focused at a time.
   */
  let { player, deck, colour }: { player: PlayerView; deck?: string; colour: string } = $props();

  const hand = $derived(visibleHand(player) ?? []);

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
  <h3>{player.name}'s hand{#if deck} <span class="deck">· {deck}</span>{/if}</h3>
  {#if hand.length === 0}
    <p class="empty">empty</p>
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
  .hand { margin-bottom: .75rem; border-left: 3px solid transparent; padding-left: .5rem; }
  h3 { margin: 0 0 .25rem; font-size: .8rem; opacity: .8; font-weight: 600; }
  .deck { font-weight: 400; opacity: .8; }
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: .2rem; }
  li { display: flex; }
  /* The focusable trigger lives inside the li so the li itself stays a pure
     data-obj anchor for arrows. tooltip-trigger role: reveals extra info on
     hover/focus, no click activation. */
  .hand-trigger {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: .5rem;
    width: 100%;
    font-size: .8rem;
    border-radius: 3px;
  }
  .hand-trigger:focus-visible { outline: 2px solid var(--initiative); outline-offset: 1px; }
  .empty { margin: 0; font-size: .8rem; opacity: .5; }
</style>
