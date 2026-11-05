import type { PartnerPlacement, RailSide } from './layoutprofile';

/**
 * partnerrail decides where the paired bot's panel goes. The table route
 * draws the main rail as before; when the table offers a paired bot, the
 * panel gets its own rail on the side away from the main one, unless the
 * layout profile keeps it in the main rail or tucks it into an edge drawer.
 *
 * Below NARROW_QUERY's width a second column would squeeze the board, so the
 * panel falls back into the main rail's flow there. A drawer takes no column
 * and stays a drawer.
 */

/** The width below which the paired bot never takes a column of its own. */
export const NARROW_QUERY = '(max-width: 64rem)';

export type PartnerLayout =
  /** inside the main rail, where it always lived */
  | { where: 'main' }
  /** its own grid column on that side of the board */
  | { where: 'column'; side: 'left' | 'right' }
  /** an edge drawer on that side, sliding in on hover or focus */
  | { where: 'drawer'; side: 'left' | 'right' };

/**
 * partnerLayout places the panel. offered is whether the table offers a
 * paired bot; without one the panel stays in the main rail, where it shows
 * the server's reason or nothing, so the layout is the one without a partner.
 */
export function partnerLayout(placement: PartnerPlacement, railSide: RailSide, offered: boolean, narrow: boolean): PartnerLayout {
  if (!offered || placement === 'same') return { where: 'main' };
  // A hidden main rail is a drawer on the right, so the partner goes left.
  const side = railSide === 'left' ? 'right' : 'left';
  if (placement === 'hidden') return { where: 'drawer', side };
  return narrow ? { where: 'main' } : { where: 'column', side };
}
