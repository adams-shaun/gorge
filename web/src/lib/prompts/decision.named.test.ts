import { describe, expect, it } from 'vitest';
import type { Decision, Option, View } from '../../protocol';
import { namedDecision } from './decision';

const option = (index: number, label: string, extra: Partial<Option> = {}): Option => ({ index, kind: 'choice', label, player: 0, ...extra }) as Option;
const decision = (options: Option[], extra: Partial<Decision> = {}): Decision => ({ seq: 1, player: 0, kind: 'choose', prompt: 'Choose', min: 1, max: 1, options, ...extra }) as Decision;
const zone: never[] = [];
const view = {
  stack: [{ id: 9, name: 'Izzet Charm' }],
  players: [{ seat: 0, hand: zone, battlefield: [{ id: 4, name: 'Jace, Vryn\'s Prodigy' }], graveyard: zone, exile: zone, command: zone }],
} as unknown as View;

describe('namedDecision', () => {
  it('names a mode after the decision source', () => {
    const d = namedDecision(decision([option(0, 'CARDNAME deals 2 damage to target creature.')], { source: 9 }), view);
    expect(d.options[0].label).toBe('Izzet Charm deals 2 damage to target creature.');
  });

  it("names a trigger after its own card, and NICKNAME after the name's first word", () => {
    const d = namedDecision(decision([option(0, 'CARDNAME taps; NICKNAME loots.', { obj: 4 })]), view);
    expect(d.options[0].label).toBe("Jace, Vryn's Prodigy taps; Jace loots.");
  });

  it('reads the name a trigger label leads with when the card has left the board', () => {
    const d = namedDecision(decision([option(0, 'City of Brass: Whenever CARDNAME becomes tapped, it deals 1 damage to you.', { obj: 77 })]), view);
    expect(d.options[0].label).toBe('City of Brass: Whenever City of Brass becomes tapped, it deals 1 damage to you.');
  });

  it('never shows the token when no name is known, and returns a clean decision untouched', () => {
    expect(namedDecision(decision([option(0, 'CARDNAME deals 2 damage.')]), null).options[0].label).toBe('This deals 2 damage.');
    const clean = decision([option(0, 'Draw two cards.')]);
    expect(namedDecision(clean, view)).toBe(clean);
  });
});
