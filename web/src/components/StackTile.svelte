<script lang="ts">
  import type { StackView, View, TargetView } from '../protocol';
  import { visibleHand } from '../lib/board';
  import CardImage from './CardImage.svelte';
  import CardDetail from './CardDetail.svelte';
  import { HoverCard, type AnchorRect } from '../lib/carddetail.svelte';

  /**
   * StackTile is one stack entry: a band coloured by kind, its card face
   * when it has one, its text, and its targets. Resolving a target's object
   * id to a display name is a rendering concern — it reads every visible
   * zone in `view` but decides nothing about the game. emphasized/dimmed
   * express survey item 10 (the top of the stack reads by contrast): the
   * seat view passes emphasized for the top entry and dimmed for the rest;
   * the spectator path renders them unset, unchanged.
   *
   * Density (U-rail-2, "we can hardly see 1 card"): the art is drawn at a
   * fixed, smaller `--card-w` than the board's own tiles — a stack entry is
   * read for identity and targets, not inspected at battlefield size, and
   * the large read is one hover away (below) — and the oracle-ish text line
   * is clamped to two lines rather than running to a paragraph, so several
   * entries fit in the stack's now-larger, but still finite, flex share.
   *
   * ART FALLBACK (Task 2a): `stack.card` is nil for a trigger or activated
   * ability today (an ability object has no card face — view/view.go), a gap
   * another task is closing on the Go side by falling back to the source
   * permanent's own card view. This component's job is only to use it
   * whenever it is there and do nothing when it is not: `{#if stack.card}`
   * already does exactly that, and needs no change here to pick up the Go
   * fix — a stack entry with no card face simply keeps rendering as it does
   * today, text only, no broken image, no placeholder box.
   *
   * HOVER (Task 2b): a stack card could not be inspected the way a
   * battlefield card can — no hover at all was wired. Wired here to the same
   * HoverCard dwell/focus/CardDetail mechanism CardTile and HandList use.
   * `hover.supervise` is the lifecycle half of that contract (carddetail.
   * svelte.ts): it takes the object id the panel is CURRENTLY showing and an
   * INDEPENDENT list of the objects still present, and closes the panel when
   * the id is no longer in that list — e.g. this entry's card leaves the
   * stack (resolves, is countered) while the pointer is still over it, which
   * never fires pointerleave. The list passed here is every card CURRENTLY
   * on the stack across every entry (`view.stack`), read fresh off `view` —
   * not `[stack.card]`, which would make the check ask whether stack.card's
   * id is in a list containing only stack.card's id: always true, the exact
   * tautology CardTile once shipped and that this rail must not repeat.
   */
  // hover and anchor are injectable so the repo's SSR test harness (no DOM,
  // no pointer events, no $effect) can drive the panel's lifecycle through
  // the same HoverCard the component owns — mirrors CardTile.svelte exactly;
  // production renders never pass them.
  let {
    stack,
    view,
    emphasized = false,
    dimmed = false,
    hover = new HoverCard(),
    anchor: anchorProp = null,
  }: {
    stack: StackView; view: View; emphasized?: boolean; dimmed?: boolean;
    hover?: HoverCard; anchor?: AnchorRect | null;
  } = $props();

  function nameFor(obj: number): string | null {
    for (const p of view.players) {
      for (const list of [p.battlefield, visibleHand(p) ?? [], p.graveyard, p.exile]) {
        const c = list.find((x) => x.id === obj);
        if (c) return c.name;
      }
    }
    for (const s of view.stack) if (s.card?.id === obj) return s.card.name;
    return null;
  }

  function playerName(seat: number): string {
    // The wire's player name is preferred over the 0-based `Seat N`
    // placeholder, exactly as IdentityBar does (the live table route can
    // hand an empty `seats` while view.players still carries the names).
    return view.players.find((p) => p.seat === seat)?.name ?? `Seat ${seat}`;
  }

  function targetLabel(t: TargetView): string {
    const who = t.is_player ? playerName(t.player) : t.obj !== undefined ? (nameFor(t.obj) ?? `#${t.obj}`) : playerName(t.player);
    return `→ ${t.label ?? 'target'}: ${who}`;
  }

  let root = $state<HTMLElement | null>(null);
  // Seeded from the prop (a test's injected rect) at mount/SSR time only —
  // in production only capture() ever sets it, from a real pointer event.
  const initialAnchor = () => anchorProp ?? null;
  let anchor = $state<AnchorRect | null>(initialAnchor());

  // Every card currently on the stack, independent of which entry (if any)
  // this tile's own hover is showing — the "present" list supervise needs.
  const stackCards = $derived(view.stack.map((s) => s.card).filter((c): c is NonNullable<typeof c> => c != null));

  // Re-checked whenever `view` changes (stackCards depends on it): if the
  // card this tile is showing the panel for has left the stack, close it.
  // Cannot run under the repo's SSR component tests (no $effect there, see
  // StackTile.svelte.test.ts) — the mutation test in that file exercises
  // hover.supervise directly instead, the same way CardTile.svelte.test.ts
  // does for superviseRendering.
  $effect(() => {
    if (stack.card) hover.supervise(stack.card.id, stackCards);
  });

  function capture(): void {
    const r = root?.getBoundingClientRect();
    anchor = r ? { left: r.left, top: r.top, right: r.right } : null;
  }
</script>

<div class="stack-tile kind-{stack.kind}" class:emphasized class:dimmed data-obj={stack.id}>
  {#if stack.card}
    {@const card = stack.card}
    <div
      class="art"
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
      <CardImage {card} />
    </div>
    {#if hover.show && anchor}<CardDetail {card} {anchor} />{/if}
  {/if}
  <div class="info">
    <header>
      <span class="kind">{stack.kind}</span>
      <span class="name">{stack.name}</span>
    </header>
    {#if stack.text}<p class="text">{stack.text}</p>{/if}
    {#if stack.targets.length}
      <ul class="targets">
        {#each stack.targets as t, i (i)}<li>{targetLabel(t)}</li>{/each}
      </ul>
    {/if}
  </div>
</div>

<style>
  .stack-tile { display: flex; gap: .5rem; padding: .3rem .4rem; border-radius: 6px; background: #1b1b1f; border-left: 4px solid #666; margin-bottom: .3rem; --card-w: 56px; }
  .stack-tile.emphasized { background: #262a36; box-shadow: 0 0 0 1px #3b82f6 inset; }
  .stack-tile.dimmed { opacity: .55; }
  .stack-tile.kind-spell { border-left-color: #3b82f6; }
  .stack-tile.kind-ability { border-left-color: #22c55e; }
  .stack-tile.kind-trigger { border-left-color: #eab308; }
  .art { flex: none; cursor: default; }
  .art:focus-visible { outline: 2px solid var(--initiative); outline-offset: 1px; }
  .info { min-width: 0; flex: 1; }
  header { display: flex; gap: .4rem; align-items: baseline; }
  .kind { font-size: .6rem; text-transform: uppercase; opacity: .6; }
  .name { font-weight: 600; font-size: .8rem; }
  /* Clamped rather than run to full length (U-rail-2): several entries have
     to fit in the stack's own share of the rail, and the full text is one
     hover away on the card itself. */
  .text {
    margin: .15rem 0;
    font-size: .7rem;
    opacity: .85;
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    overflow: hidden;
  }
  .targets { margin: .15rem 0 0; padding: 0; list-style: none; font-size: .68rem; opacity: .8; }
</style>
