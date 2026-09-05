<script lang="ts">
  import type { DvrState } from '../lib/dvr';
  import { behindLive, turnOf } from '../lib/dvr';
  let { dvr, onAction, finished = false }: { dvr: DvrState; onAction: (a: import('../lib/dvr').DvrAction) => void; finished?: boolean } = $props();
  const turn = $derived(turnOf(dvr, dvr.cursor));
</script>

<div class="dvr" data-cursor={dvr.cursor} data-live={dvr.live}>
  <button onclick={() => onAction({ type: 'step', by: -1 })} aria-label="step back">⏮</button>
  {#if dvr.live && !finished}
    <button onclick={() => onAction({ type: 'pause' })} aria-label="pause">⏸</button>
  {:else if !finished}
    <button onclick={() => onAction({ type: 'live' })} aria-label="return to live">▶ live</button>
  {/if}
  <button onclick={() => onAction({ type: 'step', by: 1 })} aria-label="step forward">⏭</button>
  <input type="range" min="0" max={dvr.head} value={dvr.cursor} list="turn-ticks"
    oninput={(e) => onAction({ type: 'scrub', seq: Number((e.target as HTMLInputElement).value) })} aria-label="scrub" />
  <datalist id="turn-ticks">{#each dvr.turnStarts as t (t)}<option value={t}></option>{/each}</datalist>
  <span class="badge" class:live={dvr.live}>
    {#if dvr.live}LIVE{:else}PAUSED · {behindLive(dvr)} behind{/if}
  </span>
  <span class="seq">seq {dvr.cursor} / {dvr.head} · turn {turn + 1}</span>
</div>

<style>
  .dvr { display: flex; gap: .5rem; align-items: center; padding: .25rem .5rem; background: #111; color: #ddd; }
  input[type=range] { flex: 1; }
  .badge { font-weight: 700; color: #f66; }
  .badge.live { color: #6f6; }
</style>
