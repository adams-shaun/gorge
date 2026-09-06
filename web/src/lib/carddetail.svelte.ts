/**
 * carddetail.svelte.ts owns the open/close state of the hover detail panel
 * and the viewport-safe placement math for it. The panel itself is
 * CardDetail.svelte; every tile that can show it (CardTile on the board,
 * HandList in the rail) drives one HoverCard and renders CardDetail while
 * `show` is true.
 *
 * Pointer enter arms a dwell timer (DWELL ms); if the pointer stays, the
 * panel opens. Keyboard focus opens immediately. Pointer leave, blur or
 * Escape close it again. Timers are injected so tests are deterministic —
 * a component creates a HoverCard with the real setTimeout, a test creates
 * one with a fake clock.
 */

export const DWELL = 250;

export interface TimerEnv {
  setTimeout: (fn: () => void, ms: number) => unknown;
  clearTimeout: (id: unknown) => void;
}

/** AnchorRect is the tile's bounding-rect corners the panel anchors to; only the three fields placement needs are carried, so a DOMRect or a test's plain object both work. */
export interface AnchorRect {
  left: number;
  top: number;
  right: number;
}

export class HoverCard {
  /** show is true while the detail panel is open. */
  show = $state(false);
  private timer: unknown = null;
  private readonly env: TimerEnv;

  constructor(env: Partial<TimerEnv> = {}) {
    this.env = {
      setTimeout: env.setTimeout ?? ((fn) => setTimeout(fn)),
      clearTimeout: env.clearTimeout ?? ((id) => clearTimeout(id as Parameters<typeof clearTimeout>[0])),
    };
  }

  /** arm starts the pointer dwell; calling it again while armed is a no-op, so a pointerenter never stacks two timers. */
  arm(onOpen?: () => void) {
    if (this.timer !== null) return;
    const cb = onOpen;
    this.timer = this.env.setTimeout(() => {
      this.timer = null;
      this.show = true;
      cb?.();
    }, DWELL);
  }

  /** open shows the panel immediately (keyboard focus path), cancelling any armed dwell. */
  open(onOpen?: () => void) {
    this.cancel();
    this.show = true;
    onOpen?.();
  }

  /** close hides the panel and cancels any armed dwell (pointer leave, blur). */
  close() {
    this.cancel();
    this.show = false;
  }

  /** keydown feeds the panel's keyboard contract: Escape closes it. */
  keydown(e: { key: string }) {
    if (e.key === 'Escape') this.close();
  }

  private cancel() {
    if (this.timer !== null) {
      this.env.clearTimeout(this.timer);
      this.timer = null;
    }
  }
}

export const PANEL_WIDTH = 264;
const PANEL_MARGIN = 8;
const MIN_PANEL_HEIGHT = 160;

export interface PanelPlacement {
  x: number;
  y: number;
  maxHeight: number;
}

/**
 * placePanel positions the fixed detail panel beside `anchor` so it never
 * leaves the viewport. Horizontally it prefers the anchor's right and flips
 * to its left when that would overflow. Vertically it stays top-aligned
 * with the card; when the space below the card is too tight for even a
 * short panel it pulls up, and maxHeight is capped at the space available
 * below the chosen top, so a tall panel scrolls instead of leaving the
 * screen. Pure (vw/vh are passed in), which is what lets the tests pin the
 * viewport guarantees.
 */
export function placePanel(anchor: AnchorRect, vw: number, vh: number): PanelPlacement {
  const topAligned = Math.max(PANEL_MARGIN, anchor.top);
  const y = vh - PANEL_MARGIN - topAligned >= MIN_PANEL_HEIGHT ? topAligned : Math.max(PANEL_MARGIN, vh - PANEL_MARGIN - MIN_PANEL_HEIGHT);
  const maxHeight = Math.max(MIN_PANEL_HEIGHT, vh - PANEL_MARGIN - y);
  let x = anchor.right + PANEL_MARGIN;
  if (x + PANEL_WIDTH > vw - PANEL_MARGIN) x = anchor.left - PANEL_MARGIN - PANEL_WIDTH;
  x = Math.max(PANEL_MARGIN, x);
  return { x, y, maxHeight };
}
