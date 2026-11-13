/**
 * commanderdraft.ts holds the Commander-draft screens' DISPLAY helpers: the
 * colour identity a pool card carries on the wire (DraftCard.identity), the
 * cards a commander slot may offer (DraftCard.commander / .partner), and
 * whether a card sits inside the chosen commanders' identity.
 *
 * Every fact comes from the server's DraftCard; nothing here decides
 * legality. The deckbuilder uses these to grey out and refuse cards the
 * server would reject, and the server's DeckCheck stays the only verdict
 * (it also tells plain partners from the draft-partner legends, which the
 * wire's one `partner` flag does not).
 */

import type { DraftCard } from '../protocol';
import { COLOURS } from './draftcards';
import type { ManaColour } from './mana';

/** identityColours reads a "WUBRG"-subset string into colour letters, in WUBRG order. */
export function identityColours(identity: string | undefined): ManaColour[] {
  const id = identity ?? '';
  return COLOURS.filter((c) => id.includes(c));
}

/** unionIdentity is the combined colour identity of some cards (CR 903.4), as a WUBRG-ordered string. */
export function unionIdentity(cards: readonly DraftCard[]): string {
  return COLOURS.filter((c) => cards.some((x) => (x.identity ?? '').includes(c))).join('');
}

/** isBasicLand reports a basic land by its type line (basics are free and outside the identity check). */
export const isBasicLand = (c: DraftCard): boolean => /\bBasic\b/.test((c.type_line ?? '').split('//')[0]) && /\bLand\b/.test(c.type_line ?? '');

/** withinIdentity reports whether every colour of the card's identity is in `identity`. */
export function withinIdentity(c: DraftCard, identity: string): boolean {
  return identityColours(c.identity).every((col) => identity.includes(col));
}

/** canLead reports a card that can fill the first commander slot: a commander alone, or half of a pair. */
export const canLead = (c: DraftCard): boolean => c.commander === true || c.partner === true;

/** commanderChoices is the first slot's offer: every pool card that can be a commander alone or half of a pair, in pool order. */
export function commanderChoices(pool: readonly DraftCard[]): DraftCard[] {
  return pool.filter(canLead);
}

/**
 * partnerChoices is the second slot's offer once the first commander can
 * partner: every other partner-capable pool card. The wire's one flag covers
 * both pair rules (plain partner, and two partnerless legends of at most one
 * colour each), so a mixed pair can still be offered here; the server's
 * check names it.
 */
export function partnerChoices(pool: readonly DraftCard[], first: DraftCard | null): DraftCard[] {
  if (!first || first.partner !== true) return [];
  return pool.filter((c) => c.id !== first.id && c.partner === true);
}

/** PARTNER_RULE is the slot hint: when two commanders may share the command zone. */
export const PARTNER_RULE =
  'Two commanders are allowed when both have partner, or when both are legendary creatures without partner and each is at most one colour.';
