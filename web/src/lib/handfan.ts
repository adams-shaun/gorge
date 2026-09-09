/**
 * handfan.ts is the pure layout math behind a fan of overlapping card faces —
 * the seated player's own hand, drawn along the bottom edge the way
 * competitive clients draw it (Arena, MTGO, XMage: a horizontal row of full
 * card faces that overlap rather than shrink as the hand grows).
 *
 * The whole question is ONE number: the horizontal step between successive
 * cards' left edges. Step is measured from two constraints that never change:
 * a card is cardWidth wide, and the whole fan must fit maxWidth (the room the
 * viewport actually has), never scroll sideways and never grow without bound.
 * So a small hand gets full card faces plus a small gutter (step = cardWidth +
 * gap); a hand that would overflow gets step shrunk (step < cardWidth, i.e.
 * the faces overlap) by exactly the amount that makes the fan end at
 * maxWidth. Overlap therefore TIGHTENS as the hand grows rather than the row
 * growing — the cards keep their size, only how much of each you can see
 * yields.
 *
 * Everything here is pure (no DOM, no wall clock) so the "does overlap
 * engage" guarantee is unit-testable directly.
 */

export interface HandFanSpec {
  /** width of one card face, px. */
  cardWidth: number;
  /** the gutter between two non-overlapping faces, px. */
  gap: number;
  /** the room the fan may occupy, px — normally the board's client width. */
  maxWidth: number;
}

export interface HandFanLayout {
  /** horizontal offset between successive cards' left edges, px. */
  step: number;
  /** total width of the fan: first card's left edge to last card's right edge, px. */
  rowWidth: number;
  /**
   * how much of each card (except the last, fully visible) is hidden under
   * the one in front: cardWidth - step. Positive means the fan overlaps
   * (large hand); zero means faces sit exactly edge to edge; negative means
   * there is a gap (small hand). "Overlap engages" is `overlap > 0`.
   */
  overlap: number;
}

export function handFanLayout(count: number, spec: HandFanSpec): HandFanLayout {
  const { cardWidth, gap, maxWidth } = spec;
  if (count <= 1) return { step: cardWidth, rowWidth: cardWidth, overlap: 0 };

  // The fan's unconstrained width: every face plus a gutter between each pair.
  const natural = count * cardWidth + (count - 1) * gap;

  // It fits: leave the gutter. step = face + gutter, nothing overlaps.
  if (natural <= maxWidth) {
    const step = cardWidth + gap;
    return { step, rowWidth: step * (count - 1) + cardWidth, overlap: cardWidth - step };
  }

  // It does not fit: the fan must end exactly at the right-hand constraint.
  // The last card's LEFT edge sits at maxWidth - cardWidth, and the count-1
  // gaps between the first card's left edge (0) and that edge are split
  // evenly. That makes `overlap` grow as count grows — the tighten.
  const step = (maxWidth - cardWidth) / (count - 1);
  return { step, rowWidth: step * (count - 1) + cardWidth, overlap: cardWidth - step };
}
