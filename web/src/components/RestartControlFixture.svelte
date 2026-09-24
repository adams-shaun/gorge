<script lang="ts">
  import RestartControl from './RestartControl.svelte';

  /**
   * Browser-test host for the arm/confirm interaction of RestartControl.
   * No production caller: the component is presentational, so this wrapper
   * owns the armed/busy state exactly as Table.svelte does and records the
   * confirm callback count on window for the test to read. It exists only so
   * the real component's two buttons can be clicked in a real browser.
   */
  let confirming = $state(false);
  let busy = $state(false);

  $effect(() => {
    (window as unknown as { __restartProbe?: unknown }).__restartProbe = {
      get confirming() { return confirming; },
      get busy() { return busy; },
      confirms: 0,
    };
  });
</script>

<div data-restart-fixture>
  <RestartControl
    {confirming}
    {busy}
    onArm={() => { confirming = true; }}
    onConfirm={() => {
      const probe = (window as unknown as { __restartProbe: { confirms: number; busy?: boolean } }).__restartProbe;
      probe.confirms += 1;
      busy = true;
    }}
  />
</div>
