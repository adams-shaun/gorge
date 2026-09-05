<script lang="ts">
  import type { DvrState } from '../lib/dvr';

  /** Transcript is the rules log: one line per event, the cursor's line highlighted and scrolled into view, clicking a line scrubs to it. Lines with no text (state-only events) are skipped. */
  let { dvr, onSeek }: { dvr: DvrState; onSeek: (seq: number) => void } = $props();

  let container: HTMLDivElement | undefined;
  const lines = $derived(dvr.events.filter((e) => e.line));

  $effect(() => {
    const seq = dvr.cursor;
    container?.querySelector<HTMLElement>(`[data-seq="${seq}"]`)?.scrollIntoView({ block: 'nearest' });
  });
</script>

<div class="transcript" bind:this={container}>
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
  .transcript { display: flex; flex-direction: column; height: 100%; }
  .line {
    display: flex; gap: .5rem; text-align: left; background: none; border: none;
    color: inherit; font: inherit; padding: .1rem .5rem; cursor: pointer; width: 100%;
  }
  .line.current { background: #2a2a30; }
  .seq { opacity: .5; font-variant-numeric: tabular-nums; width: 3em; text-align: right; flex: none; }
  .text { overflow-wrap: anywhere; }
</style>
