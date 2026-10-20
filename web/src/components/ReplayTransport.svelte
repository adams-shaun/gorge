<script lang="ts">
  import type { DvrAction, DvrState } from '../lib/dvr';
  import { behindLive } from '../lib/dvr';
  import type { SeatInfo } from '../protocol';
  import { seatColour } from '../lib/colours';
  import { pointerRelease } from '../lib/pointer';
  import { SPEEDS, STEP_UNITS, bandAt, intervalMs, stepTarget, turnBands, type StepUnit } from '../lib/replay';
  import type { TimelineMark } from '../lib/review';

  /**
   * ReplayTransport is the DVR's bottom strip for every unseated view: the
   * finished-match replay and a live spectator. It plays (a timer stepping
   * the cursor one `unit` at a time), steps and scrubs by dispatching the
   * DVR's own 'scrub'/'pause'/'live' actions, and draws the match as turn
   * bands tinted by the active seat's identity colour.
   */
  let {
    dvr,
    onAction,
    finished = false,
    seats = [],
    unit = $bindable<StepUnit>('decision'),
    speed = $bindable(1),
    playing = $bindable(false),
    hotkeys = true,
    marks = [],
    decisionStops = null,
    skipPasses = $bindable(true),
  }: {
    dvr: DvrState;
    onAction: (a: DvrAction) => void;
    finished?: boolean;
    seats?: SeatInfo[];
    unit?: StepUnit;
    speed?: number;
    playing?: boolean;
    /** hotkeys binds Space/K and the arrows table-wide (an unseated view only). */
    hotkeys?: boolean;
    /** marks are the review overlay's override/fallback ticks (a review table only). */
    marks?: TimelineMark[];
    /**
     * decisionStops are the Decision step's stops with pass-only decisions
     * left out (a review table only, which knows every decision's options);
     * null steps through every decision_ask.
     */
    decisionStops?: number[] | null;
    /** skipPasses applies decisionStops; the toggle shows only when they exist. */
    skipPasses?: boolean;
  } = $props();

  const stops = $derived(decisionStops && skipPasses ? decisionStops : null);

  const bands = $derived(turnBands(dvr, seats.length));
  const here = $derived(bandAt(bands, dvr.cursor));
  const span = $derived(Math.max(1, dvr.head));
  const pct = (seq: number) => `${(Math.min(dvr.head, Math.max(0, seq)) / span) * 100}%`;
  const atEnd = $derived(dvr.cursor >= dvr.head);
  const seatName = (p: number | null) => (p === null ? '' : (seats[p]?.name ?? `Seat ${p + 1}`));
  /** Label a round every `labelEvery` rounds so the numbers never collide. */
  const rounds = $derived(bands.length ? bands[bands.length - 1].round : 0);
  const labelEvery = $derived(rounds <= 12 ? 1 : rounds <= 30 ? 2 : rounds <= 60 ? 5 : 10);

  const unitLabel: Record<StepUnit, string> = { event: 'Event', decision: 'Decision', turn: 'Turn' };

  function step(dir: 1 | -1, by: StepUnit = unit) {
    playing = false;
    onAction({ type: 'scrub', seq: stepTarget(dvr, by, dir, stops) });
  }

  function togglePlay() {
    if (playing) {
      playing = false;
      return;
    }
    // Play from the start when parked at the end, the way a player rewinds.
    if (atEnd) onAction({ type: 'scrub', seq: 0 });
    else if (dvr.live) onAction({ type: 'pause' });
    playing = true;
  }

  // The playback clock: one step per interval until the end of the log. A
  // live spectator catching up to head simply stops there.
  $effect(() => {
    if (!playing) return;
    const id = setInterval(() => {
      if (dvr.cursor >= dvr.head) {
        playing = false;
        return;
      }
      onAction({ type: 'scrub', seq: stepTarget(dvr, unit, 1, stops) });
    }, intervalMs(unit, speed));
    return () => clearInterval(id);
  });

  // Scrubbing on the timeline: pointer position to seq, with capture so a
  // drag that leaves the strip keeps scrubbing.
  let track: HTMLDivElement | undefined = $state();
  let hover = $state<{ x: number; seq: number; mark: TimelineMark | null } | null>(null);
  function seqAt(clientX: number): number {
    if (!track) return dvr.cursor;
    const r = track.getBoundingClientRect();
    const f = Math.min(1, Math.max(0, (clientX - r.left) / Math.max(1, r.width)));
    return Math.round(f * dvr.head);
  }
  function onPointerDown(e: PointerEvent) {
    if (e.button !== 0) return;
    playing = false;
    track?.setPointerCapture(e.pointerId);
    onAction({ type: 'scrub', seq: seqAt(e.clientX) });
  }
  function onPointerMove(e: PointerEvent) {
    if (!track) return;
    const r = track.getBoundingClientRect();
    const x = e.clientX - r.left;
    // A mark within a few pixels of the pointer names itself in the tip.
    let mark: TimelineMark | null = null;
    let best = 5;
    for (const m of marks) {
      const d = Math.abs((Math.min(dvr.head, m.seq) / span) * r.width - x);
      if (d <= best) {
        best = d;
        mark = m;
      }
    }
    const h = { x, seq: seqAt(e.clientX), mark };
    hover = h;
    if (track.hasPointerCapture(e.pointerId)) onAction({ type: 'scrub', seq: h.seq });
  }
  const hoverBand = $derived(hover ? bandAt(bands, hover.seq) : null);

  function onTrackKey(e: KeyboardEvent) {
    const map: Record<string, () => void> = {
      ArrowRight: () => step(1),
      ArrowLeft: () => step(-1),
      PageDown: () => step(1, 'turn'),
      PageUp: () => step(-1, 'turn'),
      Home: () => onAction({ type: 'scrub', seq: 0 }),
      End: () => onAction({ type: 'scrub', seq: dvr.head }),
    };
    const f = map[e.key];
    if (f) {
      e.preventDefault();
      if (e.key === 'Home' || e.key === 'End') playing = false;
      f();
    }
  }

  // Table-wide replay keys for an unseated viewer: Space/K play, arrows step
  // (Shift: by turn). Ignored while typing or with a command modifier, and
  // while a control that owns the key has focus.
  function onWindowKey(e: KeyboardEvent) {
    if (!hotkeys) return;
    if (e.ctrlKey || e.metaKey || e.altKey || e.defaultPrevented) return;
    const t = e.target as HTMLElement | null;
    if (t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT|BUTTON)$/.test(t.tagName) || t.getAttribute('role') === 'slider')) return;
    if (e.code === 'Space' || e.code === 'KeyK') {
      e.preventDefault();
      togglePlay();
    } else if (e.code === 'ArrowRight' || e.code === 'ArrowLeft') {
      e.preventDefault();
      step(e.code === 'ArrowRight' ? 1 : -1, e.shiftKey ? 'turn' : unit);
    }
  }
