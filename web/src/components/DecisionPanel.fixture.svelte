<script lang="ts">
  import DecisionPanel from './DecisionPanel.svelte';

  /**
   * Browser-test host for DecisionPanel. No production caller: it owns the
   * cursor the way Table.svelte's DVR does (onScrub moves it) and exposes it
   * on window for the test to read and set.
   */
  let cursor = $state(0);
  $effect(() => {
    (window as unknown as { __reviewProbe?: unknown }).__reviewProbe = {
      get cursor() { return cursor; },
      set cursor(v: number) { cursor = v; },
    };
  });
</script>

<DecisionPanel table="fx" match={1} seats={[{ name: 'Ann', deck: 'red', colour: '#e5484d' }, { name: 'Bot', deck: 'blue', colour: '#3b82f6' }]} {cursor} onScrub={(seq) => (cursor = seq)} />
