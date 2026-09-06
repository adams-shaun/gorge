<script lang="ts">
  import type { StackView, View, TargetView } from '../protocol';
  import { visibleHand } from '../lib/board';
  import CardImage from './CardImage.svelte';

  /** StackTile is one stack entry: a band coloured by kind, its card face when it has one, its text, and its targets. Resolving a target's object id to a display name is a rendering concern — it reads every visible zone in `view` but decides nothing about the game. emphasized/dimmed express survey item 10 (the top of the stack reads by contrast): the seat view passes emphasized for the top entry and dimmed for the rest; the spectator path renders them unset, unchanged. */
  let { stack, view, emphasized = false, dimmed = false }: { stack: StackView; view: View; emphasized?: boolean; dimmed?: boolean } = $props();

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

  function targetLabel(t: TargetView): string {
    const who = t.is_player ? `Seat ${t.player}` : t.obj !== undefined ? (nameFor(t.obj) ?? `#${t.obj}`) : `Seat ${t.player}`;
    return `→ ${t.label ?? 'target'}: ${who}`;
  }
</script>

<div class="stack-tile kind-{stack.kind}" class:emphasized class:dimmed data-obj={stack.id}>
  {#if stack.card}<CardImage card={stack.card} />{/if}
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
  .stack-tile { display: flex; gap: .5rem; padding: .4rem; border-radius: 6px; background: #1b1b1f; border-left: 4px solid #666; margin-bottom: .4rem; }
  .stack-tile.emphasized { background: #262a36; box-shadow: 0 0 0 1px #3b82f6 inset; }
  .stack-tile.dimmed { opacity: .55; }
  .stack-tile.kind-spell { border-left-color: #3b82f6; }
  .stack-tile.kind-ability { border-left-color: #22c55e; }
  .stack-tile.kind-trigger { border-left-color: #eab308; }
  .info { min-width: 0; flex: 1; }
  header { display: flex; gap: .4rem; align-items: baseline; }
  .kind { font-size: .6rem; text-transform: uppercase; opacity: .6; }
  .name { font-weight: 600; font-size: .85rem; }
  .text { margin: .2rem 0; font-size: .75rem; opacity: .85; }
  .targets { margin: .2rem 0 0; padding: 0; list-style: none; font-size: .7rem; opacity: .8; }
</style>
