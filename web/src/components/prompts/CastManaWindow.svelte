<script lang="ts">
  import type { Decision, View } from '../../protocol';
  import type { SeatPanelState } from '../../lib/seatpanel.svelte';
  import { activationCostText, castManaReadout, castManaRows, isCastManaWindow, isManaSacrificeAsk, manaWindowDone } from '../../lib/prompts/castmana';
  import { selectionStatus, submitLabel } from '../../lib/prompts/anatomy';
  import { digitMap } from '../../lib/prompts/order';
  import { promptWords } from '../../lib/prompts/words';
  import { pointerRelease } from '../../lib/pointer';
  import ManaSymbols from '../ManaSymbols.svelte';
  import Digit from './Digit.svelte';
  import PromptFooter, { type FooterButton } from './PromptFooter.svelte';

  /**
   * CastManaWindow is the manual cast-payment surface: the mid-cast CR
   * 601.2g window (one row per untapped mana source, its mana and — from
   * Option.cost — its beyond-tap cost, a still-owed readout and Done) and,
   * inline in the same panel, the sacrifice election one of those
   * activations hands back (lib/prompts/castmana.ts). It is ManaPayment-
   * Panel's visual grammar on a decision that carries no `mana_payment`
   * readout: the cost comes off the stack spell and the payer's floating
   * pool, so "Still owed" is an estimate the wire's two public facts give.
   *
   * Every button posts one offered option by its own wire index through the
   * seat panel's ordinary click path — a min==max==1 ask posts on the
   * click, a wider sacrifice election toggles into `picked` and the footer
   * commits, exactly as ListPrompt does. Nothing here decides what a source
   * can pay or which permanent may die; the candidate permanents are also
   * highlighted on the board through the shared card-options index.
   */
  let { decision, view, logic }: { decision: Decision; view: View | null; logic: SeatPanelState } = $props();

  const castWindow = $derived(isCastManaWindow(decision, view));
  const sacAsk = $derived(!castWindow && isManaSacrificeAsk(decision, view));
  const rows = $derived(castWindow ? castManaRows(decision, view) : []);
  const done = $derived(castWindow ? manaWindowDone(decision) : null);
  const readout = $derived(castWindow ? castManaReadout(decision, view) : null);
  const digits = $derived(digitMap(rowOrder()));
  const multi = $derived(decision.min !== 1 || decision.max !== 1);

  /** rowOrder is the panel's drawn order: the window's source rows in offer
   *  order then Done, or the sacrifice election's own option order — the
   *  same list lib/prompts/order.ts's generic rows read, so the pick hotkeys
   *  and these digits agree. */
  function rowOrder(): number[] {
    const out: number[] = [];
    if (castWindow) {
      for (const row of rows) for (const o of row.options) out.push(o.index);
      if (done) out.push(done.index);
    } else {
      for (const o of decision.options) out.push(o.index);
    }
    return out;
  }

  const footer = $derived.by((): FooterButton[] => {
    const out: FooterButton[] = [];
    if (done) out.push({ label: done.label, onclick: () => logic.click(done.index), disabled: logic.busy, primary: true, data: { 'data-cmp-done': done.index } });
    if (logic.showSubmit) {
      out.push({
        label: submitLabel(decision),
        onclick: (e) => logic.submit(e.ctrlKey),
        disabled: !logic.canSubmit || logic.busy,
        primary: done === null,
        data: { 'data-submit': true },
      });
    }
    return out;
  });
  const status = $derived(sacAsk ? selectionStatus(decision, logic.picked.length) : null);
</script>

