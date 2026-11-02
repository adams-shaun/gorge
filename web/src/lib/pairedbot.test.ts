import { describe, expect, it } from 'vitest';
import type { Decision } from '../protocol';
import { adviceRows, differedNote, loadDifferedNotes, loadPairedBotOn, loadShowRules, pairedRowLabel, saveDifferedNotes, saveShowRules, savePairedBotOn, shownRows, winChance } from './pairedbot';

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

describe('the bot\'s rules setting', () => {
  it('is on until the player turns it off, and remembers', () => {
    const s = memoryStorage();
    expect(loadShowRules(s)).toBe(true);
    expect(loadShowRules(null)).toBe(true);
    saveShowRules(false, s);
    expect(loadShowRules(s)).toBe(false);
    saveShowRules(true, s);
    expect(loadShowRules(s)).toBe(true);
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
    expect(pairedRowLabel(d, row({ choices: [3, 0] }))).toBe('Pass priority; Bear');
  });
});

describe('pairedRowLabel words', () => {
  it('names declining by the kind of decision', () => {
    const attackers = { kind: 'attackers', options: [] } as unknown as Decision;
    const blockers = { kind: 'blockers', options: [] } as unknown as Decision;
    expect(pairedRowLabel(attackers, { choices: null })).toBe('No attack');
    expect(pairedRowLabel(blockers, { choices: [] })).toBe('No blocks');
  });
  it('never shows a payment id', () => {
    const d = { options: [], payment_actions: [] } as unknown as Decision;
    expect(pairedRowLabel(d, { choices: null, payment: 'f5c391b4' })).toBe('A planned cast');
  });
});

describe('pairedRowLabel copies', () => {
  it('names two copies of a land in hand alike, and two permanents apart', () => {
    const d = {
      options: [
        { index: 0, label: 'Play Mountain', kind: 'play_land' },
        { index: 1, label: 'Play Mountain', kind: 'play_land' },
        { index: 2, label: 'Bear', kind: 'target' },
        { index: 3, label: 'Bear', kind: 'target' },
      ],
    } as unknown as Decision;
    expect(pairedRowLabel(d, { choices: [1] })).toBe('Play Mountain');
    expect(pairedRowLabel(d, { choices: [3] })).toBe('Bear (3)');
  });
});

describe('winChance', () => {
  it('reads a value as a percentage', () => {
    expect(winChance({ visits: 3, q: 0.3 })).toBe(65);
    expect(winChance({ visits: 3, q: -1 })).toBe(0);
    expect(winChance({ visits: 3, q: 1 })).toBe(100);
  });
  it('has no estimate for a row search never visited', () => {
    expect(winChance({ visits: 0, q: 0 })).toBeNull();
  });
});

describe('adviceRows', () => {
  const d = {
    kind: 'priority',
    options: [
      { index: 0, label: 'Pass priority', kind: 'pass' },
      { index: 1, label: 'Cast Goblin Guide', kind: 'cast' },
    ],
    payment_actions: [
      { id: 'guide-a', label: 'Cast Goblin Guide', base_option_index: 1, cast: {}, plans: [] },
      { id: 'guide-b', label: 'Cast Goblin Guide', cast: {}, plans: [] },
    ],
  } as unknown as Decision;
  const row = (over: object) => ({ choices: null, visits: 0, prior: 0, q: 0, ...over });

  it('gathers copies of a card under one name, weighted by visits', () => {
    const got = adviceRows(d, {
      engine: 'azmcts',
      root_value: 0,
      rows: [
        row({ choices: [0], visits: 50, q: 0, bot: true }),
        row({ payment: 'guide-a', visits: 30, q: 0.2, chosen: true }),
        row({ payment: 'guide-b', visits: 30, q: 0.4 }),
      ],
    });
    expect(got).toEqual([
      { label: 'Cast Goblin Guide', visits: 60, chance: 65, share: 55, recommended: true, rules: false },
      { label: 'Pass priority', visits: 50, chance: 50, share: 45, recommended: false, rules: true },
    ]);
  });
});

