import { describe, expect, it } from 'vitest';
import type { CardView, Decision, Option, View } from '../../protocol';
import { divisionMarks, divisionRowText, isDivision } from './division';

const card = (id: number, name: string, extra: Partial<CardView> = {}) => ({ id, name, ...extra }) as CardView;
const opt = (label: string, index = 0) => ({ index, kind: 'division', label, player: 0 }) as Option;
const ask = (labels: string[], source = 1) =>
  ({ seq: 1, player: 0, kind: 'choose', prompt: '', min: 1, max: 1, source, options: labels.map(opt) }) as unknown as Decision;
const board = (mine: CardView[], theirs: CardView[]) =>
  ({
    viewer: 0, stack: [],
    players: [
      { seat: 0, battlefield: mine, graveyard: [], exile: [], hand: [] },
      { seat: 1, battlefield: theirs, graveyard: [], exile: [], hand: [] },
    ],
  }) as unknown as View;

describe('combat damage division', () => {
  const attacker = card(1, 'Grizzly Bears', { attacking: true, blocked_by: [8, 9] });

  it('letters same-named blockers in blocked_by order, in the row and on the board', () => {
    const v = board([attacker], [card(9, 'Grizzly Bears'), card(8, 'Grizzly Bears')]);
    const d = ask(['0 to Grizzly Bears,2 to Grizzly Bears']);
    expect(divisionRowText(d, v, d.options[0])).toBe('0 to Grizzly Bears\u00a0A, 2 to Grizzly Bears\u00a0B');
    expect([...divisionMarks(d, v)]).toEqual([[8, 'A'], [9, 'B']]);
  });

  it('leaves differently named blockers unlettered', () => {
    const v = board([{ ...attacker, blocked_by: [8, 9] }], [card(8, 'Grizzly Bears'), card(9, 'Runeclaw Bear')]);
    const d = ask(['1 to Grizzly Bears,1 to Runeclaw Bear']);
    expect(divisionRowText(d, v, d.options[0])).toBe('1 to Grizzly Bears, 1 to Runeclaw Bear');
    expect(divisionMarks(d, v).size).toBe(0);
  });

  it('keeps a comma inside a card name', () => {
    const v = board([attacker], [card(8, 'Ayara, First of Locthwain'), card(9, 'Grizzly Bears')]);
    const d = ask(['2 to Ayara, First of Locthwain,0 to Grizzly Bears']);
    expect(divisionRowText(d, v, d.options[0])).toBe('2 to Ayara, First of Locthwain, 0 to Grizzly Bears');
  });

  it('letters the attackers a banded blocker blocks', () => {
    const blocker = card(5, 'Benalish Hero');
    const v = board([blocker], [card(1, 'Grizzly Bears', { attacking: true, blocked_by: [5] }), card(2, 'Grizzly Bears', { attacking: true, blocked_by: [5] })]);
    const d = ask(['1 to Grizzly Bears,0 to Grizzly Bears'], 5);
    expect(divisionRowText(d, v, d.options[0])).toBe('1 to Grizzly Bears\u00a0A, 0 to Grizzly Bears\u00a0B');
  });

  it('skips a blocker no longer on the battlefield, as the engine does', () => {
    const v = board([attacker], [card(9, 'Grizzly Bears')]);
    v.players[1].graveyard = [card(8, 'Grizzly Bears')];
    const d = ask(['2 to Grizzly Bears']);
    expect(divisionMarks(d, v).size).toBe(0);
  });

  it('falls back to the spaced label when the board does not show the recipients', () => {
    const v = board([], []);
    const d = ask(['0 to Grizzly Bears,2 to Grizzly Bears']);
    expect(divisionRowText(d, v, d.options[0])).toBe('0 to Grizzly Bears, 2 to Grizzly Bears');
  });

  it('recognises a split by its words on every wire and nothing else', () => {
    expect(isDivision(ask(['0 to A,2 to B', '1 to A,1 to B']))).toBe(true);
    expect(isDivision(ask(['Yes', 'No']))).toBe(false);
  });
});
