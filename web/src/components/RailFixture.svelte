<script lang="ts">
  import type { View, SeatInfo, DecisionBody } from '../protocol';
  import Rail from './Rail.svelte';
  import ConcedeControl from './ConcedeControl.svelte';

  /**
   * Geometry-fixture host for the whole rail (SeatTable.geometry.ts mounts it
   * into #rail). It exists only so the fixture can render Rail with the SAME
   * logbar snippet the live route uses — a ConcedeControl passed as Rail's
   * `logbar` snippet, exactly as Table.svelte passes it (task fb-53bd45b9) —
   * so the harness measures the real rendered control, not a probe button
   * pasted into the row. No production caller; not used by any route.
   */
  let {
    view,
    seats,
    decision,
    emphasizeTop = false,
    showLog = true,
    onToggleLog = null,
    /** 'none' renders no concede control (the fixture's default, matching
     *  every rail with nothing pending, byte-identical to mounting Rail
     *  directly); 'idle' renders the unarmed Concede button (the control's
     *  first-paint state); 'confirm' renders the armed "Concede — confirm"
     *  button — the widest state the control ever takes. */
    concede = 'none',
  }: {
    view: View;
    seats: SeatInfo[];
    decision: DecisionBody | null;
    emphasizeTop?: boolean;
    showLog?: boolean;
    onToggleLog?: (() => void) | null;
    concede?: 'none' | 'idle' | 'confirm';
  } = $props();
</script>

{#if concede === 'none'}
  <Rail {view} {seats} {decision} {emphasizeTop} {showLog} {onToggleLog} />
{:else}
  <Rail {view} {seats} {decision} {emphasizeTop} {showLog} {onToggleLog}>
    {#snippet logbar()}
      <ConcedeControl confirming={concede === 'confirm'} onArm={() => {}} onConfirm={() => {}} />
    {/snippet}
  </Rail>
{/if}
