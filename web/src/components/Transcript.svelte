<script lang="ts">
  import type { DvrState } from '../lib/dvr';
  import { visibleLog } from '../lib/logfilter';

  /**
   * Transcript is the rules log: one line per event, the cursor's line
   * highlighted and scrolled into view, clicking a line scrubs to it. Lines
   * with no text (state-only events) are skipped. The three engine-noise
   * kinds (priority, decision_ask, decision_made) are hidden by default so
   * land plays, casts and triggers surface; the toggle reveals them.
   */
  let { dvr, onSeek }: { dvr: DvrState; onSeek: (seq: number) => void } = $props();

  let container: HTMLDivElement | undefined;
  let revealAll = $state(false);
  const lines = $derived(visibleLog(dvr.events, revealAll));

  $effect(() => {
    // The cursor is always a valid DVR target, but if it landed on a hidden
    // line there is no rendered row to bring into view, so the log simply
    // stays put; revealing all brings that row back and the scroll resumes.
    const seq = dvr.cursor;
    container?.querySelector<HTMLElement>(`[data-seq="${seq}"]`)?.scrollIntoView({ block: 'nearest' });
  });
</script>

<div class="transcript" bind:this={container}>
  <div class="bar">
    <button
      type="button"
      class="toggle"
      aria-pressed={revealAll}
      title="Priority, decision asks and decision answers are engine bookkeeping; they are hidden unless this is on."
      onclick={() => (revealAll = !revealAll)}
    >
      {revealAll ? 'Hide engine noise' : 'Show engine noise'}
    </button>
  </div>
  {#each lines as e (e.event.seq)}
    <button
      type="button"
      class="line"
      class:current={e.event.seq === dvr.cursor}
      data-seq={e.event.seq}
      onclick={() => onSeek(e.event.seq)}
    >
      <span class="seq">{e.event.seq}</span>
      <span class="text">{e.line}</span>
    </button>
  {/each}
</div>

<style>
  .transcript {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  /* The bar used to carry a sentence explaining the default. A permanent
     sentence is a permanent line of a 160px band whose whole job is log
     lines, and the button already says what it does; the explanation moved
     to the button's own title, where it is one hover away and costs nothing
     when it is not wanted. */
  .bar {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--sp-3);
    padding: var(--sp-1) var(--sp-3);
    border-bottom: 1px solid var(--edge-inst);
    font-size: .72rem;
    flex: none;
  }
  .toggle {
    background: none;
    border: 1px solid var(--edge-inst);
    border-radius: 4px;
    color: var(--ink-inst);
    font: inherit;
    padding: 2px var(--sp-3);
    cursor: pointer;
    flex: none;
  }
  .toggle:hover {
    border-color: var(--ink-dim);
    color: var(--ink);
  }
  .line {
    display: flex;
    gap: var(--sp-3);
    text-align: left;
    background: none;
    border: none;
    color: var(--ink-dim);
    font: inherit;
    padding: 1px var(--sp-3);
    cursor: pointer;
    width: 100%;
  }
  .line:hover {
    color: var(--ink-inst);
  }
  /* The cursor line is where the DVR is pointing: marked by a rule in the
     initiative colour, the same device the pending tray uses, rather than by
     a filled band that would fight the log's density. */
  .line.current {
    color: var(--ink);
    background: var(--instrument-raised);
    box-shadow: inset 2px 0 0 var(--initiative);
  }
  .seq {
    color: var(--ink-faint);
    font-variant-numeric: tabular-nums;
    width: 3.5em;
    text-align: right;
    flex: none;
  }
  .text {
    overflow-wrap: anywhere;
  }
</style>
