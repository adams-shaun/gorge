/**
 * oracletext.ts tokenises Scryfall-style oracle text into interleaved text
 * runs and renderable symbols — the piece the hover panel's printed-card
 * paragraph was missing. The catalog's oracle text arrives verbatim from a
 * Scryfall named lookup (cmd/gorged/art.go stores the six printed facts
 * as-is), so its symbols are in Scryfall's braced notation: `{B}`, `{T}`,
 * `{2/B}`, `{W/P}`. Nothing else in the paragraph tokenises those braces,
 * which is why the popup alone read like debug output while the transcript
 * and the feed drew the same token shape as pips.
 *
 * ONE CLASSIFIER. Mana-family tokens are classified by lib/mana.ts's own
 * parseSymbol — the exact classifier the log renderers reach through
 * ManaSymbols — so the popup and the transcript draw identical pips for the
 * same token. This module adds only the brace splitting and the non-mana
 * icons; it must never grow a second colour table.
 *
 * WHAT THE TOKENS ARE (census over 700 live Scryfall creature cards,
 * 2026-09-14): plain single-char pips ({W}..{G}, {C}, {S}, digits, {X} and
 * friends), two-colour hybrids ({W/B}), twobrid ({2/W}), Phyrexian ({U/P}),
 * plus the non-mana icons {T} (tap), {Q} (untap) and {E} (energy), and the
 * Un-set oddballs {A}/{TK} that only Un-cards carry. A token the classifier
 * does not recognise — {TK}, {A}, {∞}, {HR} — stays its literal braced text:
 * never dropped, never guessed at.
 *
 * WHITESPACE IS CONTRACT. An oracle text is multiple lines and the paragraph
 * relies on `white-space: pre-wrap` to keep its line breaks, so every
 * character between tokens — spaces and newlines alike — is preserved verbatim
 * in the text runs. The tokenizer never trims, never normalises, never joins.
 */

import { parseSymbol, type ManaSymbol } from './mana';

/**
 * One renderable piece of oracle text. A `mana` segment carries the parsed
 * symbol (renderer renders it through ManaSymbols, exactly as a transcript
 * mana piece does); `tap`/`untap`/`energy` are the non-mana icons, given
 * their own segment kinds rather than a mana `kind: 'unknown'` so a renderer
 * can draw them as icons and never confuse them with mana.
 */
export type OracleSegment =
  | { kind: 'text'; text: string }
  | { kind: 'mana'; symbol: ManaSymbol }
  | { kind: 'tap' }
  | { kind: 'untap' }
  | { kind: 'energy' };

/** Scryfall prints every symbol, mana or icon, inside braces. */
const SYMBOL = /\{[^{}]+\}/g;

/**
 * oracleSegments splits oracle text into text runs and symbol segments.
 * Text runs preserve every character between tokens verbatim (pre-wrap's
 * line breaks live in them); a braced token becomes a mana segment through
 * lib/mana.ts's classifier, an icon segment for {T}/{Q}/{E}, or — for a
 * token the classifier does not know — its literal braced text.
 */
export function oracleSegments(text: string): OracleSegment[] {
  if (!text) return [];
  const out: OracleSegment[] = [];
  let last = 0;
  for (const m of text.matchAll(SYMBOL)) {
    if (m.index > last) out.push({ kind: 'text', text: text.slice(last, m.index) });
    const inner = m[0].slice(1, -1);
    if (inner === 'T') out.push({ kind: 'tap' });
    else if (inner === 'Q') out.push({ kind: 'untap' });
    else if (inner === 'E') out.push({ kind: 'energy' });
    else {
      const symbol = parseSymbol(inner);
      // Unknown to the classifier: keep the braces, literally. {TK} and {A}
      // (Un-set) and anything unrecognised render as themselves.
      if (symbol.kind === 'unknown') out.push({ kind: 'text', text: m[0] });
      else out.push({ kind: 'mana', symbol });
    }
    last = m.index + m[0].length;
  }
  if (last < text.length) out.push({ kind: 'text', text: text.slice(last) });
  return out;
}