{#if castWindow}
  <section class="mana-pay" data-cast-mana aria-label={decision.prompt}>
    {#if readout?.card}
      <dl class="mp-readout">
        <div class="mp-cell" data-cmp-cost>
          <dt>Cost</dt>
          <dd>{#if readout.cost}<ManaSymbols cost={readout.cost} />{:else}<span class="mp-none">?</span>{/if}</dd>
        </div>
        <div class="mp-cell" data-cmp-owed>
          <dt>Still owed (estimate)</dt>
          <dd>{#if readout.owed}<ManaSymbols cost={readout.owed} />{:else}<span class="mp-none">nothing</span>{/if}</dd>
        </div>
        <div class="mp-cell" data-cmp-pool>
          <dt>Pool</dt>
          <dd>{#if readout.pool}<ManaSymbols cost={readout.pool} />{:else}<span class="mp-none">empty</span>{/if}</dd>
        </div>
      </dl>
    {/if}

    <ul class="mp-sources" aria-label="Available mana abilities">
      {#each rows as row (row.obj)}
        <li class="mp-source" data-mana-source={row.obj}>
          <span class="mp-name" title={row.name}>{row.name}</span>
          <span class="cmp-mana" data-cmp-mana={row.obj}>
            {#if row.mana}<ManaSymbols cost={row.mana} />{/if}
            {#if row.any}<span class="cmp-any">any</span>{/if}
          </span>
          <span class="mp-abilities">
            {#each row.options as o (o.index)}
              <button
                class="mp-ability text"
                type="button"
                data-option={o.index}
                use:pointerRelease
                aria-label={o.cost ? `${row.name}: ${activationCostText(o.cost)}` : `Tap ${row.name} for mana`}
                title={o.cost ? activationCostText(o.cost) : o.label}
                disabled={logic.busy}
                onclick={() => logic.click(o.index)}
              ><Digit n={digits.get(o.index)} />{#if o.cost}{activationCostText(o.cost)}{:else}Tap for mana{/if}</button>
            {/each}
          </span>
        </li>
      {/each}
    </ul>

    {#if decision.payment_fallback}
      <p class="cmp-fallback" data-payment-fallback>Suggested mana could not be used ({decision.payment_fallback.reason}). Continue with the manual payment decision.</p>
    {/if}
    <PromptFooter status={null} buttons={footer} />
  </section>
{:else if sacAsk}
  <section class="mana-pay" data-cast-mana-sac aria-label={decision.prompt}>
    <p class="cmp-sac-title">{promptWords(decision.prompt)}</p>
    <ul class="mp-sources" aria-label="Permanents to sacrifice">
      {#each decision.options as o (o.index)}
        {@const at = logic.picked.indexOf(o.index)}
        <li class="mp-source" class:picked={at >= 0} data-mana-source={o.obj}>
          <span class="mp-name" title={o.label}>{promptWords(o.label)}</span>
          <span class="mp-abilities">
            <button
              class="mp-ability text"
              type="button"
              data-option={o.index}
              aria-pressed={multi ? at >= 0 : undefined}
              use:pointerRelease
              aria-label={`Sacrifice ${o.label}`}
              disabled={logic.busy}
              onclick={() => logic.click(o.index)}
            ><Digit n={digits.get(o.index)} />{at >= 0 && multi ? `Picked ${at + 1}` : 'Sacrifice'}</button>
          </span>
        </li>
      {/each}
    </ul>
    <p class="cmp-hint">The candidate permanents are highlighted on the board.</p>
    <PromptFooter status={status} buttons={footer} />
  </section>
{/if}

<style>
  /* ManaPaymentPanel's visual grammar: hairline readout cells, raised rows,
     the offered edge on the primary answer. */
  .mana-pay {
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    padding: var(--sp-2);
    width: 100%;
    min-height: 0;
  }
  .mp-readout {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 1px;
    margin: 0;
    background: var(--edge-inst);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    overflow: hidden;
  }
  .mp-cell {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    padding: var(--sp-1);
    background: var(--instrument-raised);
    min-width: 0;
  }
  .mp-cell dt {
    font-size: var(--t-11);
    color: var(--ink-dim);
  }
  .mp-cell dd {
    margin: 0;
    min-height: 15px;
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
  }
  [data-cmp-owed] dt { color: var(--initiative); }
  .mp-none {
    font-size: var(--t-11);
    color: var(--ink-faint);
  }
  .mp-sources {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 1px;
    max-height: 11rem;
    overflow-y: auto;
  }
  .mp-source {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--sp-2);
    padding: var(--sp-1) var(--sp-2);
    background: var(--instrument-raised);
    border-left: 2px solid transparent;
  }
  .mp-source.picked { border-left-color: var(--gilt, #d4ad62); }
  .mp-name {
    font-size: var(--t-12);
    color: var(--ink-inst);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
    flex: 1;
  }
  .cmp-mana {
    flex: none;
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
  }
  .cmp-any {
    font-size: var(--t-11);
    color: var(--ink-dim);
  }
  .mp-abilities {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--sp-1);
    flex: none;
  }
  .mp-ability {
    display: inline-flex;
    gap: 0.3rem;
    align-items: center;
    min-height: 1.75rem;
    padding: 2px var(--sp-1);
    background: transparent;
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    color: var(--ink-inst);
    font-family: var(--font-ui);
    font-size: var(--t-11);
    cursor: pointer;
    text-align: left;
    max-width: 100%;
  }
  .mp-ability:hover:not(:disabled) {
    border-color: var(--offered);
    background: color-mix(in srgb, var(--offered) 12%, var(--instrument-raised));
  }
  .mp-ability:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .cmp-sac-title {
    margin: 0;
    font-size: var(--t-12);
    color: var(--ink-inst);
  }
  .cmp-hint {
    margin: 0;
    font-size: var(--t-11);
    color: var(--ink-dim);
  }
  .cmp-fallback {
    margin: 0;
    font-size: var(--t-11);
    color: var(--ink-dim);
  }
</style>
