import type { AnchorRect } from './carddetail.svelte';

/** MenuAnchor is the badge's bounding-rect corners placeMenu needs; it adds
 *  `bottom` to the detail panel's AnchorRect, since the menu hangs either
 *  below or above the badge. */
export interface MenuAnchor {
  left: number;
  top: number;
  right: number;
  bottom: number;
}

/**
 * menuplacement.ts owns the viewport-safe placement of a card's options menu.
 *
 * The menu is portalled to <body> and position-fixed (the same model the
 * hover detail panel uses in CardDetail.svelte): an absolutely-positioned
 * child of the tile inside a scrollable `.quadrant` is clipped by the
 * quadrant's `overflow: auto` — on a tall row a long menu's bottom can be cut
 * off. Portalling the menu out of that scroll container means no ancestor's
 * overflow or containing block can clip it, and placeMenu keeps it on screen.
 *
 * The menu opens DOWN, aligned to the badge's right edge, by default — and
 * FLIPS UP when the space below the card is too tight for even a short menu
 * (a land tile's menu near the board's bottom edge). maxHeight is capped at
 * the space available in the chosen direction so a tall menu scrolls rather
 * than leaving the screen, exactly the contract placePanel gives the detail
 * panel. Pure (vw/vh are passed in), which is what lets the tests pin the
 * viewport guarantees.
 */

export const MENU_WIDTH = 224;
export const MENU_MARGIN = 6;
const MIN_MENU_HEIGHT = 64;

export interface MenuPlacement {
  x: number;
  y: number;
  /** the menu's height cap, so a long list scrolls inside the box instead of
   *  leaving the viewport. */
  maxHeight: number;
  /** true when the menu was flipped above the badge (not enough room below). */
  up: boolean;
}

/** One radial option's fixed-position top-left corner. */
export interface RadialPoint { x: number; y: number }

export const RADIAL_BUTTON = 30;
export const RADIAL_MARGIN = 6;

/**
 * Arrange two through six controls on an inward-opening arc around a badge.
 * The arc points toward the viewport centre (and therefore away from the
 * nearest edge), while each circle is independently clamped as a final guard
 * for very small viewports. Six controls use a slightly larger radius so the
 * 30px touch targets never crowd one another. The radius stays small enough
 * that the wheel reads as anchored to its own card rather than spilling onto
 * a neighbour's — a card tile at play scale is only ~104px wide.
 */
export function placeRadial(anchor: MenuAnchor, count: number, vw: number, vh: number): RadialPoint[] {
  if (count <= 0) return [];
  const cx = (anchor.left + anchor.right) / 2;
  const cy = (anchor.top + anchor.bottom) / 2;
  const dx = vw / 2 - cx;
  const dy = vh / 2 - cy;
  // A cardinal centre-line makes the arc deliberate rather than skewed, and
  // choosing its dominant component gives it the greatest available runway.
  const centre = Math.abs(dx) >= Math.abs(dy)
    ? (dx >= 0 ? 0 : Math.PI)
    : (dy >= 0 ? Math.PI / 2 : -Math.PI / 2);
  const spreads = [0, 0, 54, 90, 120, 144, 160];
  const spread = (spreads[Math.min(count, 6)] * Math.PI) / 180;
  const radius = count === 6 ? 58 : 50;
  const start = centre - spread / 2;

  return Array.from({ length: count }, (_, i) => {
    const angle = count === 1 ? centre : start + spread * i / (count - 1);
    const rawX = cx + Math.cos(angle) * radius - RADIAL_BUTTON / 2;
    const rawY = cy + Math.sin(angle) * radius - RADIAL_BUTTON / 2;
    return {
      x: Math.max(RADIAL_MARGIN, Math.min(rawX, vw - RADIAL_MARGIN - RADIAL_BUTTON)),
      y: Math.max(RADIAL_MARGIN, Math.min(rawY, vh - RADIAL_MARGIN - RADIAL_BUTTON)),
    };
  });
}

export function placeMenu(anchor: MenuAnchor, vw: number, vh: number): MenuPlacement {
  // Right-align the menu to the badge's right edge, clamped to the viewport
  // so a menu on a card near the board's right edge never runs off it.
  let x = anchor.right - MENU_WIDTH;
  x = Math.max(MENU_MARGIN, Math.min(x, vw - MENU_MARGIN - MENU_WIDTH));

  const below = vh - MENU_MARGIN - anchor.bottom;
  if (below >= MIN_MENU_HEIGHT) {
    const y = anchor.bottom + MENU_MARGIN;
    return { x, y, maxHeight: Math.max(MIN_MENU_HEIGHT, below - MENU_MARGIN), up: false };
  }

  // Not enough room below: flip above the badge.
  const above = anchor.top - MENU_MARGIN;
  const maxHeight = Math.max(MIN_MENU_HEIGHT, above - MENU_MARGIN);
  const y = Math.max(MENU_MARGIN, anchor.top - MENU_MARGIN - maxHeight);
  return { x, y, maxHeight, up: true };
}
