<script lang="ts">
  import { onMount } from 'svelte';
  import type { SeatInfo, View } from '../protocol';
  import type { SeatCtx } from '../lib/seat';
  import { SeatPanelState, isConcede } from '../lib/seatpanel.svelte';

  /**
   * SeatPanel is a human seat's whole surface, overlaid on the board: the
   * four-line status block (survey item 4), the prompt as TEXT over the
   * board naming the source (item 18 — never a modal), and the options as
   * an anchored sheet, with the primary button resolved by kind (item 5 /
   * R-E4-1) and the concede option visually separated and doubly
   * confirmed. All decision state is SeatPanelState; this component only
   * renders it. The board itself (Board, IdentityBar, Rail) is the same
   * seat-scoped view everything else renders.
   */
  let { view, seats, ctx, table, match }: {
    view: View; seats: SeatInfo[]; ctx: SeatCtx; table: string; match: number;
  } = $props();

  // The props are constants per mount (Table keys the panel by match); the
  // state object captures only their initial values, like MatchState itself.
  // svelte-ignore state_referenced_locally
  const logic = new SeatPanelState(table, match, ctx);

  // The pending decision comes from view.decision (the seat-scoped view at
  // head carries exactly the decision asked of this seat); fetchPending on
  // mount covers the first-view race and is the recovery path after a
  // rejected intent.
  $effect(() => {
    logic.adoptView(view.decision ?? null);
  });
  onMount(() => {
    void logic.refreshPending();
  });

  const decision = $derived(logic.pending && logic.pending.seq !== logic.postedSeq ? logic.pending : null);
  const primary = $derived(logic.primary());
  const waitingName = $derived(seats[view.priority]?.name ?? `Seat ${view.priority}`);
  const stepLabel = $derived(view.step.charAt(0).toUpperCase() + view.step.slice(1));
</script>

