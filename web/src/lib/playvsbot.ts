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
