<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { SeatInfo, View } from '../protocol';
  import type { SeatCtx } from '../lib/seat';
  import { STOPPABLE_STEPS, type TurnSide } from '../lib/autopilot';
  import { stepLabel } from '../lib/phases';
  import { autoNoteText, isConcede, type SeatPanelState } from '../lib/seatpanel.svelte';
  import SeatPanel from './SeatPanel.svelte';

  /** A short grace period keeps a diagonal tab-to-panel pointer path open. */
  const HOT_STRIP_CLOSE_DELAY_MS = 180;

  let { view, seats, state: logic, ctx, table, match }: {
    view: View;
    seats: SeatInfo[];
    state: SeatPanelState;
    ctx: SeatCtx;
    table: string;
    match: number;
  } = $props();

  type Tab = 'actions' | 'pass' | 'ffwd' | 'done' | 'options';
  let open = $state<Tab | null>(null);
  let closeTimer: ReturnType<typeof setTimeout> | null = null;

  const decision = $derived(logic.active);
  const actions = $derived(
    decision?.options.filter((o) => o.kind !== 'pass' && !isConcede(o)) ?? [],
  );
  const passAvailable = $derived(logic.passOption !== null && !logic.busy);
  // Fast forward can only advance through a real pass option. considerAuto is
  // still the safety oracle; this gate merely avoids inventing a control for
  // a decision the server did not say can be passed.
  const ffwdAvailable = $derived(passAvailable);
  const doneAvailable = $derived(
    decision !== null && logic.showSubmit && logic.canSubmit && !logic.busy,
  );
  const doneShown = $derived(decision !== null && logic.showSubmit);
  const doneFull = $derived.by(() => {
    switch (decision?.kind) {
      case 'attackers': return 'Done selecting attackers';
      case 'blockers': return 'Done selecting blockers';
      case 'target': return 'Done selecting targets';
      case 'modes': return 'Done picking modes';
      case 'trigger_order': return 'Done ordering triggers';
      case 'choose': return 'Done picking';
      default: return 'Done selecting';
    }
  });
  const doneCompact = $derived(decision?.kind === 'attackers' ? 'ATTACK' : 'DONE_SELECT');

  function clearClose(): void {
    if (closeTimer !== null) clearTimeout(closeTimer);
    closeTimer = null;
  }
  function show(tab: Tab): void {
    clearClose();
    open = tab;
  }
  function scheduleClose(): void {
    clearClose();
    closeTimer = setTimeout(() => {
      open = null;
      closeTimer = null;
    }, HOT_STRIP_CLOSE_DELAY_MS);
  }
  function escape(e: KeyboardEvent): void {
    if (e.key !== 'Escape' || open === null) return;
    e.preventDefault();
    open = null;
    clearClose();
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
  }
  function fastForward(): void {
    if (!ffwdAvailable) return;
    logic.startFastForward();
    logic.considerAuto(view);
  }
  function toggleStop(side: TurnSide, step: string): void {
    logic.toggleStop(step, side);
  }
  onDestroy(clearClose);
</script>