{#if view.over}
  <div class="seat-panel over" role="status">
    <p class="prompt">{view.draw ? 'Draw' : `${seats[view.winner ?? -1]?.name ?? `Seat ${view.winner}`} wins`} — match over</p>
  </div>
{:else}
  <div class="seat-panel" data-seat-panel>
    <div class="status" role="status">
      <p class="line"><span class="k">Priority</span><span class="v">{view.priority === ctx.seat ? 'you' : waitingName}</span></p>
      <p class="line"><span class="k">Turn</span><span class="v">{view.turn} — {seats[view.active]?.name ?? `Seat ${view.active}`}</span></p>
      <p class="line"><span class="k">Phase</span><span class="v">{stepLabel}</span></p>
      <p class="line"><span class="k">Stack</span><span class="v">{view.stack.length} to resolve</span></p>
    </div>

    {#if logic.error}
      <p class="error" role="alert" data-error>{logic.error}</p>
    {/if}

    {#if decision}
      <p class="prompt" data-prompt>{decision.prompt}</p>
      <div class="options" data-options>
        {#if primary}
          <button class="primary" type="button" data-primary onclick={() => logic.primaryClick()} disabled={logic.busy}>
            {primary.label}
          </button>
        {/if}
        {#each decision.options as opt (opt.index)}
          {#if isConcede(opt)}
            <div class="concede-sep" role="separator"></div>
          {/if}
          {#if isConcede(opt)}
            <div class="concede-slot">
              {#if logic.confirming}
                <button class="concede confirm" type="button" data-confirm-concede onclick={() => logic.confirmConcede()} disabled={logic.busy}>
                  Concede — click again to confirm
                </button>
              {:else}
                <button class="concede" type="button" data-concede onclick={() => logic.click(opt.index)} disabled={logic.busy}>
                  {opt.label}
                </button>
              {/if}
            </div>
          {:else}
            {@const pickedAt = logic.picked.indexOf(opt.index)}
            <button
              class="option"
              class:picked={pickedAt >= 0}
              type="button"
              data-option={opt.index}
              onclick={() => logic.click(opt.index)}
              disabled={logic.busy}
            >
              {#if decision.max > 1 && pickedAt >= 0}<span class="order">{pickedAt + 1}.</span>{/if}
              <span class="label">{opt.label}</span>
            </button>
          {/if}
        {/each}
        {#if logic.showSubmit}
          <button class="submit" type="button" data-submit onclick={() => logic.submit()} disabled={!logic.canSubmit || logic.busy}>
            {decision.min === 0 ? 'Confirm' : decision.min === decision.max ? `Choose ${decision.min}` : `Choose ${decision.min}–${decision.max}`}
          </button>
        {/if}
      </div>
    {:else if logic.postedSeq !== null}
      <p class="prompt waiting">Answer sent — waiting for the game to advance…</p>
    {:else}
      <p class="prompt waiting" data-waiting>{stepLabel} — waiting for {waitingName}</p>
    {/if}
  </div>
{/if}

<style>
  .seat-panel {
    position: absolute; top: .5rem; left: 50%; transform: translateX(-50%);
    z-index: 8; display: flex; flex-direction: column; align-items: center;
    gap: .4rem; pointer-events: none; max-width: min(26rem, 70%);
  }
  .seat-panel.over .prompt { background: #1b1b1f; border: 1px solid #6c6; }
  .status {
    background: #1b1b1fdd; border: 1px solid #333; border-radius: 8px;
    padding: .35rem .8rem; display: grid; grid-template-columns: auto auto;
    gap: 0 .8rem; font-size: .72rem; pointer-events: auto;
  }
  .status .line { margin: 0; display: contents; }
  .status .k { opacity: .6; text-align: right; font-variant-numeric: tabular-nums; }
  .status .v { font-weight: 600; font-variant-numeric: tabular-nums; }
  .prompt {
    margin: 0; background: #1b1b1fdd; border: 1px solid #3b82f6; border-radius: 8px;
    padding: .4rem .9rem; font-weight: 600; font-size: .9rem; text-align: center;
    pointer-events: auto; max-width: 100%;
  }
  .prompt.waiting { border-color: #333; font-weight: 500; opacity: .85; }
  .error { margin: 0; background: #5b1010; border: 1px solid #b00; color: #ffd7d7; border-radius: 8px; padding: .3rem .8rem; font-size: .8rem; pointer-events: auto; }
  .options {
    display: flex; flex-direction: column; gap: .3rem; width: 19rem;
    background: #1b1b1fdd; border: 1px solid #333; border-radius: 10px;
    padding: .5rem; pointer-events: auto; max-height: 11rem; overflow-y: auto;
  }
  .primary {
    background: #3b82f6; color: white; border: none; border-radius: 6px;
    padding: .45rem .6rem; font-weight: 700; font-size: .85rem; cursor: pointer;
  }
  .primary:disabled { opacity: .5; cursor: default; }
  .option {
    display: flex; gap: .4rem; align-items: baseline; text-align: left;
    background: #232329; color: inherit; border: 1px solid transparent;
    border-radius: 6px; padding: .35rem .5rem; font: inherit; font-size: .8rem;
    cursor: pointer;
  }
  .option:hover { border-color: #666; }
  .option.picked { background: #2b3a55; border-color: #3b82f6; }
  .option .order { color: #6cf; font-weight: 700; }
  .concede-sep { height: 1px; background: #444; margin: .15rem 0; }
  .concede-slot button.concede {
    width: 100%; text-align: center; background: #3a1717; color: #ffb3b3;
    border: 1px solid #7a2020; border-radius: 6px; padding: .35rem .5rem;
    font: inherit; font-size: .8rem; cursor: pointer;
  }
  .concede-slot button.concede.confirm { background: #b00; color: white; font-weight: 700; }
  .submit {
    background: #232329; color: inherit; border: 1px solid #555; border-radius: 6px;
    padding: .35rem .5rem; font: inherit; font-size: .8rem; cursor: pointer;
  }
  .submit:disabled { opacity: .4; cursor: default; }
</style>
