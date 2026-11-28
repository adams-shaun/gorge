import { describe, expect, it } from 'vitest';
import { leaderboardNote } from './leaderboard';

describe('leaderboardNote', () => {
  it('names one pilot and one seat count with the even share', () => {
    expect(leaderboardNote({ pilots: ['lethal-pressure'], seats: [4], matches_counted: 812 })).toBe(
      '812 bot-only games, pilot lethal-pressure, 4-seat tables (even share 25.0%). This measures the bot, not the deck.',
    );
  });

  it('joins several pilots with ", "', () => {
    expect(leaderboardNote({ pilots: ['bot', 'lethal-pressure'], seats: [2], matches_counted: 3 })).toBe(
      '3 bot-only games, pilot bot, lethal-pressure, 2-seat tables (even share 50.0%). This measures the bot, not the deck.',
    );
  });

  it('drops the even-share clause for several seat counts', () => {
    expect(leaderboardNote({ pilots: ['bot'], seats: [2, 4], matches_counted: 10 })).toBe(
      '10 bot-only games, pilot bot, 2, 4-seat tables. This measures the bot, not the deck.',
    );
  });
});
