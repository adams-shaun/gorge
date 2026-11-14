import type { EventPodGame, EventSeries, EventState } from '../protocol';

/**
 * series.ts renders an EventSeries row's facts for the hub. Every value is
 * the server's: the score, the per-game winners (`games`, -1 = drawn), the
 * live or next game number and the between-games state. Nothing here
 * decides a result.
 */

/** GameDot is one played game of a series from one side: won, lost or drawn. */
export type GameDot = 'w' | 'l' | 'd';

/** gameDots reads the series' per-game winners from `side`'s point of view. */
export function gameDots(s: EventSeries, side: number): GameDot[] {
  return (s.games ?? []).map((w) => (w < 0 ? 'd' : w === side ? 'w' : 'l'));
}

/** seriesScore is the row's score ("2 – 1"), or "vs" before the series starts. */
export function seriesScore(s: EventSeries): string {
  if (s.state === 'pending') return 'vs';
  return `${s.score?.[0] ?? 0} – ${s.score?.[1] ?? 0}`;
}

/**
 * seriesStatus is the small line under the score: pending, live (with the
 * game number for a best of 3, and the attempt when a game was retried),
 * between games, or the final outcome.
 */
export function seriesStatus(s: EventSeries): string {
  switch (s.state) {
    case 'live': {
      const parts = ['live'];
      if (s.best_of > 1 && s.game) parts.push(`game ${s.game}`);
      if (s.attempt && s.attempt > 1) parts.push(`attempt ${s.attempt}`);
      return parts.join(' · ');
    }
    case 'between':
      return 'between games';
    case 'ended':
      return s.winner === null || s.winner === undefined ? 'draw' : 'final';
    default:
      return 'pending';
  }
}

/** podStatus is a Commander-draft pod game's state line: pending, live, or how it ended. */
export function podStatus(g: EventPodGame): string {
  switch (g.state) {
    case 'live':
      return 'live';
    case 'ended':
      return g.winner === null || g.winner === undefined ? 'draw' : 'final';
    default:
      return 'pending';
  }
}

/** inPlay reports a series whose games are under way: live, or between two of its games. */
export const inPlay = (s: EventSeries): boolean => s.state === 'live' || s.state === 'between';

/** opponentOf is the other seat of a series the viewer plays in. */
export const opponentOf = (s: EventSeries, you: number): number => (s.a === you ? s.b : s.a);

/**
 * previousResult is how the viewer's last game of the series went — the
 * game before `s.game` — read from the server's per-game winners, or null
 * before any game was played.
 */
export function previousResult(s: EventSeries, you: number): 'won' | 'lost' | 'drawn' | null {
  const g = (s.game ?? 0) - 2;
  const w = g >= 0 ? s.games?.[g] : undefined;
  if (w === undefined) return null;
  return w < 0 ? 'drawn' : w === you ? 'won' : 'lost';
}

/**
 * viewerSeries is the viewer's series in the current round, optionally the
 * one played on `table` (the table page's series), or null for a spectator
 * or a viewer with no series this round.
 */
export function viewerSeries(ev: EventState, table?: string): EventSeries | null {
  const you = ev.you;
  if (you === null || you === undefined) return null;
  return (
    (ev.series ?? []).find(
      (s) => s.round === ev.round && (s.a === you || s.b === you) && (table === undefined || !s.table || s.table === table),
    ) ?? null
  );
}

/** waitingOn reports whether the server still waits on `seat`'s deck between games. */
export const waitingOn = (s: EventSeries, seat: number): boolean => s.state === 'between' && (s.waiting ?? []).includes(seat);
