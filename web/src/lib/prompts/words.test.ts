import { describe, expect, it } from 'vitest';
import { promptWords } from './words';

describe('promptWords', () => {
  it.each([
    ['Choose 2 card(s) to discard', 'Choose 2 cards to discard'],
    ['Choose 1 to 3 card(s) to discard', 'Choose 1 to 3 cards to discard'],
    ['You may reveal up to 2 card(s)', 'You may reveal up to 2 cards'],
    ['Choose 1 to 1 mode(s)', 'Choose 1 mode'],
    ['Choose 1 to 2 mode(s)', 'Choose 1 to 2 modes'],
    ['Draw 1 card(s)?', 'Draw 1 card?'],
    ['Draw up to 3 card(s)?', 'Draw up to 3 cards?'],
    ['Look at 1 additional card(s) each time you surveil', 'Look at 1 additional card each time you surveil'],
    ['Reveal the top 1 card(s) of your library?', 'Reveal the top 1 card of your library?'],
    ['Exile 1 or more card(s) from your graveyard', 'Exile 1 or more cards from your graveyard'],
    ['Pay {1} per age (1 age counter(s))', 'Pay {1} per age (1 age counter)'],
    ['Distribute +1/+1 counter(s): choose where to place 1', 'Distribute +1/+1 counters: choose where to place 1'],
    ['Move the selected card(s)?', 'Move the selected cards?'],
    ['turn 7 — discard 2 card(s) down to the hand-size limit', 'Discard 2 cards down to the hand-size limit'],
    ["turn 7 — divide Grizzly Bears's combat damage among its blockers", "Divide Grizzly Bears's combat damage among its blockers"],
  ])('%s', (raw, want) => {
    expect(promptWords(raw)).toBe(want);
  });

  it('leaves ordinary words alone', () => {
    for (const s of ['Turn 3 — your move', 'turn 5, main1 — You have priority', 'Keep this hand?', 'Fire // Ice', '10 to the face']) {
      expect(promptWords(s)).toBe(s);
    }
  });
});
