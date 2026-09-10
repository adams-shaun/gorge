import { createGame, type CreateGame } from './api';
import { withBase } from './basepath';

/** The formats the play-vs-bot entry point offers, in the order shown. */
export const VS_BOT_FORMATS = [
  { value: 'commander', label: 'Commander' },
  { value: 'constructed', label: 'Constructed' },
] as const;
export type VsBotFormat = (typeof VS_BOT_FORMATS)[number]['value'];

/**
 * startPlayVsBot asks the server to seat the player against a bot in the
 * given format and returns the base-relative join path to follow. It is the
 * pure half of the landing-page action (the side-effect — actually
 * navigating — stays in the component), so it is testable without a DOM: a
 * success returns the join path, a server rejection throws the ApiError the
 * calling component renders.
 */
export async function startPlayVsBot(format: VsBotFormat, humanDeck = '', botDeck = ''): Promise<string> {
  const g: CreateGame = await createGame({
    format,
    ...(humanDeck ? { human_deck: humanDeck } : {}),
    ...(botDeck ? { bot_deck: botDeck } : {}),
  });
  return withBase(g.join);
}
