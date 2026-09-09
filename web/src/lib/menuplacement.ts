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
