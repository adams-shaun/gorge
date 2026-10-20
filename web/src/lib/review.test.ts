import { describe, expect, it } from 'vitest';
import type { ReviewDecision } from '../protocol';
import { initialDvr, type DvrState } from './dvr';
import { resolveStart, stepTarget } from './replay';
import {
  bySeat,
  decisionStops,
  duplicateLabels,
  isPassOnly,
  entryAt,
  entryIndexAt,
  hasExplain,
  isFallback,
  isOverride,
  isSearched,
  labelOf,
  nextEntry,
  note,
  signed,
  sourceBadge,
  timelineMarks,
  type ExplainNote,
} from './review';

const entry = (intent: number, ask: number, over: Partial<ReviewDecision> = {}): ReviewDecision => ({
  intent,
  ask_seq: ask,
  made_seq: ask + 1,
  turn: 1,
  round: 1,
  seat: intent % 2,
  kind: 'priority',
  min: 1,
  max: 1,
  options: [
    { index: 0, label: 'Pass priority', kind: 'pass' },
    { index: 1, label: 'Cast Shock', kind: 'cast' },
    { index: 2, label: 'Concede', kind: 'concede' },
  ],
  options_total: 3,
  chosen: [0],
  made: 'priority:[0]',
  explain: null,
  ...over,
});

const n = (x: Partial<ExplainNote>): ExplainNote => ({ seat: 0, source: 'policy', ...x });

describe('pass-only decisions', () => {
  const passOnly = (intent: number, ask: number) =>
    entry(intent, ask, {
      options: [
        { index: 0, label: 'Pass priority', kind: 'pass' },
        { index: 1, label: 'Concede', kind: 'concede' },
      ],
      options_total: 2,
    });

  it('a window offering only pass and concede is pass-only; a real option or a capped list is not', () => {
    expect(isPassOnly(passOnly(0, 5))).toBe(true);
    expect(isPassOnly(entry(0, 5))).toBe(false);
    expect(isPassOnly({ ...passOnly(0, 5), options_total: 300 })).toBe(false);
    expect(isPassOnly({ ...passOnly(0, 5), options: [] })).toBe(false);
  });

  it('decision stops leave the pass-only asks out, and stepping uses them', () => {
    const entries = [entry(0, 5), passOnly(1, 15), passOnly(2, 25), entry(3, 35)];
    const stops = decisionStops(entries);
    expect(stops).toEqual([5, 35]);
    const dvr: DvrState = { ...initialDvr, match: 't/1', head: 39, cursor: 5, live: false, events: [], turnStarts: [] };
    expect(stepTarget(dvr, 'decision', 1, stops)).toBe(35);
    expect(stepTarget({ ...dvr, cursor: 35 }, 'decision', -1, stops)).toBe(5);
    expect(stepTarget({ ...dvr, cursor: 35 }, 'decision', 1, stops)).toBe(39);
  });
});

/**
 * Six decisions asked at 5, 15, 25, 35, 45, 55:
 *   0 no note; 1 searched, overrode the bot; 2 heuristic, agreed;
 *   3 root fallback; 4 searched, agreed; 5 refusal fallback that also differs from bot.
 */
const index: ReviewDecision[] = [
  entry(0, 5),
  entry(1, 15, { chosen: [1], explain: n({ seat: 1, bot: [0], search: { engine: 'azmcts', root_value: 0.3, rows: [] } }) }),
  entry(2, 25, { explain: n({ source: 'heuristic', bot: [0] }) }),
  entry(3, 35, { explain: n({ seat: 1, source: 'fallback_root', reason: 'no feed' }) }),
  entry(4, 45, { explain: n({ bot: [0], search: { engine: 'azmcts', root_value: -0.1, rows: [] } }) }),
  entry(5, 55, { chosen: [2], explain: n({ seat: 1, source: 'refusal_fallback', rung: 2, bot: [1] }) }),
];

describe('review cursor mapping', () => {
  it('entryAt is the entry with ask_seq <= cursor < next ask_seq, null before the first', () => {
    expect(entryAt(index, 0)).toBeNull();
    expect(entryAt(index, 4)).toBeNull();
    expect(entryAt(index, 5)?.intent).toBe(0);
    expect(entryAt(index, 14)?.intent).toBe(0);
    expect(entryAt(index, 15)?.intent).toBe(1);
    expect(entryAt(index, 54)?.intent).toBe(4);
    expect(entryAt(index, 999)?.intent).toBe(5);
    expect(entryAt([], 10)).toBeNull();
    expect(entryIndexAt(index, 35)).toBe(3);
  });

  it('?intent=N lands the cursor on entry N', () => {
    const events = index.flatMap((e) => [
      { event: { seq: e.ask_seq, kind: 'decision_ask', player: e.seat }, line: '' },
      { event: { seq: e.made_seq, kind: 'decision_made', player: e.seat }, line: '' },
    ]);
    const dvr: DvrState = { ...initialDvr, match: 't/1', head: 60, cursor: 60, live: false, events, turnStarts: [] };
    for (const e of index) {
      const seq = resolveStart({ intent: e.intent }, dvr, 2);
      expect(seq).toBe(e.ask_seq);
      expect(entryAt(index, seq!)?.intent).toBe(e.intent);
    }
  });
});

