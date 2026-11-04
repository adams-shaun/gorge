<script lang="ts" module>
  import type { BotPick } from '../../lib/pairedbot';
  /** FooterButton is one footer action; `data` carries the test/automation hooks (data-submit, data-primary …). */
  export interface FooterButton {
    label: string;
    onclick: (e: MouseEvent) => void;
    disabled?: boolean;
    primary?: boolean;
    title?: string;
    data?: Record<string, string | number | boolean>;
    /** The paired bot's pick when it would press this button (No attack, No blocks). */
    advised?: BotPick | null;
  }
</script>

<script lang="ts">
  import { pointerRelease } from '../../lib/pointer';
  import BotPickTag from './BotPickTag.svelte';
  /**
   * PromptFooter is the anatomy's last row: the live
   * state on the left ("2 of 2 chosen", "you'd take 5 (17 → 12)") and the
   * secondary then primary buttons on the right. The primary is gilt — the
   * one saturated control, because it is "your move".
   */
  let { status = null, buttons = [] }: { status?: string | null; buttons?: FooterButton[] } = $props();
</script>

{#if status || buttons.length > 0}
  <div class="foot" data-prompt-footer>
    <span class="status" data-prompt-status>{status ?? ''}</span>
    {#each buttons as b (b.label)}
      {#if b.advised}<BotPickTag pick={b.advised} />{/if}
      <button class="btn" class:primary={b.primary} class:advised={!!b.advised} type="button" title={b.title} disabled={b.disabled} use:pointerRelease onclick={b.onclick} data-bot-pick={b.advised ? true : undefined} {...b.data}>{b.label}</button>
    {/each}
  </div>
{/if}

<style>
  .foot {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: flex-end;
    gap: var(--sp-2);
    padding: var(--sp-2) 0 0;
  }
  .status {
    margin-right: auto;
    font-size: var(--t-12);
    color: var(--ink-dim);
    min-width: 0;
  }
  .btn {
    white-space: nowrap;
    padding: 0.4rem 0.9rem;
    border-radius: 7px;
    border: 1px solid var(--edge-inst);
    background: var(--instrument-raised);
    color: var(--ink-inst);
    font: 500 var(--t-12) var(--font-ui);
    cursor: pointer;
  }
  .btn:hover:not(:disabled) {
    border-color: var(--ink-dim);
    color: var(--ink);
  }
  .btn.primary {
    background: var(--gilt, #d4ad62);
    border-color: var(--gilt, #d4ad62);
    color: var(--felt-sunk);
    font-weight: 600;
  }
  .btn.advised { outline: 1px dashed var(--gilt, #d4ad62); outline-offset: 2px; }
  .btn:disabled {
    opacity: 0.45;
    cursor: default;
  }
</style>
