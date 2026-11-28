import { createGame, type CreateGame, type DeckInfo } from './api';
import { withBase } from './basepath';
import type { SeatInfo } from '../protocol';

/** deckLabel is a deck as the pickers list it; a refused deck reads why. */
export function deckLabel(d: DeckInfo): string {
  if (d.refused) return `${d.name} — ${d.refused}`;
  const details = [d.archetype, d.commander ? `Commander: ${d.commander}` : ''].filter(Boolean);
  return details.length === 0 ? d.name : `${d.name} — ${details.join(' · ')}`;
}

/** The formats the play-vs-bot entry point offers, in the order shown. */
export const VS_BOT_FORMATS = [
  { value: 'commander', label: 'Commander' },
  { value: 'constructed', label: 'Constructed' },
] as const;
export type VsBotFormat = (typeof VS_BOT_FORMATS)[number]['value'];

/** The mulligan allowances the play-vs-bot entry point offers: 0 (no pre-game round) through 6 (down to a one-card hand), the default. */
export const VS_BOT_MULLIGANS = [0, 1, 2, 3, 4, 5, 6] as const;
export type VsBotMulligans = (typeof VS_BOT_MULLIGANS)[number];

/** The seat counts the play-vs-bot entry point offers for Commander: 2 is a duel, 3 and 4 are pods (one human, bots in the rest). */
export const VS_BOT_SEATS = [2, 3, 4] as const;
export type VsBotSeats = (typeof VS_BOT_SEATS)[number];

/** The largest pasted decklist the client sends, in UTF-8 bytes; the server's own bound for one seat's text. */
export const MAX_DECK_TEXT_BYTES = 65536;

/** deckTextTooLong reports a pasted list over MAX_DECK_TEXT_BYTES. It is the only check the client makes: legality is the server's. */
export function deckTextTooLong(s: string): boolean {
  return new TextEncoder().encode(s).length > MAX_DECK_TEXT_BYTES;
}

/**
 * startPlayVsBot asks the server to seat the player against a bot in the
 * given format and returns the base-relative join path to follow. It is the
 * pure half of the landing-page action (the side-effect — actually
 * navigating — stays in the component), so it is testable without a DOM: a
 * success returns the join path, a server rejection throws the ApiError the
 * calling component renders.
 *
 * mulligans is the per-game London allowance. It is always sent: the
 * entry point's default of 6 (mulligan down to one card) is a client choice,
 * independent of the server's -mulligans flag for its own tables.
 *
 * botPolicy names the opponent's hosted policy; empty keeps the server's
 * default (-bot-policy), exactly as an omitted deck id keeps random.
 *
 * pairedBot chooses the table's paired bot; empty keeps the server's default.
 *
 * seats above 2 asks for a pod with one human seat: the deck ids are not
 * sent (the server deals pod decks from its Commander pool and refuses
 * bot_deck there), and seats/humans follow the other keys. At 2 the body is
 * the duel body, with no seats key at all.
 *
 * deckText is the player's own pasted decklist. When it holds more than
 * whitespace it is sent trimmed as seat 0's deck, and humanDeck is not sent.
 */
export async function startPlayVsBot(
  format: VsBotFormat,
  humanDeck = '',
  botDeck = '',
  mulligans: VsBotMulligans = 6,
  botPolicy = '',
  pairedBot = '',
  seats: VsBotSeats = 2,
  deckText = '',
): Promise<string> {
  const pod = seats > 2;
  const text = deckText.trim();
  const g: CreateGame = await createGame({
    format,
    ...(!pod && !text && humanDeck ? { human_deck: humanDeck } : {}),
    ...(!pod && botDeck ? { bot_deck: botDeck } : {}),
    ...(botPolicy ? { bot_policy: botPolicy } : {}),
    mulligans,
    ...(pairedBot ? { paired_bot: pairedBot } : {}),
    ...(pod ? { seats, humans: 1 } : {}),
    ...(text ? { decks: [{ seat: 0, text }] } : {}),
  });
  return withBase(g.join);
}

/** Return the exact deck ids in human/bot roles, independent of seat order. */
export function rematchDecks(seats: SeatInfo[], humanSeat: number): { humanDeck: string; botDeck: string } {
  const botSeat = seats.findIndex((_, index) => index !== humanSeat);
  const humanDeck = seats[humanSeat]?.deck_id;
  const botDeck = seats[botSeat]?.deck_id;
  if (!humanDeck || !botDeck) throw new Error('Cannot restart: exact deck IDs are unavailable for this game.');
  return { humanDeck, botDeck };
}

/**
 * startRematch asks the server for a NEW vs-bot game with the SAME matchup as
 * the one the player is in — same format, same two deck ids, same bot_policy
 * and same London mulligan allowance — and returns the base-relative join
 * path for it. The server mints a fresh seed on every create, so the rematch
 * is the same decks with fresh luck. It is the pure half of the seated
 * Restart control (Table.svelte navigates), so it is testable without a DOM
 * exactly like startPlayVsBot.
 *
 * Every value here is the CURRENT game's resolved public config: format and
 * bot_policy and mulligans come off the table's TableInfo, and the deck ids
 * are the exact pool ids a seat plays (SeatInfo.deck_id, host/match.go's
 * deckNames) — NOT the display names, which may differ from the id. All four
 * are sent: unlike the entry point's "leave it random" path, a rematch must
 * recreate the matchup exactly. Missing deck ids throw before any POST, so
 * Table's catch renders an error instead of silently choosing random decks.
 */
export async function startRematch(
  format: VsBotFormat,
  humanDeck: string,
  botDeck: string,
  botPolicy: string,
  mulligans: number,
  pairedBot = '',
): Promise<string> {
  // Unlike the lobby's random-deck flow, a rematch must not let the server
  // substitute random decks when an older snapshot lacks either exact id.
  if (!humanDeck || !botDeck) throw new Error('Cannot restart: exact deck IDs are unavailable for this game.');
  const g: CreateGame = await createGame({
    format,
    human_deck: humanDeck,
    bot_deck: botDeck,
    ...(botPolicy ? { bot_policy: botPolicy } : {}),
    mulligans,
    ...(pairedBot ? { paired_bot: pairedBot } : {}),
  });
  return withBase(g.join);
}
