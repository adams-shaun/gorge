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

/**
 * seatChoices is the seat counts the entry point may offer a server that
 * seats at most maxSeats at a table. A server that does not say (0, or a
 * value below a duel) keeps the full list, exactly as it did before
 * /api/capabilities reported max_seats.
 */
export function seatChoices(maxSeats: number): VsBotSeats[] {
  const cap = !(maxSeats >= 2) ? 4 : Math.min(4, Math.floor(maxSeats));
  return VS_BOT_SEATS.filter((n) => n <= cap);
}

/**
 * commanderSeats is the Commander entry's default: four seats (a pod) on a
 * server that seats four, else the largest table the server seats. It is
 * never a seat count seatChoices does not offer.
 */
export function commanderSeats(maxSeats: number): VsBotSeats {
  return seatChoices(maxSeats).at(-1) ?? 4;
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
 */
export async function startPlayVsBot(
  format: VsBotFormat,
  humanDeck = '',
  botDeck = '',
  mulligans: VsBotMulligans = 6,
  botPolicy = '',
  pairedBot = '',
  seats: VsBotSeats = 2,
): Promise<string> {
  const pod = seats > 2;
  const g: CreateGame = await createGame({
    format,
    ...(!pod && humanDeck ? { human_deck: humanDeck } : {}),
    ...(!pod && botDeck ? { bot_deck: botDeck } : {}),
    ...(botPolicy ? { bot_policy: botPolicy } : {}),
    mulligans,
    ...(pairedBot ? { paired_bot: pairedBot } : {}),
    ...(pod ? { seats, humans: 1 } : {}),
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
