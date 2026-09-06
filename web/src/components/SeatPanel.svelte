<script lang="ts">
  import { onMount } from 'svelte';
  import type { CardView, Option, SeatInfo, View } from '../protocol';
  import type { SeatCtx } from '../lib/seat';
  import { SeatPanelState, isConcede, mulliganPhase, toneOf } from '../lib/seatpanel.svelte';
  import CardImage from './CardImage.svelte';
  import CardTile from './CardTile.svelte';
  import ManaPool from './ManaPool.svelte';

  /**
   * SeatPanel is a human seat's whole surface, overlaid on the board: the
   * four-fact status readout (survey item 4), the seat's floating mana, the
   * prompt as TEXT over the board naming the source (item 18 — never a
   * modal), and the options, with the primary button resolved by kind (item
   * 5 / R-E4-1) and the concede option visually separated and doubly
   * confirmed. All decision state is SeatPanelState; this component only
   * renders it. The board itself (Board, IdentityBar, Rail) is the same
   * seat-scoped view everything else renders.
   *
   * It floats on the felt but it IS an instrument — it is where the engine
   * asks a question — so it is drawn in the instrument register: one cool,
   * flat panel divided by hairlines, not a stack of rounded cards. It is
   * opaque enough to stay legible over card art and blurs what it covers.
   *
   * Colour states what the engine wants. Warm `--initiative` means the game
   * is blocked on this seat; cool `--offered` means a priority window is open
   * and passing is a legal answer; no colour means the wait is on someone
   * else. See toneOf — the distinction comes from option kinds, not from the
   * prompt text.
   */
  let { view, seats, ctx, table, match }: {
    view: View; seats: SeatInfo[]; ctx: SeatCtx; table: string; match: number;
  } = $props();

  // The props are constants per mount (Table keys the panel by match); the
  // state object captures only their initial values, like MatchState itself.
  // svelte-ignore state_referenced_locally
  const logic = new SeatPanelState(table, match, ctx);
  // Adopting the first view here rather than only in the effect below means
  // the panel's first paint already carries the decision — including on the
  // server, where effects never run.
  // svelte-ignore state_referenced_locally
  logic.adoptView(view.decision ?? null);

  // The pending decision comes from view.decision (the seat-scoped view at
  // head carries exactly the decision asked of this seat); fetchPending on
  // mount covers the first-view race and is the recovery path after a
  // rejected intent.
  $effect(() => {
    logic.adoptView(view.decision ?? null);
  });
  onMount(() => {
    void logic.refreshPending();
    // The panel used to learn about a new decision ONLY from view.decision,
    // which is refreshed by the SSE 'decision' frame. That makes the stream a
    // single point of failure for a seat: miss one frame -- a dropped
    // subscription, a reconnect, a race between the intent POST resolving and
    // the frame arriving -- and the panel sits empty forever while the server
    // holds a decision addressed to this seat, with no way back. Observed in
    // the first real game played through this client.
    //
    // A seat is one person clicking, so a slow poll costs nothing and makes
    // the stream an optimisation rather than a dependency. It only fires when
    // the panel believes it has nothing to answer, so a decision on screen is
    // never refetched out from under the user.
    const t = setInterval(() => {
      if (logic.pending === null && !logic.busy) void logic.refreshPending();
    }, 1000);
    return () => clearInterval(t);
  });

  const decision = $derived(logic.pending && logic.pending.seq !== logic.postedSeq ? logic.pending : null);
  const primary = $derived(logic.primary());
  const tone = $derived(toneOf(decision));
  const waitingName = $derived(seats[view.priority]?.name ?? `Seat ${view.priority}`);
  const stepLabel = $derived(view.step.charAt(0).toUpperCase() + view.step.slice(1));

  // This seat's own player row, found by seat number. Seat index and array
  // position are not guaranteed equal, and a seat missing from the view is a
  // real state (a spectator token pointed at a seat that left), so it is a
  // null the template handles rather than an assumption.
  const mine = $derived(view.players.find((p) => p.seat === ctx.seat) ?? null);

  // The London round gets its own body. mull is null for every other
  // decision, and also for a mulligan decision carrying an option kind this
  // layout does not cover — the generic list renders that one instead, so no
  // option is ever unreachable.
  const mull = $derived(mulliganPhase(decision));
  const handById = $derived(new Map((mine?.hand ?? []).map((c) => [c.id, c])));
  const bottomCount = $derived(decision?.min ?? 0);

  // The engine labels these "keep" and "mulligan"; a button says what
  // pressing it does. Resolved by kind, with the server's own label as the
  // fallback for anything unrecognised.
  const CHOICE_LABEL: Record<string, string> = { keep: 'Keep this hand', mulligan: 'Mulligan' };
  function choiceLabel(o: Option): string {
    return CHOICE_LABEL[o.kind] ?? o.label;
  }

  function cardFor(o: Option): CardView | undefined {
    return o.obj === undefined ? undefined : handById.get(o.obj);
  }
