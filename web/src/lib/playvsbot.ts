import { createGame, type CreateGame } from './api';
import { withBase } from './basepath';

/** The formats the play-vs-bot entry point offers, in the order shown. */
export const VS_BOT_FORMATS = [
  { value: 'commander', label: 'Commander' },
  { value: 'constructed', label: 'Constructed' },
] as const;
export type VsBotFormat = (typeof VS_BOT_FORMATS)[number]['value'];

/** The mulligan allowances the play-vs-bot entry point offers, 0 (no pre-game round) through 7 (the opening hand); 1 is the server default. */
export const VS_BOT_MULLIGANS = [0, 1, 2, 3, 4, 5, 6, 7] as const;
export type VsBotMulligans = (typeof VS_BOT_MULLIGANS)[number];

/**
 * startPlayVsBot asks the server to seat the player against a bot in the
 * given format and returns the base-relative join path to follow. It is the
 * pure half of the landing-page action (the side-effect — actually
 * navigating — stays in the component), so it is testable without a DOM: a
 * success returns the join path, a server rejection throws the ApiError the
 * calling component renders.
 *
 * mulligans is the per-game London allowance; at the default 1 the key is
 * omitted from the JSON body entirely, so the omitted-means-server-default
 * contract is exercised end to end.
 */
export async function startPlayVsBot(
  format: VsBotFormat,
  humanDeck = '',
  botDeck = '',
  mulligans: VsBotMulligans = 1,
): Promise<string> {
  const g: CreateGame = await createGame({
    format,
    ...(humanDeck ? { human_deck: humanDeck } : {}),
    ...(botDeck ? { bot_deck: botDeck } : {}),
    ...(mulligans !== 1 ? { mulligans } : {}),
  });
  return withBase(g.join);
}
