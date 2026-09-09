import { describe, expect, it } from 'vitest';
import { defaultLogShown, loadLogShown, logShownKey, saveLogShown, type LogScope } from './logshown';

// logshown persists whether the transcript is shown, keyed per table and per
// scope (seat vs spectator), exactly the stops.ts contract. These tests use a
// plain in-memory Storage stub and assert nothing about the DOM.

class MemStorage implements Storage {
  private m = new Map<string, string>();
  get length() { return this.m.size; }
  clear() { this.m.clear(); }
  getItem(k: string) { return this.m.get(k) ?? null; }
  key(i: number) { return [...this.m.keys()][i] ?? null; }
  removeItem(k: string) { this.m.delete(k); }
  setItem(k: string, v: string) { this.m.set(k, v); }
}

describe('logshown — per-table, per-scope persistence', () => {
  it('the default differs by view: hidden for a seat, shown for a spectator', () => {
    expect(defaultLogShown('seat')).toBe(false);
    expect(defaultLogShown('spectator')).toBe(true);
  });

  it('saves and reloads a choice keyed by table AND scope', () => {
    const s = new MemStorage();
    saveLogShown(s, 't1', 'seat', true);
    saveLogShown(s, 't1', 'spectator', false);
    saveLogShown(s, 't2', 'seat', false);

    expect(loadLogShown(s, 't1', 'seat')).toBe(true);
    expect(loadLogShown(s, 't1', 'spectator')).toBe(false);
    expect(loadLogShown(s, 't2', 'seat')).toBe(false);
  });

  it('a seat and a spectator on the same table do not share a choice', () => {
    const s = new MemStorage();
    saveLogShown(s, 't1', 'seat', false);
    saveLogShown(s, 't1', 'spectator', true);
    // stored under distinct keys
    expect(logShownKey('t1', 'seat')).not.toBe(logShownKey('t1', 'spectator'));
    expect(loadLogShown(s, 't1', 'seat')).toBe(false);
    expect(loadLogShown(s, 't1', 'spectator')).toBe(true);
  });

  it('an absent or corrupt value returns null, so the default is applied', () => {
    const s = new MemStorage();
    expect(loadLogShown(s, 't1', 'seat')).toBeNull();
    s.setItem(logShownKey('t1', 'seat'), 'banana');
    expect(loadLogShown(s, 't1', 'seat')).toBeNull();
  });

  it('a null storage handle (SSR / refused site data) never throws and returns null / no-ops', () => {
    expect(loadLogShown(null, 't1', 'seat')).toBeNull();
    expect(() => saveLogShown(null, 't1', 'seat', true)).not.toThrow();
  });

  it('the persisted value round-trips repeatedly (a toggle is stable)', () => {
    const s = new MemStorage();
    let shown: boolean = defaultLogShown('seat' as LogScope);
    for (let i = 0; i < 3; i++) {
      shown = !shown;
      saveLogShown(s, 't1', 'seat', shown);
      expect(loadLogShown(s, 't1', 'seat')).toBe(shown);
    }
  });
});
