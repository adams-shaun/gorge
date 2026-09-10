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
  /**
   * The object the panel is (or would be) describing — learned from arm/open
   * when the caller knows whose panel this is, so a surface can later prove
   * an open panel describes an object it is no longer rendering (see
   * superviseRendering). Null for callers that never pass one (HandList
   * manages its subject through supervise instead).
   */
  private showingId: number | null = null;
  private timer: unknown = null;
  private readonly env: TimerEnv;

  constructor(env: Partial<TimerEnv> = {}) {
    this.env = {
      setTimeout: env.setTimeout ?? ((fn) => setTimeout(fn)),
      clearTimeout: env.clearTimeout ?? ((id) => clearTimeout(id as Parameters<typeof clearTimeout>[0])),
    };
  }

  /**
   * arm starts the pointer dwell; calling it again while armed is a no-op, so
   * a pointerenter never stacks two timers. objectId is the object the panel
   * is being prepared for — HoverCard remembers it as the panel's subject, so
   * a surface that later re-renders a different object can prove the open
   * panel describes the wrong one. A callback-only call (HandList's dwell) is
   * accepted for surfaces that manage the subject themselves via supervise.
   */
  arm(objectIdOrOnOpen?: (() => void) | number | null | undefined, onOpen?: () => void) {
    const objectId = typeof objectIdOrOnOpen === 'function' ? null : (objectIdOrOnOpen ?? null);
    const cb = typeof objectIdOrOnOpen === 'function' ? objectIdOrOnOpen : onOpen;
    if (objectId !== null) this.showingId = objectId;
    if (this.timer !== null) return;
    this.timer = this.env.setTimeout(() => {
      this.timer = null;
      this.show = true;
      cb?.();
    }, DWELL);
  }

  /**
   * open shows the panel immediately (keyboard focus path), cancelling any
   * armed dwell. objectId, when given, is remembered as the panel's subject
   * (same contract as arm).
   */
  open(objectIdOrOnOpen?: (() => void) | number | null | undefined, onOpen?: () => void) {
    const objectId = typeof objectIdOrOnOpen === 'function' ? null : (objectIdOrOnOpen ?? null);
    const cb = typeof objectIdOrOnOpen === 'function' ? objectIdOrOnOpen : onOpen;
    if (objectId !== null) this.showingId = objectId;
    this.cancel();
    this.show = true;
    cb?.();
  }

  /** close hides the panel and cancels any armed dwell (pointer leave, blur). */
  close() {
    this.cancel();
    this.show = false;
  }

  /**
   * supervise is the lifecycle half of the tight contract: a trigger can be
   * removed from the DOM *while the pointer is still over it* — a card played
   * from hand, a permanent destroyed — and a removed element never fires
   * pointerleave, so the panel would be left open on an object that no longer
   * exists, its artwork hanging on screen attached to nothing. Every surface
   * therefore re-feeds the objects it currently shows through this whenever
   * that set changes; when the object the panel describes is no longer among
   * them the panel closes without any pointer event being delivered. Returns
   * true when a live panel was closed (the object went away), false when the
   * object stays present or the panel was not open.
   */
  supervise(objectId: number | null | undefined, present: readonly { id: number }[]): boolean {
    if (objectId === null || objectId === undefined) return false;
    if (present.some((c) => c.id === objectId)) return false;
    const wasOpen = this.show;
    this.close();
    return wasOpen;
  }

  /**
   * superviseRendering is the rendering half of the same lifecycle contract:
   * on the board Svelte KEEPS a CardTile instance alive when the object under
   * it changes (a permanent dies, the tile is handed the next object) and the
   * pointer never leaves — no pointerleave fires, so nothing would close the
   * panel, and worse, the panel would describe a card the reader never asked
   * for. Every surface therefore re-feeds the object id it is currently
   * rendering whenever its card prop changes: when the panel is open for an
   * object that is no longer the one being rendered, it closes — the panel's
   * lifetime follows the object it was opened for, not the mounting. An
   * armed-but-not-open dwell is re-pointed at the object being rendered now,
   * so the timer can never open a panel labelled for a stale id. Returns true
   * when a live panel was closed.
   */
  superviseRendering(objectId: number | null | undefined): boolean {
    if (objectId === null || objectId === undefined) return false;
    if (this.showingId === objectId) return false;
    const wasOpen = this.show;
    // close() also cancels any timer — but a timer can never be pending while
    // the panel is open (arm/open/close keep those mutually exclusive), so
    // when the panel is closed this branch is skipped entirely and a pending
    // dwell survives, re-pointed below.
    if (wasOpen) this.close();
    this.showingId = objectId;
    return wasOpen;
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

// The single-column ledger keeps its established outer width. A resolved
// plate adds its natural card width beside the ledger, rather than shrinking
// either register to make a narrower imitation of both.
export const PANEL_WIDTH = 264;
export const PLATE_WIDTH = 220;
export const PANEL_COLUMN_GAP = 8;
export const PANEL_WIDTH_WITH_PLATE = PANEL_WIDTH + PLATE_WIDTH + PANEL_COLUMN_GAP;
export const MIN_SIDE_BY_SIDE_VIEWPORT = 800;
export const PANEL_MARGIN = 8;
export const MIN_PANEL_HEIGHT = 160;

export interface DetailColumnRect {
  left: number;
  right: number;
}

export interface DetailLayout {
  width: number;
  sideBySide: boolean;
  /** Relative panel rectangles; absent plate means the ledger owns the panel. */
  plate?: DetailColumnRect;
  ledger: DetailColumnRect;
}

/**
 * The card and ledger share a row only where a full-width panel is useful:
 * 800px and up. Below that the panel returns to its established stacked
 * surface, avoiding both an off-screen panel and a cropped plate.
 */
export function detailLayout(hasPlate: boolean, viewportWidth: number): DetailLayout {
  const sideBySide = hasPlate && viewportWidth >= MIN_SIDE_BY_SIDE_VIEWPORT;
  const padding = 12; // --sp-3, kept here so the geometry contract is testable.
  if (!sideBySide) {
    return { width: PANEL_WIDTH, sideBySide, ledger: { left: padding, right: PANEL_WIDTH - padding } };
  }
  const plate = { left: padding, right: padding + PLATE_WIDTH };
  return {
    width: PANEL_WIDTH_WITH_PLATE,
    sideBySide,
    plate,
    ledger: { left: plate.right + PANEL_COLUMN_GAP, right: PANEL_WIDTH_WITH_PLATE - padding },
  };
}

export interface PanelPlacement {
  x: number;
  /** top is set when the panel opens DOWNWARD (grows down from the card's top). */
  y?: number;
  /** bottom is set when the panel opens UPWARD (grows up from just above the card). Exactly one of y/bottom is set. */
  bottom?: number;
  maxHeight: number;
}

/**
 * placePanel positions the fixed detail panel beside `anchor` so it never
 * leaves the viewport. Horizontally it prefers the anchor's right and flips
 * to its left when that would overflow.
 *
 * Vertically it opens on the ROOMIER side: the panel is top-aligned with the
 * card and grows DOWNWARD when there is at least as much room below the card
 * as above it, and flips to be bottom-anchored just above the card and grows
 * UPWARD when there is more room above. The flip is what keeps a hand card's
 * detail from being cut off: a hand card sits near the bottom of the board,
 * where the room below it is a fixed, cramped band (just the fan's own
 * height and a little margin), so a panel that always opened downward was
 * capped at that band and scrolled — the card's lower reading unreachable.
 * Opening upward hands the panel the tall space above the card instead. In
 * both directions maxHeight is the room available on the chosen side, so a
 * panel is never shorter than it needs to be and never leaves the screen.
 * Pure (vw/vh are passed in), which is what lets the tests pin the viewport
 * guarantees.
 */
export function placePanel(anchor: AnchorRect, vw: number, vh: number, panelWidth = PANEL_WIDTH): PanelPlacement {
  const roomBelow = vh - PANEL_MARGIN - anchor.top;
  const roomAbove = anchor.top - PANEL_MARGIN;

  let x = anchor.right + PANEL_MARGIN;
  if (x + panelWidth > vw - PANEL_MARGIN) x = anchor.left - PANEL_MARGIN - panelWidth;
  // A wide, two-column surface may fit in the viewport but not wholly on
  // either side of a centrally placed anchor. Clamp both edges in that case;
  // it may overlap the anchor, but never leaves the viewport.
  x = Math.max(PANEL_MARGIN, Math.min(x, vw - PANEL_MARGIN - panelWidth));

  if (roomBelow >= roomAbove) {
    // Open downward (top-aligned with the card), pulling up when the space
    // below is too short for even a minimal panel. maxHeight is exactly the
    // room that remains below the chosen top, so the panel can fill it
    // without ever leaving the viewport.
    const topAligned = Math.max(PANEL_MARGIN, anchor.top);
    const y = vh - PANEL_MARGIN - topAligned >= MIN_PANEL_HEIGHT ? topAligned : Math.max(PANEL_MARGIN, vh - PANEL_MARGIN - MIN_PANEL_HEIGHT);
    const maxHeight = Math.max(MIN_PANEL_HEIGHT, vh - PANEL_MARGIN - y);
    return { x, y, maxHeight };
  }

  // Open upward: the roomier side is above the card. Anchor the panel's
  // bottom edge PANEL_MARGIN above the card's top and let it grow up into the
  // room above; maxHeight is the space from the viewport's top margin to that
  // edge, so the panel fills the room above without ever clipping at the top.
  const bottom = vh - (anchor.top - PANEL_MARGIN);
  const maxHeight = Math.max(MIN_PANEL_HEIGHT, anchor.top - 2 * PANEL_MARGIN);
  return { x, bottom, maxHeight };
}
