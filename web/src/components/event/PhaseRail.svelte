<script lang="ts">
  import { commanderFormat, sealedFormat } from '../../lib/newevent';

  /**
   * The event's phase rail: Draft · Build · Round 1 · Round 2 ·
   * Round 3 · Results. A step is done, now, or still ahead, read from the
   * server's phase string alone. A sealed or prerelease event opens packs
   * instead of drafting them, so its supply step is named for that. A
   * Commander draft plays `games` free-for-all pod games instead of three
   * rounds of series (ROUND1..ROUNDn, one game each).
   */
  let { phase, format, games = 0 }: { phase: string; format?: string; games?: number } = $props();

  const SERIES_ROUNDS = 3;
  const STEPS = $derived.by((): { key: string; label: string }[] => {
    const pod = commanderFormat(format);
    const n = pod ? Math.max(games, 1) : SERIES_ROUNDS;
    const rounds = Array.from({ length: n }, (_, i) => ({
      key: `ROUND${i + 1}`,
      label: pod ? (n === 1 ? 'Pod game' : `Game ${i + 1}`) : `Round ${i + 1}`,
    }));
    return [{ key: 'DRAFT', label: 'Draft' }, { key: 'BUILD', label: 'Build' }, ...rounds, { key: 'COMPLETE', label: 'Results' }];
  });
  const at = $derived(phase === 'LOBBY' ? -1 : STEPS.findIndex((s) => s.key === phase));
  const packStep = $derived(sealedFormat(format) ? 'Open packs' : 'Draft');
</script>

<ol class="ev-phases" aria-label="Event phases">
  {#each STEPS as s, i (s.key)}
    <li class="ph" class:done={i < at || (phase === 'COMPLETE' && i === at)} class:now={i === at && phase !== 'COMPLETE'} aria-current={i === at ? 'step' : undefined}>
      <i aria-hidden="true"></i>{s.key === 'DRAFT' ? packStep : s.label}
    </li>
  {/each}
</ol>

<style>
  .ph {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    font: 500 var(--t-12) var(--font-data);
    color: var(--ink-faint);
    padding-right: var(--sp-3);
  }
  .ph::after {
    content: '';
    width: 1.2rem;
    height: 1px;
    background: var(--edge-felt);
    margin-left: 0.35rem;
  }
  .ph:last-child::after {
    display: none;
  }
  .ph i {
    width: 0.55rem;
    height: 0.55rem;
    border-radius: 50%;
    border: 1px solid var(--ink-faint);
  }
  .ph.done {
    color: var(--ink-dim);
  }
  .ph.done i {
    background: var(--ink-dim);
    border-color: var(--ink-dim);
  }
  .ph.now {
    color: var(--gilt);
  }
  .ph.now i {
    background: var(--gilt);
    border-color: var(--gilt);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--gilt) 25%, transparent);
  }
</style>
