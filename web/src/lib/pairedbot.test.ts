import { describe, expect, it } from 'vitest';
import type { Decision } from '../protocol';
import { loadPairedBotOn, pairedRowLabel, savePairedBotOn } from './pairedbot';

function memoryStorage(): Storage {
  const m = new Map<string, string>();
  return {
    length: 0,
    clear: () => m.clear(),
    key: () => null,
    getItem: (k) => m.get(k) ?? null,
    setItem: (k, v) => void m.set(k, v),
    removeItem: (k) => void m.delete(k),
  };
}

describe('paired bot preference', () => {
  it('is off until the player turns it on, and remembers', () => {
    const s = memoryStorage();
    expect(loadPairedBotOn(s)).toBe(false);
    expect(loadPairedBotOn(null)).toBe(false);
    savePairedBotOn(true, s);
    expect(loadPairedBotOn(s)).toBe(true);
    savePairedBotOn(false, s);
    expect(loadPairedBotOn(s)).toBe(false);
  });
});

describe('pairedRowLabel', () => {
  const d = {
    options: [
      { index: 0, label: 'Pass priority', kind: 'pass' },
      { index: 1, label: 'Shock', kind: 'target' },
      { index: 2, label: 'Shock', kind: 'target' },
      { index: 3, label: 'Bear', kind: 'target' },
    ],
    payment_actions: [{ id: 'pay-1', label: 'Cast Shock', cast: {}, plans: [] }],
  } as unknown as Decision;
  const row = (over: object) => ({ choices: [], visits: 1, prior: 0, q: 0, ...over });

  it('names a planned cast by its payment action', () => {
    expect(pairedRowLabel(d, row({ payment: 'pay-1' }))).toBe('Cast Shock');
  });
  it('names an option, and tells copies apart', () => {
    expect(pairedRowLabel(d, row({ choices: [0] }))).toBe('Pass priority');
    expect(pairedRowLabel(d, row({ choices: [2] }))).toBe('Shock (2)');
    expect(pairedRowLabel(d, row({ choices: [0, 3] }))).toBe('Pass priority; Bear');
  });
});