describe('shownRows', () => {
  const row = (visits: number, recommended = false) => ({ label: String(visits), visits, chance: 50, share: 0, recommended, rules: false });
  it('keeps the recommendation and counts the rest', () => {
    const got = shownRows([row(9), row(8), row(7), row(1, true)], 3, 2);
    expect(got.rows.map((r) => r.visits)).toEqual([9, 8, 1]);
    expect(got.more).toBe(3);
  });
});

describe('differedNote', () => {
  const d = {
    kind: 'priority',
    options: [
      { index: 0, label: 'Activate Island for mana', kind: 'activate' },
      { index: 1, label: 'Pass priority', kind: 'pass' },
      { index: 2, label: 'Cast Opt', kind: 'cast' },
      { index: 3, label: 'Cast Ponder', kind: 'cast' },
    ],
    payment_actions: [
      { id: 'pay-shock', label: 'Cast Shock', cast: {}, plans: [] },
      { id: 'pay-opt', label: 'Cast Opt', base_option_index: 2, cast: {}, plans: [] },
    ],
  } as unknown as Decision;
  const search = {
    engine: 'azmcts',
    root_value: 0,
    rows: [
      { choices: null, payment: 'pay-shock', visits: 60, prior: 0, q: 0.3, chosen: true },
      { choices: [1], visits: 30, prior: 0, q: -0.1 },
      { choices: null, payment: 'pay-opt', visits: 10, prior: 0, q: 0 },
      { choices: [3], visits: 0, prior: 0, q: 0 },
    ],
  };
  const answer = (over: object) => ({ seq: 1, choices: [], ...over });

  it('says nothing when the player follows the recommendation', () => {
    expect(differedNote(d, search, answer({ action: 'pay-shock' }))).toBeNull();
  });
  it('gives both estimates', () => {
    expect(differedNote(d, search, answer({ choices: [1] }))).toBe('You differed: Pass priority, estimated 45%. Search preferred Cast Shock, estimated 65%.');
  });
  it('treats a planned cast and its legacy option as one cast', () => {
    expect(differedNote(d, search, answer({ choices: [2] }))).toContain('You differed: Cast Opt, estimated 50%');
    expect(differedNote(d, search, answer({ action: 'pay-opt' }))).toContain('You differed: Cast Opt, estimated 50%');
  });
  it('treats casting another copy of the recommended card as following it', () => {
    const twin = { ...d, payment_actions: [...(d.payment_actions ?? []), { id: 'pay-shock-2', label: 'Cast Shock', cast: {}, plans: [] }] } as unknown as Decision;
    expect(differedNote(twin, search, answer({ action: 'pay-shock-2' }))).toBeNull();
  });
  it('says so when search never weighed the answer', () => {
    expect(differedNote(d, search, answer({ choices: [3] }))).toBe('You differed: Cast Ponder, which search did not weigh. Search preferred Cast Shock, estimated 65%.');
  });
  it('does not count tapping a land as an answer', () => {
    expect(differedNote(d, search, answer({ choices: [0] }))).toBeNull();
  });
  it('compares a set of attackers in any order', () => {
    const a = { kind: 'attackers', options: [{ index: 0, label: 'Attack with A', kind: 'attacker' }, { index: 1, label: 'Attack with B', kind: 'attacker' }] } as unknown as Decision;
    const s = { engine: 'azmcts', root_value: 0, rows: [{ choices: [0, 1], visits: 5, prior: 0, q: 0, chosen: true }, { choices: null, visits: 5, prior: 0, q: 0 }] };
    expect(differedNote(a, s, answer({ choices: [1, 0] }))).toBeNull();
    expect(differedNote(a, s, answer({ choices: [] }))).toContain('You differed: No attack, estimated 50%');
  });
});

describe('differed notes in storage', () => {
  it('belong to one seat of one match', () => {
    const s = memoryStorage();
    saveDifferedNotes('t1', 1, 0, [{ turn: 3, text: 'x' }], s);
    expect(loadDifferedNotes('t1', 1, 0, s)).toEqual([{ turn: 3, text: 'x' }]);
    expect(loadDifferedNotes('t1', 1, 1, s)).toEqual([]);
    expect(loadDifferedNotes('t1', 2, 0, s)).toEqual([]);
    expect(loadDifferedNotes('t2', 1, 0, null)).toEqual([]);
  });
});
