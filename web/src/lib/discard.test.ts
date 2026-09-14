import { describe, expect, it } from 'vitest';
import type { Decision } from '../protocol';
import { discardCard, discardName, isDiscardPick } from './discard';

/**
 * The discard-pick ask's pure half (fb-20260914T120705Z): the decision shape
 * the card-face row triggers on, and the label→name extraction every
 * synthesized face resolves its art and oracle text by. The four engine
 * shapes (effects/cardflow.go's RevealYouChoose and TgtChoose, rules/combat.go
 * cleanupStep, rules/cast.go's cost discard) span two decision kinds and two
 * label conventions; the fixtures here mirror each one's exact wire shape.
 */

const modesDiscard: Decision = {
  seq: 1, player: 0, kind: 'modes', min: 1, max: 1, prompt: 'Choose 1 card(s) to discard',
  options: [
    { index: 0, kind: 'discard', label: 'Discard Thoughtseize', obj: 21, player: 0 },
    { index: 1, kind: 'discard', label: 'Discard Mother of Runes', obj: 22, player: 0 },
  ],
};
const chooseCleanup: Decision = {
  ...modesDiscard, kind: 'choose', min: 2, max: 2, prompt: 'discard 2 card(s) down to the hand-size limit',
  options: [
    { index: 0, kind: 'discard', label: 'Discard Brazen Borrower', obj: 11, player: 0 },
    { index: 1, kind: 'discard', label: 'Discard Fabled Pass', obj: 12, player: 0 },
    { index: 2, kind: 'discard', label: 'Discard Gitaxian Probe', obj: 13, player: 0 },
  ],
};
const chooseCost: Decision = {
  ...modesDiscard, kind: 'choose', prompt: 'Discard a card to pay the cost of Late Game',
  options: [
    { index: 0, kind: 'discard', label: 'Late Game', obj: 31, player: 0 },
    { index: 1, kind: 'discard', label: "Lookout's Dispersal", obj: 32, player: 0 },
  ],
};
const charmModes: Decision = {
  seq: 2, player: 0, kind: 'modes', min: 1, max: 1, prompt: 'Choose a mode',
  options: [
    { index: 0, kind: 'mode', label: 'Deal 2 damage', player: 0 },
    { index: 1, kind: 'mode', label: 'Draw a card', player: 0 },
  ],
};

describe('discardName', () => {
  it("strips the engine's \"Discard \" prefix and tolerates the bare cast-cost label", () => {
    expect(discardName('Discard Thoughtseize')).toBe('Thoughtseize');
    expect(discardName('Late Game')).toBe('Late Game');
  });

  it('leaves a name that merely contains the word intact', () => {
    expect(discardName('Discarded Secrets')).toBe('Discarded Secrets');
  });
});

describe('isDiscardPick', () => {
  it('admits every discard-pick shape: both decision kinds and both label conventions', () => {
    expect(isDiscardPick(modesDiscard)).toBe(true);
    expect(isDiscardPick(chooseCleanup)).toBe(true);
    expect(isDiscardPick(chooseCost)).toBe(true);
  });

  it('rejects null, empty and mixed decisions', () => {
    expect(isDiscardPick(null)).toBe(false);
    expect(isDiscardPick({ ...modesDiscard, options: [] })).toBe(false);
    // A modal mode list (no Obj) is the generic text list, never a face row.
    expect(isDiscardPick(charmModes)).toBe(false);
    // One option without an object to show poisons the whole ask: the face row
    // would hide a pickable thing the text list can name.
    expect(
      isDiscardPick({ ...modesDiscard, options: [...modesDiscard.options, { index: 2, kind: 'discard', label: 'a card', player: 0 }] }),
    ).toBe(false);
  });
});

describe('discardCard', () => {
  it('synthesizes a face named by the stripped label, id from Obj', () => {
    const card = discardCard(modesDiscard, modesDiscard.options[0]);
    expect(card.id).toBe(21);
    expect(card.name).toBe('Thoughtseize');
    expect(card.printing.name).toBe('Thoughtseize');
  });

  it('carries the bare cast-cost name through unchanged', () => {
    const card = discardCard(chooseCost, chooseCost.options[1]);
    expect(card.name).toBe("Lookout's Dispersal");
  });
});
