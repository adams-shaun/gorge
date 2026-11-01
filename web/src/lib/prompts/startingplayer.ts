import type { Decision, Option } from '../../protocol';

/**
 * startingPlayerLabel is a two-seat starting_player row's words: the option
 * naming the asking seat itself is "Play first", the other "Draw first".
 * The option's `player` is the server's;
 * the answer posted is still that option's own index. Any other decision —
 * and a starting_player ask among three or more seats, where seat names
 * stay the clearer words — gets null and keeps the server's label.
 */
export function startingPlayerLabel(d: Decision, o: Option): string | null {
  if (d.kind !== 'starting_player' || d.options.length !== 2) return null;
  if (o.player === undefined || o.player === null) return null;
  return o.player === d.player ? 'Play first' : 'Draw first';
}
