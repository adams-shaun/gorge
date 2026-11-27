<script lang="ts">
  import type { Decision, Option, View } from '../../protocol';
  import type { SeatPanelState } from '../../lib/seatpanel.svelte';
  import { findCardAnywhere } from '../../lib/board';
  import { remaining } from '../../lib/prompts/divide';
  import { hoverLinkOf, promptHover } from '../../lib/prompts/hover.svelte';
  import PromptFooter, { type FooterButton } from './PromptFooter.svelte';

  /**
   * DividePrompt is the `divide` renderer: a target ask whose spell divides a
   * total among its targets (DividedAsYouChoose$). Every legal target is a row
   * with a - / + stepper; a target at 0 is not targeted. Submit posts the
   * targets, and the seat panel answers the engine's follow-up division ask
   * from the same counts, so the player sees one prompt for both rules steps.
   */
  let { decision, view, logic }: { decision: Decision; view: View; logic: SeatPanelState } = $props();

  const left = $derived(remaining(decision, logic.divideCounts));

  function detailOf(o: Option): string | null {
    if (o.obj === undefined || o.obj === 0) {
      const p = view.players.find((x) => x.seat === o.player);
      return p ? `${p.life} life` : null;
    }
    const card = findCardAnywhere(view, o.obj);
    return card && /\bCreature\b/.test(card.types) ? `${card.power}/${card.toughness}` : null;
  }

  const footer = $derived.by((): FooterButton[] => [
    { label: 'Divide', onclick: () => logic.submitDivide(), disabled: !logic.canSubmitDivision || logic.busy, primary: true, data: { 'data-submit': true } },
  ]);
</script>

<div class="divide" data-divide-prompt>
  <div class="left" data-divide-left aria-live="polite">{left} left to assign</div>
  <ul class="rows" data-options>
    {#each decision.options as opt (opt.index)}
      {@const n = logic.divideCounts[opt.index] ?? 0}
      {@const link = hoverLinkOf(opt)}
      {@const detail = detailOf(opt)}
      <li class="row" class:on={n > 0} data-option={opt.index} onpointerenter={() => link && promptHover.set(link)} onpointerleave={() => promptHover.clear()}>
        <span class="name">{opt.label}</span>
        {#if detail}<small class="detail">{detail}</small>{/if}
        <button type="button" class="step" data-divide-minus aria-label={`Fewer to ${opt.label}`} disabled={logic.busy || !logic.canStepDivideDown(opt.index)} onclick={() => logic.stepDivide(opt.index, -1)}>−</button>
        <span class="count" data-divide-count>{n}</span>
        <button type="button" class="step" data-divide-plus aria-label={`More to ${opt.label}`} disabled={logic.busy || !logic.canStepDivideUp(opt.index)} onclick={() => logic.stepDivide(opt.index, 1)}>+</button>
      </li>
    {/each}
  </ul>
  <PromptFooter status={left === 0 ? 'All assigned' : `${left} left to assign`} buttons={footer} />
</div>

<style>
  .divide { display: flex; flex-direction: column; gap: var(--sp-2); width: 100%; }
  .left { font: 600 var(--t-12)/1.3 var(--font-ui); color: var(--gilt, #d4ad62); }
  .rows { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; max-height: 16rem; overflow-y: auto; }
  .row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 4px 8px;
    border-radius: 8px;
    background: var(--instrument-raised);
    border: 1px solid var(--edge-inst);
    color: var(--ink);
    font: 500 var(--t-12)/1.3 var(--font-ui);
  }
  .row.on { border-color: var(--gilt, #d4ad62); }
  .name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .detail { color: var(--ink-dim); font-size: var(--t-11); white-space: nowrap; }
  .step {
    width: 1.8rem;
    height: 1.8rem;
    border-radius: 50%;
    border: 1px solid var(--edge-inst);
    background: transparent;
    color: var(--ink);
    cursor: pointer;
  }
  .step:disabled { opacity: 0.35; cursor: default; }
  .count { min-width: 1.5rem; text-align: center; font-variant-numeric: tabular-nums; }
</style>
