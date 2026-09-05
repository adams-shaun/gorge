<script lang="ts">
  import type { View } from '../protocol';
  import { arrowsFor } from '../lib/arrows';
  import type { Arrow, End } from '../lib/arrows';

  /**
   * Arrows overlays the board with one line per arrowsFor(view) result. It
   * has no rules knowledge itself — arrows.ts already decided which ends to
   * connect; this component only resolves each End to its DOM anchor
   * ([data-obj] / [data-seat] — some of which live outside the board's own
   * subtree: the stack is rendered in the rail, identity bars are siblings
   * of Board) and draws a line relative to this overlay's own box. A
   * missing anchor (not rendered yet, or already gone) simply draws
   * nothing for that arrow rather than throwing.
   */
  let { view }: { view: View } = $props();

  interface Line { x1: number; y1: number; x2: number; y2: number; kind: Arrow['kind'] }

  const STROKE: Record<Arrow['kind'], string> = {
    target: '#f5a524',
    attack: '#e5484d',
    block: '#3b82f6',
  };

  let root = $state<HTMLDivElement | undefined>(undefined);
  let lines = $state<Line[]>([]);

  function anchorEl(end: End): Element | null {
    return 'obj' in end ? document.querySelector(`[data-obj="${end.obj}"]`) : document.querySelector(`[data-seat="${end.seat}"]`);
  }

  function centre(el: Element, base: DOMRect): { x: number; y: number } {
    const r = el.getBoundingClientRect();
    return { x: r.left + r.width / 2 - base.left, y: r.top + r.height / 2 - base.top };
  }

  function recompute(): void {
    if (!root) return;
    const base = root.getBoundingClientRect();
    const next: Line[] = [];
    for (const arrow of arrowsFor(view)) {
      const from = anchorEl(arrow.from);
      const to = anchorEl(arrow.to);
      if (!from || !to) continue;
      const a = centre(from, base);
      const b = centre(to, base);
      next.push({ x1: a.x, y1: a.y, x2: b.x, y2: b.y, kind: arrow.kind });
    }
    lines = next;
  }

  // Redraw whenever the view changes. requestAnimationFrame defers the
  // measurement past this tick's DOM update so the new frame's tiles have
  // laid out before we read their positions.
  $effect(() => {
    void view;
    const id = requestAnimationFrame(recompute);
    return () => cancelAnimationFrame(id);
  });

  // Redraw on layout changes that don't themselves change `view` — window
  // resize, a panel collapsing, fonts loading in — anything that moves the
  // board or its anchors without a new frame arriving.
  $effect(() => {
    if (!root) return;
    const ro = new ResizeObserver(() => recompute());
    ro.observe(root);
    return () => ro.disconnect();
  });
</script>

<div class="arrows" bind:this={root}>
  <svg>
    <defs>
      <marker id="arrow-target" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
        <path d="M0,0 L10,5 L0,10 z" fill={STROKE.target} />
      </marker>
      <marker id="arrow-attack" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
        <path d="M0,0 L10,5 L0,10 z" fill={STROKE.attack} />
      </marker>
      <marker id="arrow-block" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
        <path d="M0,0 L10,5 L0,10 z" fill={STROKE.block} />
      </marker>
    </defs>
    {#each lines as l, i (i)}
      <line x1={l.x1} y1={l.y1} x2={l.x2} y2={l.y2} stroke={STROKE[l.kind]} stroke-width="2" marker-end={`url(#arrow-${l.kind})`} />
    {/each}
  </svg>
</div>

<style>
  .arrows { position: absolute; inset: 0; pointer-events: none; overflow: visible; }
  svg { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; }
</style>
