import type { DeckReport, DeckReportCard } from '../protocol';

/**
 * deckcheck is the pure wording of the deck-check panel: every string here is
 * assembled from the server's DeckReport alone. The client never judges the
 * deck — it never counts cards, never decides whether a swap is allowed, and
 * shows no rate or score; these helpers only say what the report says.
 */

/** statusLine names the deck in one of exactly three ways, from the server's legal and playable alone. */
export function statusLine(r: DeckReport): string {
  if (!r.legal) return 'Not legal';
  return r.playable ? 'Legal and playable' : 'Legal, but this server cannot play every card';
}

/** substitutionNotice is the consent sentence under a substitution list, or '' when nothing was swapped. The count it shows is the sum of the server's own substitution counts. */
export function substitutionNotice(r: DeckReport): string {
  const k = (r.substituted ?? []).reduce((sum, s) => sum + s.count, 0);
  if (k === 0) return '';
  return `This deck will play with ${k} basic lands in place of the cards listed above. Games with it are not games of the deck as written.`;
}

/** unsupportedLine renders one card the server cannot play and the primitives it needs. */
export function unsupportedLine(c: DeckReportCard): string {
  return `${c.name} ×${c.count} — needs ${c.needs.join(', ')}`;
}
