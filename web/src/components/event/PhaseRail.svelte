<script lang="ts">
  import { sealedFormat } from '../../lib/newevent';

  /**
   * The event's phase rail: Draft · Build · Round 1 · Round 2 ·
   * Round 3 · Results. A step is done, now, or still ahead, read from the
   * server's phase string alone. A sealed or prerelease event opens packs
   * instead of drafting them, so its supply step is named for that.
   */
  let { phase, format }: { phase: string; format?: string } = $props();

  const STEPS: { key: string; label: string }[] = [
    { key: 'DRAFT', label: 'Draft' },
    { key: 'BUILD', label: 'Build' },
    { key: 'ROUND1', label: 'Round 1' },
    { key: 'ROUND2', label: 'Round 2' },
    { key: 'ROUND3', label: 'Round 3' },
    { key: 'COMPLETE', label: 'Results' },
  ];
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
