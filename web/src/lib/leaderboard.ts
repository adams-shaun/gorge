/**
 * The sentence under the landing page's leaderboard saying what it measures:
 * bot self-play under the named pilot(s) at the named seat count(s). With one
 * seat count it adds the even share (100 / seats); with several it does not.
 */
export function leaderboardNote(t: { pilots: string[]; seats: number[]; matches_counted: number }): string {
  const pilot = `pilot ${t.pilots.join(', ')}`;
  let tables = `${t.seats.join(', ')}-seat tables`;
  if (t.seats.length === 1) tables += ` (even share ${(100 / t.seats[0]).toFixed(1)}%)`;
  return `${t.matches_counted} bot-only games, ${pilot}, ${tables}. This measures the bot, not the deck.`;
}