describe('review predicates', () => {
  it('derives override, fallback and searched from the note', () => {
    expect(index.map(isOverride)).toEqual([false, true, false, false, false, true]);
    expect(index.map(isFallback)).toEqual([false, false, false, true, false, true]);
    expect(index.map(isSearched)).toEqual([false, true, false, false, true, false]);
    expect(note(index[0])).toBeNull();
    expect(hasExplain(index)).toBe(true);
    expect(hasExplain([entry(0, 1), entry(1, 2)])).toBe(false);
  });

  it('compares a set answer as a set and an ordered answer as a sequence', () => {
    const atk = entry(0, 1, { kind: 'attackers', chosen: [2, 0], explain: n({ bot: [0, 2] }) });
    expect(isOverride(atk)).toBe(false);
    const order = entry(0, 1, { kind: 'trigger_order', chosen: [2, 0], explain: n({ bot: [0, 2] }) });
    expect(isOverride(order)).toBe(true);
    expect(isOverride(entry(0, 1, { chosen: [], explain: n({ bot: [] }) }))).toBe(false);
  });

  it('nextEntry jumps forward and back by predicate', () => {
    expect(nextEntry(index, 0, 1, isOverride)?.intent).toBe(1);
    expect(nextEntry(index, 15, 1, isOverride)?.intent).toBe(5);
    expect(nextEntry(index, 55, 1, isOverride)).toBeNull();
    expect(nextEntry(index, 55, -1, isOverride)?.intent).toBe(1);
    // Between decisions, back returns to the one just passed if it matches.
    expect(nextEntry(index, 20, -1, isOverride)?.intent).toBe(1);
    expect(nextEntry(index, 15, -1, isOverride)).toBeNull();
    expect(nextEntry(index, 0, 1, isFallback)?.intent).toBe(3);
    expect(nextEntry(index, 35, 1, isFallback)?.intent).toBe(5);
    expect(nextEntry(index, 40, -1, isFallback)?.intent).toBe(3);
    expect(nextEntry(index, 15, 1, isSearched)?.intent).toBe(4);
    expect(nextEntry(index, 45, -1, isSearched)?.intent).toBe(1);
    expect(nextEntry(index, 5, 1, bySeat(1))?.intent).toBe(1);
    expect(nextEntry(index, 15, 1, bySeat(1))?.intent).toBe(3);
  });
});

describe('review labels', () => {
  it('labels choices from the listed options', () => {
    expect(labelOf(index[0], [1])).toBe('Cast Shock');
    expect(labelOf(index[0], [1, 2])).toBe('Cast Shock; Concede');
    expect(labelOf(index[0], [])).toBe('Nothing');
    expect(labelOf(index[0], [9])).toBe('option 9');
    const twin = entry(0, 1, { options: [{ index: 0, label: 'Cast Terminus' }, { index: 1, label: 'Cast Terminus' }, { index: 2, label: 'Pass priority' }] });
    expect([...duplicateLabels(twin)]).toEqual(['Cast Terminus']);
    expect(duplicateLabels(index[0]).size).toBe(0);
    expect(duplicateLabels(null).size).toBe(0);
  });

  it('names every source in plain words', () => {
    for (const s of ['policy', 'heuristic', 'forced', 'prior_skip', 'fallback_feed', 'fallback_root', 'refusal_retry', 'refusal_fallback']) {
      const b = sourceBadge(n({ source: s }));
      expect(b.text.length).toBeGreaterThan(0);
      expect(b.tip.length).toBeGreaterThan(10);
      expect(b.tone).toBe(s.startsWith('fallback_') || s.startsWith('refusal_') ? 'fallback' : s === 'policy' ? 'policy' : 'plain');
    }
    expect(sourceBadge(n({ source: 'refusal_fallback', rung: 2 })).tip).toContain('rung 2');
    expect(sourceBadge(n({ source: 'fallback_root', reason: 'no feed' })).tip).toContain('no feed');
  });

  it('marks overrides and fallbacks only, and nothing without explain', () => {
    const marks = timelineMarks(index, (s) => `Seat ${s + 1}`);
    expect(marks.map((m) => [m.seq, m.kind])).toEqual([[15, 'override'], [35, 'fallback'], [55, 'fallback']]);
    expect(marks[0].tip).toContain('Seat 2');
    expect(timelineMarks([entry(0, 1), entry(1, 2)], String)).toEqual([]);
  });

  it('prints values with their sign', () => {
    expect(signed(0.25)).toBe('+0.25');
    expect(signed(-0.4)).toBe('−0.40');
    expect(signed(0)).toBe('0.00');
    expect(signed(-0.001)).toBe('0.00');
  });
});
