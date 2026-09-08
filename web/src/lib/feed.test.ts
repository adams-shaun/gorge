import { describe, expect, it } from 'vitest';
import { isRoutineLine, isStepLine, lastNotableByTable, latestPerTable, notableLines, pushFeed, type FeedLine } from './feed';

const l = (table: string, seq: number, line = 'x'): FeedLine => ({ table, match: 1, seq, line });

describe('feed', () => {
  it('appends newest last, dedupes by table/match/seq, and caps', () => {
    let f: FeedLine[] = [];
    f = pushFeed(f, l('t1', 1, 'a'));
    f = pushFeed(f, l('t2', 1, 'b'));
    f = pushFeed(f, l('t1', 1, 'a again'));
    expect(f.map((x) => x.line)).toEqual(['a', 'b']);
    for (let i = 2; i < 500; i++) f = pushFeed(f, l('t1', i), 100);
    expect(f.length).toBe(100);
    expect(f[f.length - 1].seq).toBe(499);
  });
  it('drops empty lines', () => {
    expect(pushFeed([], l('t1', 1, ''))).toEqual([]);
  });
});

describe('isRoutineLine', () => {
  it('matches the two lines that say only "the engine reached a priority window"', () => {
    expect(isRoutineLine('mono-red-goblins is asked: priority')).toBe(true);
    expect(isRoutineLine('Ann has priority')).toBe(true);
  });
  it('keeps every line that reports something happening', () => {
    expect(isRoutineLine('eldrazi-stompy is asked: attackers')).toBe(false);
    expect(isRoutineLine('Ann is asked: mulligan')).toBe(false);
    expect(isRoutineLine('Ann casts Lightning Bolt')).toBe(false);
    expect(isRoutineLine('Ann answers priority: pass')).toBe(false);
    expect(isRoutineLine('')).toBe(false);
  });
});

describe('isStepLine', () => {
  it('recognises a phase/step boundary by the server literal "Step: ", and nothing else', () => {
    expect(isStepLine('Step: main-1')).toBe(true);
    expect(isStepLine('Step: combat')).toBe(true);
    expect(isStepLine('Ann casts Lightning Bolt')).toBe(false);
    expect(isStepLine('Step down the hall')).toBe(false); // not the literal
    expect(isStepLine('')).toBe(false);
  });
});

describe('latestPerTable', () => {
  const feed: FeedLine[] = [
    l('t1', 1, 'a has priority'),
    l('t2', 1, 'b has priority'),
    l('t1', 2, 'a has priority'),
    l('t2', 2, 'b is asked: attackers'),
    l('t1', 3, 'a has priority'),
  ];

  it('collapses a table\'s own repeat run across interleaved tables', () => {
    const now = latestPerTable(feed, ['t1', 't2']);
    expect(now[0]).toEqual({ table: 't1', line: 'a has priority', count: 3, routine: true });
    expect(now[1]).toEqual({ table: 't2', line: 'b is asked: attackers', count: 1, routine: false });
  });

  it('returns one row per table in the order given, silent tables included', () => {
    const now = latestPerTable(feed, ['t2', 't1', 't3']);
    expect(now.map((n) => n.table)).toEqual(['t2', 't1', 't3']);
    expect(now[2]).toEqual({ table: 't3', line: '', count: 0, routine: false });
  });

  it('counts a priority run across the players it names', () => {
    const now = latestPerTable(
      [
        l('t1', 1, 'a is asked: priority'),
        l('t1', 2, 'b has priority'),
        l('t1', 3, 'c is asked: priority'),
      ],
      ['t1'],
    );
    expect(now[0].count).toBe(3);
    expect(now[0].line).toBe('c is asked: priority');
  });

  it('restarts the count when the line changes', () => {
    const now = latestPerTable([...feed, l('t1', 4, 'a casts Shock')], ['t1']);
    expect(now[0].count).toBe(1);
    expect(now[0].line).toBe('a casts Shock');
  });

  it('skips a trailing step line when picking a table\'s current line (ui9 B4)', () => {
    const now = latestPerTable([l('t1', 1, 'a casts Shock'), l('t1', 2, 'Step: main-1')], ['t1']);
    expect(now[0].line).toBe('a casts Shock');
    expect(now[0].count).toBe(1);
  });

  it('reports an empty line when a table has only step lines', () => {
    const now = latestPerTable([l('t1', 1, 'Step: main-1'), l('t1', 2, 'Step: combat')], ['t1']);
    expect(now[0].line).toBe('');
  });
});

describe('lastNotableByTable', () => {
  it('keeps the newest non-routine line per table and skips silent ones', () => {
    const m = lastNotableByTable([
      l('t1', 1, 'a casts Shock'),
      l('t2', 1, 'b has priority'),
      l('t1', 2, 'a is asked: priority'),
      l('t1', 3, 'a is asked: attackers'),
    ]);
    expect(m.get('t1')).toBe('a is asked: attackers');
    expect(m.has('t2')).toBe(false);
    expect(lastNotableByTable([]).size).toBe(0);
  });
  it('does not report a step line as the last notable thing a table did (ui9 B4)', () => {
    const m = lastNotableByTable([l('t1', 1, 'Step: main-1'), l('t1', 2, 'a casts Shock')]);
    expect(m.get('t1')).toBe('a casts Shock');
  });
});

describe('notableLines', () => {
  const feed: FeedLine[] = [
    l('t1', 1, 'a has priority'),
    l('t2', 1, 'b is asked: attackers'),
    l('t1', 2, 'a is asked: priority'),
    l('t1', 3, 'a casts Shock'),
  ];

  it('drops routine lines and keeps chronological order', () => {
    expect(notableLines(feed).map((x) => x.line)).toEqual(['b is asked: attackers', 'a casts Shock']);
  });
  it('shows everything when asked', () => {
    expect(notableLines(feed, true).length).toBe(4);
  });
  it('drops step lines by default, and shows them when all is set (ui9 B4)', () => {
    const withSteps = [...feed, l('t1', 4, 'Step: combat')];
    expect(notableLines(withSteps).some((x) => isStepLine(x.line))).toBe(false);
    expect(notableLines(withSteps, true).some((x) => isStepLine(x.line))).toBe(true);
  });
  it('caps to the newest lines', () => {
    const many = Array.from({ length: 50 }, (_, i) => l('t1', i, `a casts ${i}`));
    const out = notableLines(many, false, 10);
    expect(out.length).toBe(10);
    expect(out[out.length - 1].line).toBe('a casts 49');
  });
});
