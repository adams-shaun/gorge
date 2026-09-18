import { afterEach, describe, expect, it, vi } from 'vitest';
import { PROBE_KEY, safeStorage, storageWritable } from './storage';

/**
 * Tests for the ONE shared localStorage handle (fb-20260917T232814Z). Every
 * persistent store — playsettings, layoutsettings, remembered decisions,
 * logshown, the art cache, the oracle cache — routes through safeStorage()
 * here; storageWritable() is the single availability signal the Game
 * Options panel renders as its refusal notice. The stub shapes mirror the
 * fixtures seatpanel.prio6.test.ts and PlaySettingsPanel.svelte.test.ts
 * already use for refusing browsers.
 */

/** workingStorage is a minimal real-behaviour fake backed by a Map. */
function workingStorage() {
  const store = new Map<string, string>();
  return {
    fake: {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => void store.set(k, v),
      removeItem: (k: string) => void store.delete(k),
    },
    store,
  };
}

describe('storage — the one shared handle (fb-20260917T232814Z)', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('safeStorage returns null with no localStorage global (node, non-browser embedding)', () => {
    vi.stubGlobal('localStorage', undefined);
    expect(safeStorage()).toBeNull();
    expect(storageWritable()).toBe(false);
  });

  it('safeStorage returns the handle and a write round-trips when storage works', () => {
    const { fake, store } = workingStorage();
    vi.stubGlobal('localStorage', fake);
    expect(safeStorage()).toBe(fake as unknown as Storage);
    expect(storageWritable()).toBe(true);
    // The probe cleans up after itself: a working browser never carries it.
    expect(store.has(PROBE_KEY)).toBe(false);
  });

  it('storageWritable is false when writes throw (refusing browser, full quota)', () => {
    const refusing = {
      getItem: () => null,
      setItem: () => {
        throw new Error('refused');
      },
      removeItem: () => undefined,
    };
    vi.stubGlobal('localStorage', refusing);
    // The handle itself is still returned — reads keep working in a
    // write-refusing browser, exactly as every store's swallowed save assumed.
    expect(safeStorage()).not.toBeNull();
    expect(storageWritable()).toBe(false);
  });

  it('safeStorage returns null when even ACCESS to localStorage throws', () => {
    const desc = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
    Object.defineProperty(globalThis, 'localStorage', {
      configurable: true,
      get() {
        throw new Error('access refused');
      },
    });
    try {
      expect(safeStorage()).toBeNull();
      expect(storageWritable()).toBe(false);
    } finally {
      if (desc) Object.defineProperty(globalThis, 'localStorage', desc);
      else delete (globalThis as Record<string, unknown>).localStorage;
    }
  });

  it('probes live: a storage that starts refusing and is stubbed again is re-read, never memoised', () => {
    const refusing = {
      getItem: () => null,
      setItem: () => {
        throw new Error('refused');
      },
      removeItem: () => undefined,
    };
    vi.stubGlobal('localStorage', refusing);
    expect(storageWritable()).toBe(false);
    const { fake } = workingStorage();
    vi.stubGlobal('localStorage', fake);
    expect(storageWritable()).toBe(true);
  });
});