</script>

{#if view.over}
  <div class="seat-panel over" data-tone="idle" role="status">
    <p class="prompt result">{view.draw ? 'Draw' : `${seats[view.winner ?? -1]?.name ?? `Seat ${view.winner}`} wins`}</p>
    <p class="sub">Match over</p>
  </div>
{:else}
  <div class="seat-panel" class:wide={mull !== null} data-seat-panel data-tone={tone}>
    {#if mull === null}
      <div class="readout" role="status">
        <div class="fact">
          <span class="k">Priority</span>
          <span class="v">{view.priority === ctx.seat ? 'you' : waitingName}</span>
        </div>
        <div class="fact">
          <span class="k">Turn</span>
          <span class="v"><span class="num">{view.turn}</span> {seats[view.active]?.name ?? `Seat ${view.active}`}</span>
        </div>
        <div class="fact">
          <span class="k">Phase</span>
          <span class="v">{stepLabel}</span>
        </div>
        <div class="fact">
          <span class="k">Stack</span>
          <span class="v num">{view.stack.length}</span>
        </div>
        {#if mine}<ManaPool pool={mine.pool} />{/if}
      </div>
    {/if}

    {#if logic.error}
      <p class="error" role="alert" data-error>{logic.error}</p>
    {/if}

    {#if decision}
      <p class="prompt" data-prompt>{decision.prompt}</p>

      {#if mull !== null && mull.phase === 'keep'}
        <div class="hand" aria-label="Your opening hand">
          {#each mine?.hand ?? [] as c (c.id)}<CardTile card={c} />{/each}
        </div>
        <div class="choices" data-options>
          {#each mull.choices as opt (opt.index)}
            <button
              class="choice"
              class:keep={opt.kind === 'keep'}
              type="button"
              data-option={opt.index}
              onclick={() => logic.click(opt.index)}
              disabled={logic.busy}
            >{choiceLabel(opt)}</button>
          {/each}
        </div>
      {:else if mull !== null && mull.phase === 'bottom'}
        <div class="hand picking" data-options aria-label="Choose cards to put on the bottom">
          {#each mull.cards as opt (opt.index)}
            {@const card = cardFor(opt)}
            {@const at = logic.picked.indexOf(opt.index)}
            <button
              class="pick"
              class:picked={at >= 0}
              type="button"
              data-option={opt.index}
              aria-pressed={at >= 0}
              aria-label={card ? card.name : opt.label}
              onclick={() => logic.toggle(opt.index)}
              disabled={logic.busy}
            >
              {#if card}<CardImage {card} />{:else}<span class="fallback">{opt.label}</span>{/if}
              {#if at >= 0}<span class="order">{at + 1}</span>{/if}
            </button>
          {/each}
        </div>
        <div class="choices">
          <button
            class="choice keep"
            type="button"
            data-submit
            onclick={() => logic.submit()}
            disabled={!logic.canSubmit || logic.busy}
          >
            Bottom {bottomCount} {bottomCount === 1 ? 'card' : 'cards'}
          </button>
        </div>
      {:else}
        <div class="options" data-options>
          {#if primary}
            <button class="primary" type="button" data-primary onclick={() => logic.primaryClick()} disabled={logic.busy}>
              {primary.label}
            </button>
          {/if}
          <div class="list">
            {#each decision.options as opt (opt.index)}
              {#if isConcede(opt)}
                <div class="concede-sep" role="separator"></div>
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
                  {#if decision.max > 1 && pickedAt >= 0}<span class="order inline">{pickedAt + 1}</span>{/if}
                  <span class="label">{opt.label}</span>
                </button>
              {/if}
            {/each}
          </div>
          {#if logic.showSubmit}
            <button class="submit" type="button" data-submit onclick={() => logic.submit()} disabled={!logic.canSubmit || logic.busy}>
              {decision.min === 0 ? 'Confirm' : decision.min === decision.max ? `Choose ${decision.min}` : `Choose ${decision.min}–${decision.max}`}
            </button>
          {/if}
        </div>
      {/if}
    {:else if logic.postedSeq !== null}
      <p class="prompt waiting">Answer sent — waiting for the game to advance</p>
    {:else}
      <p class="prompt waiting" data-waiting>{stepLabel} — waiting for {waitingName}</p>
    {/if}
  </div>
{/if}

<style>
  /*
   * One instrument panel, not three floating cards. Hairlines divide the
   * readout from the prompt from the options, the way the rail divides its
   * sections — a stack of identically rounded boxes would read as chrome,
   * and this panel is the engine's own face.
   *
   * The tone rule on the left edge is the same grammar the rail already
   * speaks: PendingTray marks a waiting trigger with a rule, IdentityBar
   * marks a seat with one. Here it says whose move it is, and it is stated
   * ONCE — the panel's edge and the primary button share the tone colour
   * because they are one signal read at two distances, not two decorations.
   */
  .seat-panel {
    position: absolute;
    top: var(--sp-2);
    left: 50%;
    transform: translateX(-50%);
    z-index: 8;
    width: 21rem;
    max-width: 92%;
    max-height: calc(100% - var(--sp-4));
    display: flex;
    flex-direction: column;
    background: color-mix(in srgb, var(--instrument) 92%, transparent);
    border: 1px solid var(--edge-inst);
    border-left: 3px solid var(--edge-inst);
    border-radius: var(--radius);
    backdrop-filter: blur(8px);
    color: var(--ink-inst);
    overflow: hidden;
  }
  /* Warm: the table is stopped until this seat answers, and there is no pass.
     Cool: a window is open and passing is a legal answer. Neither: the wait
     is on someone else, and the panel says nothing louder than its hairline. */
  .seat-panel[data-tone='initiative'] {
    border-color: color-mix(in srgb, var(--initiative) 40%, var(--edge-inst));
    border-left-color: var(--initiative);
  }
  .seat-panel[data-tone='offered'] {
    border-left-color: var(--offered);
  }

  /* The mulligan round is not a heads-up display: nothing else is happening,
     the hand is the whole content, and the panel takes the middle of the
     board for the one moment it exists. */
  .seat-panel.wide {
    top: 50%;
    transform: translate(-50%, -50%);
    width: max-content;
    max-width: 94%;
    max-height: 96%;
    align-items: center;
  }

  .seat-panel > :global(*) {
    border-bottom: 1px solid var(--edge-inst);
  }
  .seat-panel > :global(*:last-child) {
    border-bottom: 0;
  }

  /* An instrument readout: labels above their values in four columns, read
     across in one glance, rather than four rows of a form read down. */
  .readout {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: var(--sp-1) var(--sp-4);
    padding: var(--sp-2) var(--sp-3);
    width: 100%;
  }
  .fact {
    display: flex;
    flex-direction: column;
    gap: 1px;
    min-width: 0;
  }
  .k {
    font-size: 0.6875rem;
    line-height: 1.2;
    color: var(--ink-faint);
  }
  .v {
    font-size: var(--t-12);
    line-height: 1.3;
    color: var(--ink-inst);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .num {
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
  }

  .prompt {
    margin: 0;
    padding: var(--sp-2) var(--sp-3);
    font-size: var(--t-14);
    font-weight: 600;
    line-height: 1.35;
    color: var(--ink);
    width: 100%;
    text-align: center;
  }
  .seat-panel[data-tone='initiative'] .prompt {
    color: var(--initiative);
  }
  .prompt.waiting {
    font-weight: 400;
    font-size: var(--t-12);
    color: var(--ink-dim);
  }

  /* An error is the one thing allowed to outrank the tone: the seat's last
     answer did not land, and nothing else on the panel matters until it is
     read. */
  .error {
    margin: 0;
    padding: var(--sp-2) var(--sp-3);
    background: color-mix(in srgb, var(--danger) 22%, var(--instrument));
    border-left: 3px solid var(--danger);
    color: var(--ink);
    font-size: var(--t-12);
    line-height: 1.35;
    width: 100%;
  }

  .options {
    display: flex;
    flex-direction: column;
    gap: var(--sp-1);
    padding: var(--sp-2);
    width: 100%;
    min-height: 0;
  }
  /* Only the option list scrolls. The primary button used to scroll away with
     it on a long priority window, which defeats the point of having one. */
  .list {
    display: flex;
    flex-direction: column;
    gap: 1px;
    max-height: 11rem;
    overflow-y: auto;
    min-height: 0;
  }
  .primary {
    background: var(--offered);
    color: var(--felt-sunk);
    border: 0;
    border-radius: var(--radius);
    padding: var(--sp-2) var(--sp-3);
    font-family: var(--font-ui);
    font-size: var(--t-14);
    font-weight: 600;
    cursor: pointer;
  }
  .seat-panel[data-tone='initiative'] .primary {
    background: var(--initiative);
  }
  .primary:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .option {
    display: flex;
    gap: var(--sp-2);
    align-items: baseline;
    text-align: left;
    background: var(--instrument-raised);
    color: var(--ink-inst);
    border: 0;
    border-left: 2px solid transparent;
    border-radius: 0;
    padding: var(--sp-1) var(--sp-2);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    line-height: 1.35;
    cursor: pointer;
  }
  .option:hover {
    background: color-mix(in srgb, var(--ink) 7%, var(--instrument-raised));
    border-left-color: var(--ink-dim);
    color: var(--ink);
  }
  .option.picked {
    border-left-color: var(--initiative);
    color: var(--ink);
  }
  .order {
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    font-size: 0.6875rem;
    line-height: 1;
    color: var(--felt-sunk);
    background: var(--initiative);
    border-radius: 2px;
    padding: 0.15em 0.3em;
  }
  .order.inline {
    flex: none;
  }

  .concede-sep {
    height: 1px;
    background: var(--edge-inst);
    margin: var(--sp-1) 0;
  }
  .concede-slot button.concede {
    width: 100%;
    text-align: center;
    background: transparent;
    color: color-mix(in srgb, var(--danger) 70%, var(--ink));
    border: 1px solid color-mix(in srgb, var(--danger) 50%, var(--edge-inst));
    border-radius: var(--radius);
    padding: var(--sp-1) var(--sp-2);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    cursor: pointer;
  }
  .concede-slot button.concede.confirm {
    background: var(--danger);
    color: var(--felt-sunk);
    font-weight: 600;
  }

  .submit {
    background: var(--instrument-raised);
    color: var(--ink-inst);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    padding: var(--sp-1) var(--sp-2);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    cursor: pointer;
  }
  .submit:disabled {
    opacity: 0.4;
    cursor: default;
  }

  /* The hand is the mulligan's content, so it gets the room. It scrolls
     inside itself on a narrow board rather than pushing the page sideways. */
  /* The hand IS the mulligan's content: seven cards you have to read before
     you can answer. CardImage ships two fixed widths — 90px, too small to
     read, and 220px, which wraps seven cards onto three rows of a laptop
     board — so the mulligan sizes them itself, between those two and
     following the viewport, and the row still wraps rather than scrolling
     sideways on a very narrow board. A card you have to scroll to is a card
     you skip. */
  .hand {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    align-items: flex-start;
    gap: var(--sp-2);
    padding: var(--sp-3);
    width: 100%;
    overflow-y: auto;
    min-height: 0;
  }
  .hand :global(.card-image) {
    width: clamp(88px, 10vw, 168px);
  }
  .pick {
    position: relative;
    display: block;
    padding: 0;
    background: transparent;
    border: 0;
    border-radius: var(--radius-card);
    cursor: pointer;
    line-height: 0;
    flex: none;
  }
  /* A chosen card leaves the hand, so it reads as leaving: dimmed and sunk,
     with the ordinal that says where in the library's bottom it lands. */
  .pick.picked {
    opacity: 0.55;
    box-shadow: 0 0 0 2px var(--initiative);
  }
  .pick .order {
    position: absolute;
    top: -0.4em;
    left: -0.4em;
    line-height: 1.3;
  }
  .fallback {
    display: inline-block;
    padding: var(--sp-2);
    font-size: var(--t-12);
    line-height: 1.35;
    color: var(--ink-inst);
  }

  .choices {
    display: flex;
    justify-content: center;
    gap: var(--sp-2);
    padding: var(--sp-3);
    width: 100%;
  }
  .choice {
    background: transparent;
    color: var(--ink-inst);
    border: 1px solid var(--ink-faint);
    border-radius: var(--radius);
    padding: var(--sp-2) var(--sp-6);
    font-family: var(--font-ui);
    font-size: var(--t-14);
    font-weight: 600;
    cursor: pointer;
  }
  .choice.keep {
    background: var(--initiative);
    border-color: var(--initiative);
    color: var(--felt-sunk);
  }
  .choice:hover:not(:disabled) {
    border-color: var(--ink-dim);
  }
  .choice.keep:hover:not(:disabled) {
    border-color: var(--initiative);
  }
  .choice:disabled {
    opacity: 0.5;
    cursor: default;
  }

  /* The match is over: a result, stated plainly. No new colour — a win is
     neither mana, card colour nor seat identity. */
  .over {
    align-items: center;
  }
  .result {
    font-size: var(--t-20);
    font-weight: 600;
    color: var(--ink);
  }
  .sub {
    margin: 0;
    padding: var(--sp-2) var(--sp-3);
    font-size: var(--t-12);
    color: var(--ink-dim);
    width: 100%;
    text-align: center;
  }
</style>
