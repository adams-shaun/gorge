import { describe, expect, it } from 'vitest';
import { wheelFace } from './cardoptions';

// The radial wheel's button face must tell a card's abilities apart. Every
// "ability" option's label is "<card name>: <description>", so the old
// first-word face read "Jace," on all four of Jace, the Mind Sculptor's
// buttons (2026-09-24 report).
const jace = (cost: string, text: string) => ({ kind: 'ability', label: `Jace, the Mind Sculptor: ${text}`, cost });

describe('wheelFace', () => {
  it('shows a planeswalker ability as its signed loyalty cost', () => {
    expect(wheelFace(jace('AddCounter<2/LOYALTY>', 'Look at the top card of target player\'s library.'))).toBe('+2');
    expect(wheelFace(jace('AddCounter<0/LOYALTY>', 'Draw three cards, then put two cards back.'))).toBe('0');
    expect(wheelFace(jace('SubCounter<1/LOYALTY>', 'Return target creature to its owner\'s hand.'))).toBe('−1');
    expect(wheelFace(jace('SubCounter<12/LOYALTY>', 'Exile all cards from target player\'s library.'))).toBe('−12');
  });

  it('gives each ability of one card a distinct face', () => {
    const faces = [
      jace('AddCounter<2/LOYALTY>', 'Look at the top card.'),
      jace('AddCounter<0/LOYALTY>', 'Draw three cards.'),
      jace('SubCounter<1/LOYALTY>', 'Return target creature.'),
      jace('SubCounter<12/LOYALTY>', 'Exile all cards.'),
    ].map(wheelFace);
    expect(new Set(faces).size).toBe(4);
  });

  it('drops the card-name prefix and uses the first word of the ability text otherwise', () => {
    expect(wheelFace({ kind: 'ability', label: 'Sailor: Draw a card.', cost: '3 U' })).toBe('Draw');
    expect(wheelFace({ kind: 'ability', label: 'Sailor: Regenerate Sailor.' })).toBe('Regen…');
  });

  it('keeps a label without a card-name prefix', () => {
    expect(wheelFace({ kind: 'cast', label: 'Cast' })).toBe('Cast');
  });
});
