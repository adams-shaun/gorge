import { describe, expect, it } from 'vitest';
import { FORMAT_ORDER, formatTitle, gridMetrics, normalizeFormat, sectionTables } from './lobby';

const t = (id: string, format?: string, state = 'live') => ({ id, info: { format, state } });

describe('sectionTables', () => {
  it('puts Commander first and constructed second', () => {
    const s = sectionTables([t('a', 'constructed'), t('b', 'commander'), t('c', 'constructed')]);
    expect(s.map((x) => x.format)).toEqual(['commander', 'constructed']);
    expect(s[0].tables.map((x) => x.id)).toEqual(['b']);
    expect(s[1].tables.map((x) => x.id)).toEqual(['a', 'c']);
    expect(s.map((x) => x.title)).toEqual(['Commander', 'Constructed']);
  });

  it('keeps the server order inside a section', () => {
    const s = sectionTables([t('t3', 'commander'), t('t1', 'commander'), t('t2', 'commander')]);
    expect(s[0].tables.map((x) => x.id)).toEqual(['t3', 't1', 't2']);
  });

  it('suppresses the heading when one format is running, and shows it when two are', () => {
    expect(sectionTables([t('a', 'constructed'), t('b', 'constructed')])[0].heading).toBe(false);
    expect(sectionTables([t('a', 'commander')])[0].heading).toBe(false);
    expect(sectionTables([t('a', 'commander'), t('b', 'constructed')]).map((x) => x.heading)).toEqual([true, true]);
  });

  it('counts live tables per section', () => {
    const s = sectionTables([
      t('a', 'commander', 'live'),
      t('b', 'commander', 'cooldown'),
      t('c', 'constructed', 'halted'),
    ]);
    expect(s[0].live).toBe(1);
    expect(s[0].tables.length).toBe(2);
    expect(s[1].live).toBe(0);
  });

  it('treats a missing or blank format as constructed, the wire zero value', () => {
    expect(normalizeFormat(undefined)).toBe('constructed');
    expect(normalizeFormat('')).toBe('constructed');
    expect(normalizeFormat('  Commander ')).toBe('commander');
    const s = sectionTables([t('a'), t('b', 'constructed')]);
    expect(s.length).toBe(1);
    expect(s[0].format).toBe('constructed');
    expect(s[0].heading).toBe(false);
  });

  it('renders an unknown format after the known ones, alphabetically', () => {
    const s = sectionTables([t('a', 'zzz'), t('b', 'constructed'), t('c', 'brawl'), t('d', 'commander')]);
    expect(s.map((x) => x.format)).toEqual(['commander', 'constructed', 'brawl', 'zzz']);
    expect(s[2].title).toBe('Brawl');
  });

  it('returns nothing for an empty server', () => {
    expect(sectionTables([])).toEqual([]);
  });

  it('names its two known formats', () => {
    expect(FORMAT_ORDER).toEqual(['commander', 'constructed']);
    expect(formatTitle('commander')).toBe('Commander');
  });
});

describe('gridMetrics', () => {
  it('gives a four-table server big cells in two columns', () => {
    const m = gridMetrics(4);
    expect(m.cols).toBe(2);
    expect(m.roomy).toBe(true);
    expect(m.cellMin).toBe('24rem');
  });

  it('steps down as tables multiply', () => {
    const few = gridMetrics(4);
    const mid = gridMetrics(12);
    const many = gridMetrics(13);
    expect(mid.cols).toBeGreaterThan(few.cols);
    expect(many.cols).toBeGreaterThan(mid.cols);
    expect(parseFloat(mid.cellMin)).toBeLessThan(parseFloat(few.cellMin));
    expect(parseFloat(many.cellMin)).toBeLessThan(parseFloat(mid.cellMin));
    expect(parseFloat(many.rowMin)).toBeLessThan(parseFloat(few.rowMin));
    expect([few.lifeSize, mid.lifeSize, many.lifeSize]).toEqual([
      'var(--t-40)',
      'var(--t-28)',
      'var(--t-20)',
    ]);
  });

  it('only calls the largest step roomy', () => {
    expect(gridMetrics(1).roomy).toBe(true);
    expect(gridMetrics(5).roomy).toBe(false);
    expect(gridMetrics(40).roomy).toBe(false);
  });
});
