<script module lang="ts">
  export function railWidthAfterArrow(side: 'left' | 'right', key: string, width: number): number {
    const wider = side === 'left' ? key === 'ArrowRight' : key === 'ArrowLeft';
    return width + (wider ? 0.02 : -0.02);
  }
</script>

<script lang="ts">
  import { RAIL_MAX, RAIL_MIN } from '../lib/layoutprofile';

  let { side, width, onWidth, onDrag, onReset, min = RAIL_MIN, max = RAIL_MAX, name = 'Rail' }: {
    /** the side of the table the resized rail sits on; the handle is on its inner edge */
    side: 'left' | 'right';
    width: number;
    onWidth: (width: number) => void;
    onDrag: (width: number | null) => void;
    onReset: () => void;
    min?: number;
    max?: number;
    /** what the separator resizes, for its accessible name */
    name?: string;
  } = $props();
  let el = $state<HTMLElement | null>(null);
  let dragging = $state(false);
  let pointer: number | null = null;
  let draft = $state(0);
  const shown = $derived(dragging ? draft : width);

  function move(e: PointerEvent): void {
    if (pointer !== e.pointerId || window.innerWidth <= 0) return;
    const edge = side === 'left' ? e.clientX : window.innerWidth - e.clientX;
    draft = Math.min(max, Math.max(min, edge / window.innerWidth));
    onDrag(draft);
  }
  function start(e: PointerEvent): void {
    if (e.button !== 0 || el === null) return;
    pointer = e.pointerId;
    dragging = true;
    draft = width;
    el.setPointerCapture(e.pointerId);
  }
  function end(e: PointerEvent): void {
    if (pointer !== e.pointerId) return;
    move(e);
    pointer = null;
    dragging = false;
    onDrag(null);
    onWidth(draft);
  }
  function cancel(e: PointerEvent): void {
    if (pointer !== e.pointerId) return;
    pointer = null;
    dragging = false;
    onDrag(null);
  }
  function key(e: KeyboardEvent): void {
    if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
      onWidth(railWidthAfterArrow(side, e.key, width));
      e.preventDefault();
    } else if (e.key === 'Home') {
      onReset();
      e.preventDefault();
    }
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<span
  bind:this={el}
  class="rail-resizer"
  class:dragging
  role="separator"
  tabindex="0"
  aria-orientation="vertical"
  aria-label="{name} width: drag, or use arrow keys; Home resets"
  aria-valuemin={Math.round(min * 100)}
  aria-valuemax={Math.round(max * 100)}
  aria-valuenow={Math.round(shown * 100)}
  data-rail-resizer
  data-side={side}
  onpointerdown={start}
  onpointermove={move}
  onpointerup={end}
  onpointercancel={cancel}
  onkeydown={key}
  ondblclick={onReset}
></span>

<style>
  .rail-resizer {
    position: absolute;
    z-index: 35;
    top: 0;
    bottom: 0;
    width: 12px;
    cursor: col-resize;
    touch-action: none;
  }
  .rail-resizer[data-side='right'] { left: -6px; }
  .rail-resizer[data-side='left'] { right: -6px; }
  .rail-resizer:focus-visible { outline: 2px solid var(--gilt); outline-offset: -2px; }
  .rail-resizer.dragging { background: color-mix(in srgb, var(--gilt) 30%, transparent); }
</style>
