<script lang="ts">
  import { layoutStore } from '../lib/layoutsettings.svelte';
  import { SCALE_STEP, type LayoutZone } from '../lib/layoutsettings';

  /**
   * ZoneStepper is the on-board resize affordance for one card zone (the
   * fb-20260916T182801Z brief's ask 1): a − / + pair with the zone's current
   * size readout, parked just above the zone container's top edge. It is
   * purely presentational — every press goes through the ONE shared
   * LayoutStore (lib/layoutsettings.svelte.ts), which persists the change
   * and pulses the dotted outline; the parent zone draws the outline itself
   * from store.flash plus its own hover flag (onhover), because a dotted
   * outline on the stepper's ancestor cannot be expressed in CSS (`:has` on
   * a pointer-events:none host is not dependable).
   *
   * The stepper is mounted ONLY on the viewer's own surfaces (Board passes
   * `own` to Quadrant; HandFan is the viewer's own hand by construction) —
   * four copies of one control mutating one shared setting would be noise.
   * The Game Options panel's Layout section edits the same setting, so a
   * spectator-side panel change still lands (and flashes the outline) even
   * where no stepper is mounted.
   */
  let { zone, label, onhover = null }: {
    zone: LayoutZone;
    /** label names the zone in the accessible verbs ("Smaller creature cards"). */
    label: string;
    /** onhover reports the pointer/keyboard presence so the parent can hold the dotted outline open while the control is being used. */
    onhover?: ((hovering: boolean) => void) | null;
  } = $props();

  const pct = $derived(Math.round(layoutStore.scale(zone) * 100));
  function smaller(): void {
    layoutStore.bump(zone, -SCALE_STEP);
  }
  function larger(): void {
    layoutStore.bump(zone, SCALE_STEP);
  }
</script>

<span
  class="zsize"
  data-zone-stepper={zone}
  role="group"
  aria-label="Card size, {label}"
  onpointerenter={() => onhover?.(true)}
  onpointerleave={() => onhover?.(false)}
  onfocusin={() => onhover?.(true)}
  onfocusout={() => onhover?.(false)}
>
  <button type="button" class="zsbtn" data-zone-step="smaller" data-zone-stepper={zone} aria-label="Smaller {label} cards" onclick={smaller}>
    <span aria-hidden="true">−</span>
  </button>
  <span class="zsval" data-zone-scale-readout={zone} aria-hidden="true">{pct}%</span>
  <button type="button" class="zsbtn" data-zone-step="larger" data-zone-stepper={zone} aria-label="Larger {label} cards" onclick={larger}>
    <span aria-hidden="true">+</span>
  </button>
</span>

<style>
  .zsize {
    position: absolute;
    top: calc(-1 * var(--sp-2) - 1.1rem);
    right: 0;
    z-index: 3;
    display: inline-flex;
    align-items: center;
    gap: 1px;
    padding: 1px 2px;
    border-radius: var(--radius);
    background: color-mix(in srgb, var(--felt-sunk) 88%, transparent);
    border: 1px solid var(--edge-felt);
    /* The HandFan host track is pointer-events:none (only card faces claim
       the pointer); the stepper re-enables the pointer on itself so it is
       reachable there too, without making the track clickable. */
    pointer-events: auto;
  }
  .zsbtn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.15rem;
    height: 1.15rem;
    padding: 0;
    border: 0;
    border-radius: 2px;
    background: none;
    color: var(--ink-dim);
    font-family: var(--font-data);
    font-size: var(--t-12);
    line-height: 1;
    cursor: pointer;
  }
  .zsbtn:hover,
  .zsbtn:focus-visible {
    background: var(--instrument-raised);
    color: var(--ink);
    outline: none;
  }
  .zsbtn:focus-visible {
    outline: 2px solid var(--initiative);
    outline-offset: 0;
  }
  .zsval {
    min-width: 2.6em;
    text-align: center;
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    font-size: var(--t-10);
    line-height: 1;
    color: var(--ink-inst);
  }
</style>
