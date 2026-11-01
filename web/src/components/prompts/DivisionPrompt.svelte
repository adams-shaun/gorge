<script lang="ts">
  import type { Decision, View } from '../../protocol';
  import type { SeatPanelState } from '../../lib/seatpanel.svelte';
  import { divisionMarks, divisionRowText } from '../../lib/prompts/division';
  import ListPrompt from './ListPrompt.svelte';

  /**
   * DivisionPrompt is the combat damage division renderer: the ordinary
   * numbered rows, with same-named recipients told apart by a letter both in
   * the row and on the recipient's board tile.
   */
  let { decision, view, logic }: { decision: Decision; view: View; logic: SeatPanelState } = $props();

  const marks = $derived(divisionMarks(decision, view));

  $effect(() => {
    const marked = [...marks].flatMap(([obj, mark]) => {
      const el = document.querySelector(`[data-obj="${obj}"]`);
      el?.setAttribute('data-division-mark', mark);
      return el ? [el] : [];
    });
    return () => {
      for (const el of marked) el.removeAttribute('data-division-mark');
    };
  });
</script>

<ListPrompt {decision} {logic} rowText={(o) => divisionRowText(decision, view, o)} />

<style>
  :global([data-division-mark])::after {
    content: attr(data-division-mark);
    position: absolute;
    top: 4px;
    left: 4px;
    z-index: 3;
    min-width: 1.5em;
    padding: 0.1em 0.3em;
    border-radius: 4px;
    background: var(--verdigris, #6fb7ae);
    color: var(--felt-sunk);
    font: 700 var(--t-12)/1.3 var(--font-data);
    text-align: center;
    pointer-events: none;
  }
</style>
