import { createGame, type CreateGame } from './api';
import { withBase } from './basepath';

/** The formats the play-vs-bot entry point offers, in the order shown. */
export const VS_BOT_FORMATS = [
  { value: 'commander', label: 'Commander' },
  { value: 'constructed', label: 'Constructed' },
] as const;
export type VsBotFormat = (typeof VS_BOT_FORMATS)[number]['value'];

/** The mulligan allowances the play-vs-bot entry point offers: 0 (no pre-game round) through 6 (down to a one-card hand), the default. */
export const VS_BOT_MULLIGANS = [0, 1, 2, 3, 4, 5, 6] as const;
export type VsBotMulligans = (typeof VS_BOT_MULLIGANS)[number];

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
 */
export async function startPlayVsBot(
  format: VsBotFormat,
  humanDeck = '',
  botDeck = '',
  mulligans: VsBotMulligans = 6,
): Promise<string> {
  const g: CreateGame = await createGame({
    format,
    ...(humanDeck ? { human_deck: humanDeck } : {}),
    ...(botDeck ? { bot_deck: botDeck } : {}),
    mulligans,
  });
  return withBase(g.join);
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
 * recreate the matchup exactly, so an empty deck id would be a bug, not a
 * default.
 */
export async function startRematch(
  format: VsBotFormat,
  humanDeck: string,
  botDeck: string,
  botPolicy: string,
  mulligans: number,
): Promise<string> {
  const g: CreateGame = await createGame({
    format,
    ...(humanDeck ? { human_deck: humanDeck } : {}),
    ...(botDeck ? { bot_deck: botDeck } : {}),
    ...(botPolicy ? { bot_policy: botPolicy } : {}),
    mulligans,
  });
  return withBase(g.join);
}
