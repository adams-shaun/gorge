import { describe, expect, it } from 'vitest';
import captured from '../fixtures/dirac-priority.json';
import type { Decision, View } from '../protocol';
import { decide } from './autopilot';
import { castableActions } from './announcepay';
import { deckLabel } from './playvsbot';
import { PRESETS } from './playsettings';
import { SeatPanelState } from './seatpanel.svelte';

// A /view and /pending a dirac table served at a priority window with a castable spell.
const decision = captured.pending as unknown as Decision;
const view = captured.view as unknown as View;

describe('a dirac table', () => {
  it('offers its plan-paid cast on a hand card', () => {
    const [action] = castableActions(decision, true, true);
    expect(action.plans[0].activations).toEqual([]);
    expect(view.players[0].hand?.some((c) => c.id === action.cast.object)).toBe(true);
  });

  it('stops auto-pass on the seat\'s own play', () => {
    const got = decide({ decision, view, seat: 0, settings: PRESETS.casual });
    expect(got.act).toBe('stop');
  });

  it('has no off for auto mana', () => {
    const p = new SeatPanelState('t1', 1, { seat: 0, name: 'p' } as never, null);
    p.setAutoManaAvailable(true);
    p.setNoManualMana(true);
    expect(p.autoPayOnly).toBe(true);
    p.setAutoPayMana(false);
    expect(p.autoPayMana).toBe(true);
  });

  it('reads a refused deck\'s reason in the picker', () => {
    const refused = 'dirac: card "Underground Sea" is not in the pool';
    expect(deckLabel({ id: 'd', name: 'Dimir', refused } as never)).toContain(refused);
  });
});