<div class="hot-strip" data-hot-strip role="toolbar" aria-label="Game controls" tabindex="-1" onkeydown={escape}>
  <div class="hot-tab" role="presentation" onpointerenter={() => show('actions')} onpointerleave={scheduleClose} onfocusin={() => show('actions')} onfocusout={scheduleClose}>
    <button class="tab" type="button" data-hot-tab="actions" aria-label="Actions" aria-haspopup="true" aria-expanded={open === 'actions'} aria-controls="hot-panel-actions" aria-disabled={actions.length === 0} onclick={() => show('actions')}>
      <span class="full">ACTIONS</span><span class="compact" aria-hidden="true">A</span>
    </button>
    <div class="drop actions" class:open={open === 'actions'} id="hot-panel-actions" data-hot-panel="actions" role="group" aria-label="Available actions">
      <!-- SeatPanel owns the polling/autopilot lifecycle as well as this
           option list, so it stays mounted even when no action is offered.
           Hiding it in that case would also silently disable Skip Empty. -->
      <SeatPanel {view} {seats} {ctx} {table} {match} state={logic} placement="strip" />
      {#if actions.length === 0}
        <p class="unavailable">No action is offered by this decision.</p>
      {/if}
    </div>
  </div>

  <div class="hot-tab" role="presentation" onpointerenter={() => show('pass')} onpointerleave={scheduleClose} onfocusin={() => show('pass')} onfocusout={scheduleClose}>
    <button class="tab" type="button" data-hot-tab="pass" aria-label="Pass" aria-haspopup="true" aria-expanded={open === 'pass'} aria-controls="hot-panel-pass" aria-disabled={!passAvailable} onclick={() => show('pass')}>
      <span class="full">PASS</span><span class="compact" aria-hidden="true">&gt;</span>
    </button>
    <div class="drop small" class:open={open === 'pass'} id="hot-panel-pass" data-hot-panel="pass" role="group" aria-label="Pass options">
      {#if logic.passOption}
        <button class="drop-action" type="button" data-pass-action onclick={() => logic.passClick()} disabled={logic.busy}>{logic.passOption.label}</button>
      {:else}
        <p class="unavailable">Pass is not offered by this decision.</p>
      {/if}
    </div>
  </div>

  <div class="hot-tab" role="presentation" onpointerenter={() => show('ffwd')} onpointerleave={scheduleClose} onfocusin={() => show('ffwd')} onfocusout={scheduleClose}>
    <button class="tab" class:on={logic.fastForward} type="button" data-hot-tab="ffwd" aria-label="Fast forward" aria-haspopup="true" aria-expanded={open === 'ffwd'} aria-controls="hot-panel-ffwd" aria-disabled={!ffwdAvailable} onclick={() => show('ffwd')}>
      <span class="full">FFWD</span><span class="compact" aria-hidden="true">&gt;&gt;</span>
    </button>
    <div class="drop small" class:open={open === 'ffwd'} id="hot-panel-ffwd" data-hot-panel="ffwd" role="group" aria-label="Fast forward options">
      {#if ffwdAvailable || logic.fastForward}
        <button class="drop-action" type="button" data-fast-forward aria-pressed={logic.fastForward} onclick={fastForward} disabled={!ffwdAvailable}>Fast forward to the next stop</button>
      {:else}
        <p class="unavailable">Fast forward needs a pass option.</p>
      {/if}
    </div>
  </div>

  <div class="hot-tab contextual" role="presentation" onpointerenter={() => show('done')} onpointerleave={scheduleClose} onfocusin={() => show('done')} onfocusout={scheduleClose}>
    <button class="tab" type="button" data-hot-tab="done" aria-label={doneFull} aria-haspopup="true" aria-expanded={open === 'done'} aria-controls="hot-panel-done" aria-disabled={!doneAvailable} onclick={() => show('done')}>
      <span class="full">{doneFull.toUpperCase()}</span><span class="compact" aria-hidden="true">{doneCompact}</span>
    </button>
    <div class="drop small" class:open={open === 'done'} id="hot-panel-done" data-hot-panel="done" role="group" aria-label="Selection options">
      {#if doneShown}
        <button class="drop-action" type="button" data-done-action onclick={() => logic.submit()} disabled={!doneAvailable}>{doneFull}</button>
      {:else}
        <p class="unavailable">This decision does not need a separate selection submit.</p>
      {/if}
    </div>
  </div>

  <div class="hot-tab" role="presentation" onpointerenter={() => show('options')} onpointerleave={scheduleClose} onfocusin={() => show('options')} onfocusout={scheduleClose}>
    <button class="tab" type="button" data-hot-tab="options" aria-label="Game options" aria-haspopup="true" aria-expanded={open === 'options'} aria-controls="hot-panel-options" onclick={() => show('options')}>
      <span class="full">GAME OPTIONS</span><span class="compact" aria-hidden="true">OPTS</span>
    </button>
    <div class="drop game" class:open={open === 'options'} id="hot-panel-options" data-hot-panel="options" role="group" aria-label="Game options">
      <div class="setting-row">
        <button class:on={logic.auto} type="button" role="switch" aria-checked={logic.auto} onclick={() => logic.setAuto(!logic.auto)}>Auto pass</button>
        <button class:on={logic.skipEmpty} type="button" role="switch" aria-checked={logic.skipEmpty} onclick={() => logic.setSkipEmpty(!logic.skipEmpty)}>Skip empty windows</button>
      </div>
      <p class="note">{autoNoteText(logic.note)}</p>
      {#each ['yours', 'opponents'] as side (side)}
        <fieldset>
          <legend>{side === 'yours' ? 'Stops on your turn' : 'Stops on opponents’ turns'}</legend>
          <div class="stop-grid">
            {#each STOPPABLE_STEPS as step (step)}
              <button
                class:on={logic.stops[side as TurnSide].has(step)}
                type="button"
                aria-pressed={logic.stops[side as TurnSide].has(step)}
                onclick={() => toggleStop(side as TurnSide, step)}
              >{stepLabel(step)}</button>
            {/each}
          </div>
        </fieldset>
      {/each}
    </div>
  </div>
</div>

<style>
  /* Tabs share an edge with the phase row above: this is one instrument. Each
     wrapper contains both tab and panel, so the pointer crosses no dead felt.
     JavaScript applies HOT_STRIP_CLOSE_DELAY_MS after leaving that contiguous
     region; focus uses the same region and Escape closes it immediately. */
  .hot-strip {
    position: relative;
    z-index: 2;
    display: flex;
    justify-content: center;
    height: var(--hot-strip-h);
    min-width: 0;
    font-family: var(--font-ui);
  }
  .hot-tab {
    position: relative;
    height: 100%;
  }
  .tab {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    min-width: 4rem;
    padding: 0 var(--sp-2);
    border: 1px solid var(--edge-inst);
    border-top: 0;
    border-right: 0;
    border-radius: 0 0 var(--radius) var(--radius);
    background: var(--instrument);
    color: var(--ink-inst);
    font-size: var(--t-11);
    font-weight: 600;
    cursor: pointer;
    white-space: nowrap;
  }
  .hot-tab:first-child .tab { border-left-color: color-mix(in srgb, var(--seat) 34%, var(--edge-inst)); }
  .hot-tab:last-child .tab { border-right: 1px solid var(--edge-inst); }
  .tab.on,
  .tab[aria-expanded='true'] {
    color: var(--ink);
    background: var(--instrument-raised);
    border-bottom-color: var(--offered);
  }
  .tab[aria-disabled='true'] {
    color: var(--ink-faint);
    cursor: default;
  }
  .compact { display: none; }

  .drop {
    position: absolute;
    top: 100%;
    left: 50%;
    z-index: 9;
    width: min(24rem, calc(100vw - var(--sp-4)));
    max-height: min(42vh, 28rem);
    overflow-y: auto;
    transform: translateX(-50%);
    visibility: hidden;
    pointer-events: none;
    background: var(--instrument);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    color: var(--ink-inst);
  }
  .drop.open {
    visibility: visible;
    pointer-events: auto;
  }
  .drop.small { width: min(18rem, calc(100vw - var(--sp-4))); }
  .drop.game { width: min(34rem, calc(100vw - var(--sp-4))); }
  /* Keep the edge under the tab contiguous even though the panel has a
     border: no margin, translate, or transparent bridge sits in this path. */
  .drop-action,
  .setting-row button,
  .stop-grid button {
    border: 0;
    border-radius: 0;
    background: var(--instrument-raised);
    color: var(--ink-inst);
    font-family: var(--font-ui);
    cursor: pointer;
  }
  .drop-action {
    width: 100%;
    padding: var(--sp-2) var(--sp-3);
    text-align: left;
    font-size: var(--t-12);
  }
  .drop-action:not(:disabled):hover,
  .setting-row button:hover,
  .stop-grid button:hover { color: var(--ink); }
  .drop-action:disabled { color: var(--ink-faint); cursor: default; }
  .unavailable,
  .note {
    margin: 0;
    padding: var(--sp-2) var(--sp-3);
    color: var(--ink-faint);
    font-size: var(--t-11);
    line-height: 1.35;
  }
  .setting-row {
    display: flex;
    gap: 1px;
    padding: 1px;
    border-bottom: 1px solid var(--edge-inst);
  }
  .setting-row button {
    flex: 1 1 0;
    padding: var(--sp-2);
    font-size: var(--t-12);
  }
  .setting-row button.on,
  .stop-grid button.on {
    background: var(--offered);
    color: var(--felt-sunk);
  }
  fieldset {
    margin: 0;
    padding: var(--sp-2);
    border: 0;
    border-top: 1px solid var(--edge-inst);
  }
  legend {
    padding: 0 var(--sp-1);
    color: var(--ink-faint);
    font-size: var(--t-10);
  }
  .stop-grid {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: 1px;
  }
  .stop-grid button {
    min-width: 0;
    padding: var(--sp-1);
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: var(--t-10);
    white-space: nowrap;
  }
  .actions :global(.seat-panel.strip) {
    width: 100%;
    min-width: 0;
    max-height: none;
    border: 0;
    border-radius: 0;
    backdrop-filter: none;
  }

  @media (max-width: 60rem) {
    .full { display: none; }
    .compact { display: inline; }
    .tab { min-width: 2.5rem; padding: 0 var(--sp-1); }
    .contextual .tab { min-width: 5.5rem; }
  }
</style>
