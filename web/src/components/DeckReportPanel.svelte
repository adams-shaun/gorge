<script lang="ts">
  import type { DeckEngineVerdict, DeckReport } from '../protocol';
  import { statusLine, substitutionNotice, unsupportedLine } from '../lib/deckcheck';

  // DeckReportPanel renders the server's DeckReport verbatim: status, the
  // construction problems, the unknown names, the cards this server cannot
  // play, the basic-land swaps consented to, and the second engine's verdict
  // when the server asked one. It judges nothing itself — no legality call,
  // no card counting, no rate — and the consent checkbox is the only input:
  // it flips the parent-bound substitute so the check can be re-posted.
  interface Props {
    report: DeckReport | null;
    substitute?: boolean;
  }

  let { report, substitute = $bindable(false) }: Props = $props();

  const notice = $derived(report ? substitutionNotice(report) : '');

  function diracLine(d: DeckEngineVerdict): string {
    if (d.playable) return `dirac: playable on ${d.set}`;
    const names = d.unsupported.map((c) => c.name).join(', ');
    return names.length === 0 ? `dirac: cannot play ${d.unsupported.length} cards` : `dirac: cannot play ${d.unsupported.length} cards: ${names}`;
  }
</script>

{#if report}
  <div class="deck-report" data-testid="deck-report">
    <p class="status">{statusLine(report)}</p>
    {#each report.problems as p, i (i)}
      <p class="problem" data-testid="deck-report-problem">{p}</p>
    {/each}
    {#each report.unknown as u, i (i)}
      <p class="unknown">Not a card we know: {u}</p>
    {/each}
    {#each report.unsupported as c (c.name)}
      <p class="unsupported" data-testid="deck-report-unsupported">{unsupportedLine(c)}</p>
    {/each}
    {#each report.substituted as s (s.out)}
      <p class="substituted" data-testid="deck-report-substituted">{s.out} → {s.in} ×{s.count}</p>
    {/each}
    {#if notice}<p class="notice">{notice}</p>{/if}
    {#if report.dirac}<p class="dirac">{diracLine(report.dirac)}</p>{/if}
    {#if report.unsupported.length > 0}
      <label class="consent">
        <input type="checkbox" data-testid="deck-substitute" bind:checked={substitute} />
        Replace the cards this server cannot play with basic lands (at most 3)
      </label>
    {/if}
  </div>
{/if}

<style>
  .deck-report {
    display: grid;
    gap: var(--sp-1);
    padding: var(--sp-2);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    background: var(--instrument-raised);
    font-size: var(--t-12);
  }
  .status {
    margin: 0;
    font-weight: 600;
  }
  .problem,
  .unknown,
  .unsupported,
  .substituted,
  .notice,
  .dirac {
    margin: 0;
    color: var(--ink-dim);
  }
  .notice {
    color: var(--ink-inst);
  }
  .consent {
    display: flex;
    align-items: baseline;
    gap: var(--sp-1);
    color: var(--ink-dim);
  }
</style>
