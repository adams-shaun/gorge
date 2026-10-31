import { afterEach, describe, expect, it, vi } from 'vitest';
import { loadProtocol, PROTOCOL_KEY, saveProtocol, tableProtocol } from './pref';

class Mem implements Storage {
  m = new Map<string, string>();
  get length() { return this.m.size; }
  clear() { this.m.clear(); }
  getItem(k: string) { return this.m.get(k) ?? null; }
  key(i: number) { return [...this.m.keys()][i] ?? null; }
  removeItem(k: string) { this.m.delete(k); }
  setItem(k: string, v: string) { this.m.set(k, v); }
}

describe('protocol setting', () => {
  it('defaults to native, round-trips manabrew, and ignores junk', () => {
    const s = new Mem();
    expect(loadProtocol(s)).toBe('native');
    saveProtocol('manabrew', s);
    expect(s.getItem(PROTOCOL_KEY)).toBe('manabrew');
    expect(loadProtocol(s)).toBe('manabrew');
    saveProtocol('native', s);
    expect(s.getItem(PROTOCOL_KEY)).toBeNull();
    s.setItem(PROTOCOL_KEY, 'bogus');
    expect(loadProtocol(s)).toBe('native');
    expect(loadProtocol(null)).toBe('native');
  });

  describe('a table loads over ManaBrew only where the server offers it', () => {
    afterEach(() => vi.unstubAllGlobals());
    const page = (offered: boolean) => vi.stubGlobal('document', { querySelector: () => (offered ? {} : null) });

    it('keeps the stored choice but plays Native when the tag is absent', () => {
      const s = new Mem();
      saveProtocol('manabrew', s);
      page(false);
      expect(tableProtocol(s)).toBe('native');
      expect(loadProtocol(s)).toBe('manabrew');
      page(true);
      expect(tableProtocol(s)).toBe('manabrew');
    });
  });
});
