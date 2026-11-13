/**
 * draftcards.ts holds the draft and deckbuilder screens' DISPLAY helpers:
 * mana-value columns, colour chips, pass direction and who opened a pack.
 *
 * Nothing here decides legality. The facts a card is grouped and tinted by
 * — mana value, colours, printed cost, type line — come from the server on
 * the wire (protocol.DraftCard), read from the same corpus the engine
 * plays; the client never looks them up or computes them on its own. A card
 * the server's corpus does not know carries none of them and lands in a "?"
 * column rather than being guessed at, and the server's DeckCheck stays the
 * only word on whether a deck is legal.
 */

import type { CardView, DraftCard } from '../protocol';
import type { ManaColour } from './mana';

export const COLOURS: readonly ManaColour[] = ['W', 'U', 'B', 'R', 'G'];
export const BASIC_NAMES = ['Plains', 'Island', 'Swamp', 'Mountain', 'Forest'] as const;

/** CardFacts is what the screens group and tint by. mv is null while unknown. */
export interface CardFacts {
  mv: number | null;
  colours: ManaColour[];
  land: boolean;
  creature: boolean;
  /** Forge-notation cost ("2 W") for ManaSymbols; "" when none or unknown. */
  cost: string;
  types: string;
}

export const UNKNOWN_FACTS: CardFacts = { mv: null, colours: [], land: false, creature: false, cost: '', types: '' };

/**
 * costTokens splits a printed cost into its symbols. It reads both the
 * catalog's brace form ("{2}{W}{U/B}") and Forge's spaced form ("2 W U/B").
 */
export function costTokens(cost: string | undefined): string[] {
  if (!cost) return [];
  const c = cost.trim();
  if (c === '' || /^no cost$/i.test(c)) return [];
  if (c.includes('{')) {
    // A split/adventure card's catalog cost joins faces with " // ": the
    // front face's half is the one a deckbuilder sorts by.
    const front = c.split('//')[0];
    return [...front.matchAll(/\{([^}]+)\}/g)].map((m) => m[1].toUpperCase());
  }
  return c.split(/\s+/).map((t) => t.toUpperCase());
}

/** forgeCost renders tokens in the Forge notation ManaSymbols draws. */
export function forgeCost(tokens: string[]): string {
  return tokens.join(' ');
}

/**
 * factsOf reads one card's display facts off the wire. The server omits
 * every fact for a card its corpus does not know (no type line), so that
 * card's mana value is null — the "?" column. A known card with no
 * mana_value is a zero (the field is omitempty): a land, or a free spell.
 * Colours are the server's colour identity, a subset of "WUBRG".
 */
export function factsOf(c: DraftCard): CardFacts {
  const types = c.type_line ?? '';
  const known = types !== '' || c.mana_cost !== undefined || c.mana_value !== undefined;
  if (!known) return UNKNOWN_FACTS;
  const front = types.split('//')[0];
  const colours = c.colors ?? '';
  return {
    mv: c.mana_value ?? 0,
    colours: COLOURS.filter((col) => colours.includes(col)),
    land: /\bLand\b/.test(front),
    creature: /\bCreature\b/.test(front),
    cost: forgeCost(costTokens(c.mana_cost)),
    types,
  };
}

/** One mana-value column of a pool or deck. */
export interface Column<T> {
  key: string;
  label: string;
  cards: T[];
}

const MV_LABELS = ['0–1', '2', '3', '4', '5', '6+'];

/**
 * mvColumns buckets cards into the mockup's columns: 0–1, 2, 3, 4, 5, 6+,
 * Land, and — only when some card arrived with no facts (the server's
 * corpus does not know it) — "?". Order
 * inside a column is the input order, so a pool reads in pick order.
 */
export function mvColumns<T>(cards: T[], factsOf: (c: T) => CardFacts): Column<T>[] {
  const cols: Column<T>[] = MV_LABELS.map((label, i) => ({ key: `mv${i}`, label, cards: [] }));
  const land: Column<T> = { key: 'land', label: 'Land', cards: [] };
  const unknown: Column<T> = { key: 'unknown', label: '?', cards: [] };
  for (const c of cards) {
    const f = factsOf(c);
    if (f.land) land.cards.push(c);
    else if (f.mv === null) unknown.cards.push(c);
    else cols[Math.min(Math.max(f.mv, 1), 6) - 1].cards.push(c);
  }
  return unknown.cards.length > 0 ? [...cols, land, unknown] : [...cols, land];
}

/** ColourFilter is one deckbuilder chip. */
export type ColourFilter = 'all' | ManaColour | 'multi' | 'colourless';

export const COLOUR_FILTERS: readonly { key: ColourFilter; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'W', label: 'W' },
  { key: 'U', label: 'U' },
  { key: 'B', label: 'B' },
  { key: 'R', label: 'R' },
  { key: 'G', label: 'G' },
  { key: 'multi', label: 'Multi' },
  { key: 'colourless', label: 'Colourless' },
];

export function passesFilter(f: ColourFilter, facts: CardFacts): boolean {
  switch (f) {
    case 'all': return true;
    case 'multi': return facts.colours.length > 1;
    case 'colourless': return facts.colours.length === 0;
    default: return facts.colours.includes(f);
  }
}

/** tint is the card's colour class: one colour letter, M for multicolour, C for colourless or unknown. */
export function tint(facts: CardFacts): ManaColour | 'M' | 'C' {
  if (facts.colours.length > 1) return 'M';
  return facts.colours[0] ?? 'C';
}

/** passDirection is the pass rule: packs 1 and 3 pass left (seat i to i+1), pack 2 passes right. */
export function passDirection(round: number): 'left' | 'right' {
  return round === 2 ? 'right' : 'left';
}

/**
 * openedBy is the seat whose pack this is: a pack moves one seat per pass,
 * and a pass is picksPerPass picks (one outside a Commander draft), so at
 * pick p it has travelled floor((p-1)/picksPerPass) seats against the pass
 * direction.
 */
export function openedBy(seat: number, round: number, pickNo: number, seats = 4, picksPerPass = 1): number {
  const steps = Math.floor(Math.max(pickNo - 1, 0) / Math.max(picksPerPass, 1)) % seats;
  return passDirection(round) === 'left' ? (seat - steps + seats) % seats : (seat + steps) % seats;
}

/** cardView adapts a DraftCard to the CardView CardImage renders (art by name; the blank face shows the wire's cost and type line). */
export function cardView(c: DraftCard): CardView {
  const facts = factsOf(c);
  return {
    id: 0,
    name: c.name,
    types: facts.types,
    mana_cost: facts.cost || undefined,
    printing: { name: c.name },
  } as unknown as CardView;
}

/** rarityLabel spells a DraftCard rarity letter. */
export function rarityLabel(r: string): string {
  return ({ C: 'common', U: 'uncommon', R: 'rare', M: 'mythic', L: 'land', S: 'special' } as Record<string, string>)[r] ?? r;
}

/** seatLabel names a seat for the event screens: "You" for the viewer. */
export function seatLabel(seat: number, name: string | undefined, you: number | null): string {
  if (you === seat) return 'You';
  return name ? `Seat ${seat + 1} · ${name}` : `Seat ${seat + 1}`;
}