</script>

<svelte:window onkeydown={onWindowKey} />

<div class="transport" role="group" aria-label="Replay" data-cursor={dvr.cursor} data-live={dvr.live} data-playing={playing}>
  <div class="buttons">
    <button type="button" onclick={() => step(-1, 'turn')} aria-label="previous turn" title="Previous turn (Shift+←)" use:pointerRelease>
      <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 3h2v10H3zM13 3v10L6 8z" /></svg>
    </button>
    <button type="button" onclick={() => step(-1)} aria-label="step back" title={`Back one ${unitLabel[unit].toLowerCase()} (←)`} use:pointerRelease>
      <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M11 3v10L4 8z" /></svg>
    </button>
    <button type="button" class="play" onclick={togglePlay} aria-label={playing ? 'pause' : 'play'} aria-pressed={playing} title="Play or pause (Space)" use:pointerRelease>
      {#if playing}
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M4 3h3v10H4zM9 3h3v10H9z" /></svg>
      {:else}
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M4 2.5v11L13 8z" /></svg>
      {/if}
    </button>
    <button type="button" onclick={() => step(1)} aria-label="step forward" title={`Forward one ${unitLabel[unit].toLowerCase()} (→)`} use:pointerRelease>
      <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M5 3v10l7-5z" /></svg>
    </button>
    <button type="button" onclick={() => step(1, 'turn')} aria-label="next turn" title="Next turn (Shift+→)" use:pointerRelease>
      <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 3v10l7-5zM11 3h2v10h-2z" /></svg>
    </button>
    {#if !finished}
      {#if dvr.live}
        <button type="button" class="live is-live" onclick={() => onAction({ type: 'pause' })} aria-label="pause live" use:pointerRelease>Live</button>
      {:else}
        <button type="button" class="live" onclick={() => { playing = false; onAction({ type: 'live' }); }} aria-label="return to live" use:pointerRelease>Go live <span class="behind">{behindLive(dvr)} behind</span></button>
      {/if}
    {/if}
  </div>

  <div class="where" aria-live="polite">
    <span class="turn">{here ? `Turn ${here.round}` : 'Setup'}</span>
    {#if here && here.player !== null}
      <span class="who" style:--seat={seatColour(here.player, seats)}>{seatName(here.player)}</span>
    {/if}
  </div>

  <div
    class="track"
    bind:this={track}
    role="slider"
    tabindex="0"
    aria-label="Replay position"
    aria-valuemin={0}
    aria-valuemax={dvr.head}
    aria-valuenow={dvr.cursor}
    aria-valuetext={`${here ? `Turn ${here.round}, ${seatName(here.player)}` : 'Setup'}, event ${dvr.cursor} of ${dvr.head}`}
    onpointerdown={onPointerDown}
    onpointermove={onPointerMove}
    onpointerleave={() => (hover = null)}
    onkeydown={onTrackKey}
  >
    <div class="lane">
      {#each bands as b (b.start)}
        <div
          class="band"
          class:first={b.round % labelEvery === 0 && (b.turn - 1) % Math.max(1, seats.length) === 0}
          style:left={pct(b.start)}
          style:width={`calc(${pct(b.end)} - ${pct(b.start)})`}
          style:--seat={b.player === null ? 'var(--ink-faint)' : seatColour(b.player, seats)}
        ></div>
      {/each}
      <div class="played" style:width={pct(dvr.cursor)}></div>
      <div class="head" style:left={pct(dvr.cursor)}></div>
    </div>
    {#if marks.length > 0}
      <div class="marks" aria-hidden="true">
        {#each marks as mk (mk.seq)}
          <span class="mark mark-{mk.kind}" style:left={pct(mk.seq)}></span>
        {/each}
      </div>
    {/if}
    <div class="rounds" aria-hidden="true">
      {#each bands as b (b.start)}
        {#if (b.turn - 1) % Math.max(1, seats.length) === 0 && (b.round === 1 || b.round % labelEvery === 0)}
          <span style:left={pct(b.start)}>{b.round}</span>
        {/if}
      {/each}
    </div>
    {#if hover}
      <div class="tip" style:left={`${hover.x}px`} aria-hidden="true">
        {#if hover.mark}<span class="tip-mark tip-{hover.mark.kind}">{hover.mark.tip}</span>{:else}{hoverBand ? `Turn ${hoverBand.round}, ${seatName(hoverBand.player)}` : 'Setup'} <span class="tip-seq">event {hover.seq}</span>{/if}
      </div>
    {/if}
  </div>

  <div class="settings">
    <span class="seq">event {dvr.cursor} of {dvr.head}</span>
    <div class="seg" role="radiogroup" aria-label="Step by">
      {#each STEP_UNITS as u (u)}
        <button type="button" role="radio" aria-checked={unit === u} class:on={unit === u} onclick={() => (unit = u)} use:pointerRelease>{unitLabel[u]}</button>
      {/each}
    </div>
    {#if decisionStops && unit === 'decision'}
      <label class="skip" title="Step and play past decisions whose only options are Pass priority and Concede">
        <input type="checkbox" bind:checked={skipPasses} use:pointerRelease />
        Skip passes
      </label>
    {/if}
    <label class="speed">
      <span class="sr">Speed</span>
      <select bind:value={speed} aria-label="Playback speed">
        {#each SPEEDS as s (s)}<option value={s}>{s === 0.5 ? '½' : s}×</option>{/each}
      </select>
    </label>
  </div>
</div>

<style>
  /* The transport is instrument, like the phase track above it: cool, dense,
     hairline structure. Its one loud element is the timeline. */
  .transport {
    display: grid;
    grid-template-columns: auto auto minmax(8rem, 1fr) auto;
    grid-template-areas: 'buttons where track settings';
    align-items: center;
    gap: var(--sp-2) var(--sp-4);
    padding: var(--sp-2) var(--sp-4);
    background: var(--instrument);
    border-top: 1px solid var(--edge-inst);
    color: var(--ink-inst);
    font-family: var(--font-ui);
    font-size: var(--t-12);
  }
  .buttons { grid-area: buttons; display: flex; align-items: center; gap: 2px; }
  .where { grid-area: where; display: flex; align-items: baseline; gap: var(--sp-2); min-width: 9.5rem; }
  .track { grid-area: track; }
  .settings { grid-area: settings; display: flex; align-items: center; gap: var(--sp-3); }

  button {
    font: inherit;
    color: inherit;
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--radius);
    cursor: pointer;
  }
  .buttons button {
    width: 2rem;
    height: 2rem;
    display: grid;
    place-items: center;
  }
  .buttons button:hover { background: var(--instrument-raised); border-color: var(--edge-inst); }
  .buttons svg { width: 14px; height: 14px; fill: currentColor; }
  .buttons .play {
    width: 2.5rem;
    height: 2.5rem;
    margin: 0 var(--sp-1);
    border-radius: 50%;
    background: var(--gilt);
    color: var(--felt-sunk);
  }
  .buttons .play:hover { background: #e2c07e; border-color: transparent; }
  .buttons .play svg { width: 16px; height: 16px; }
  .buttons .live {
    width: auto;
    padding: 0 var(--sp-3);
    margin-left: var(--sp-2);
    border-color: var(--edge-inst);
    white-space: nowrap;
  }
  .buttons .live.is-live { color: var(--verdigris); border-color: color-mix(in srgb, var(--verdigris) 50%, transparent); }
  .behind { color: var(--ink-faint); margin-left: var(--sp-1); }
  .skip { display: flex; align-items: center; gap: var(--sp-1); white-space: nowrap; cursor: pointer; }
  .skip input { margin: 0; accent-color: var(--gilt); }
  .skip input:focus-visible { outline: 2px solid var(--gilt); outline-offset: 2px; }
  button:focus-visible, select:focus-visible, .track:focus-visible {
    outline: 2px solid var(--gilt);
    outline-offset: 2px;
  }

  .turn {
    font-family: var(--font-serif);
    font-size: var(--t-20);
    font-weight: 600;
    color: var(--ink);
    white-space: nowrap;
  }
  .who {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 10rem;
    padding-left: var(--sp-2);
    border-left: 3px solid var(--seat);
  }

  /* The timeline: each engine turn is a band in its active seat's identity
     colour, kept low so the played portion and the playhead read first. */
  .track {
    position: relative;
    height: 2.25rem;
    cursor: pointer;
    touch-action: none;
    border-radius: var(--radius);
  }
  .lane {
    position: absolute;
    left: 0;
    right: 0;
    top: 0.35rem;
    height: 0.9rem;
    background: var(--instrument-raised);
    border-radius: 2px;
    overflow: hidden;
  }
  .band {
    position: absolute;
    top: 0;
    bottom: 0;
    background: color-mix(in srgb, var(--seat) 30%, transparent);
    box-shadow: inset 1px 0 0 rgb(0 0 0 / 0.45);
  }
  .band.first { box-shadow: inset 1px 0 0 color-mix(in srgb, var(--ink-inst) 55%, transparent); }
  .played {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    background: rgb(255 255 255 / 0.10);
    pointer-events: none;
  }
  .head {
    position: absolute;
    top: -3px;
    bottom: -3px;
    width: 3px;
    margin-left: -1.5px;
    background: var(--gilt);
    box-shadow: 0 0 0 1px var(--felt-sunk);
    pointer-events: none;
    z-index: 2;
  }
  /* The review overlay's ticks: a thin stem through the lane with a cap
     above it, verdigris for an override (the bot's alternative colour in
     the decision panel), ember for a fallback. They sit under the playhead. */
  .marks {
    position: absolute;
    left: 0;
    right: 0;
    top: 0.35rem;
    height: 0.9rem;
    pointer-events: none;
    z-index: 1;
  }
  .mark {
    position: absolute;
    top: -0.3rem;
    bottom: 0;
    width: 2px;
    margin-left: -1px;
    border-radius: 1px;
  }
  .mark::before {
    content: '';
    position: absolute;
    top: 0;
    left: -2px;
    width: 6px;
    height: 4px;
    border-radius: 1px;
    background: inherit;
  }
  .mark-override { background: var(--verdigris); }
  .mark-fallback { background: var(--ember); }
  .tip-override { color: var(--verdigris); }
  .tip-fallback { color: var(--ember); }
  .rounds {
    position: absolute;
    left: 0;
    right: 0;
    top: 1.35rem;
    height: 0.9rem;
    font-family: var(--font-serif);
    font-size: var(--t-12);
    color: var(--ink-faint);
    pointer-events: none;
  }
  .rounds span { position: absolute; transform: translateX(-1px); }
  .tip {
    position: absolute;
    bottom: calc(100% + 4px);
    transform: translateX(-50%);
    padding: 2px var(--sp-2);
    background: var(--instrument-raised);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    color: var(--ink);
    white-space: nowrap;
    pointer-events: none;
    z-index: 5;
  }
  .tip-seq { color: var(--ink-faint); margin-left: var(--sp-1); }

  .seq { color: var(--ink-faint); white-space: nowrap; font-variant-numeric: tabular-nums; }
  .seg {
    display: inline-flex;
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    overflow: hidden;
  }
  .seg button { border-radius: 0; padding: 3px var(--sp-2); border: 0; }
  .seg button + button { border-left: 1px solid var(--edge-inst); }
  .seg button.on { background: var(--instrument-raised); color: var(--ink); box-shadow: inset 0 -2px 0 var(--gilt); }
  .speed select {
    font: inherit;
    color: inherit;
    background: var(--instrument-raised);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    padding: 2px var(--sp-1);
  }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }

  /* Narrow board: the timeline takes its own full row. */
  @media (max-width: 900px) {
    .transport {
      grid-template-columns: auto 1fr auto;
      grid-template-areas: 'buttons where settings' 'track track track';
    }
    .seq { display: none; }
  }
</style>
