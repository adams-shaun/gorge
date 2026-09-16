import { describe, expect, it } from 'vitest';
import { LayoutStore, layoutStore } from './layoutsettings.svelte';
import { defaultLayout, LAYOUT_KEY, SCALE_MAX, SCALE_MIN, withHandPeek, withScale, withSteppers } from './layoutsettings';

/**
 * The reactive shell (layoutsettings.svelte.ts) is tested with its timers
 * injected (HoverCard's TimerEnv pattern): the fake clock collects the
 * scheduled callbacks and the test fires them, so the FLASH window is
 * deterministic. Each test builds its own store — the module singleton
 * (layoutStore) is the production binding and is only asserted to exist and
 * to start at the defaults this environment can load (node has no storage).
 */

/** memStorage — same helper the other settings tests use. */
const memStorage = (throwing = false): Storage => {
  const map = new Map<string, string>();
  const guard = <T>(fn: () => T): T => {
    if (throwing) throw new DOMException('denied', 'SecurityError');
    return fn();
  };
  return {
    length: 0,
    clear: () => guard(() => map.clear()),
    getItem: (k: string) => guard(() => map.get(k) ?? null),
    key: (i: number) => guard(() => [...map.keys()][i] ?? null),
    removeItem: (k: string) => guard(() => void map.delete(k)),
    setItem: (k: string, v: string) => guard(() => void map.set(k, v)),
  } as unknown as Storage;
};

/** fakeTimers collects scheduled callbacks; clearTimeout really cancels, so a restarted pulse's timer count is observable. fire() runs them all. */
function fakeTimers() {
  let nextId = 1;
  const pending = new Map<number, () => void>();
  return {
    env: {
      setTimeout: (fn: () => void) => {
        const id = nextId++;
        pending.set(id, fn);
        return id;
      },
      clearTimeout: (id: unknown) => {
        pending.delete(id as number);
      },
    },
    fireAll: () => {
      for (const fn of [...pending.values()]) fn();
      pending.clear();
    },
    count: () => pending.size,
  };
}

function mkStore(throwing = false) {
  const storage = memStorage(throwing);
  const t = fakeTimers();
  const store = new LayoutStore({ storage, ...t.env });
  return { storage, t, store };
}

describe('LayoutStore — loading', () => {
  it('starts at the defaults when storage is empty or missing', () => {
    expect(new LayoutStore({ storage: memStorage() }).settings).toEqual(defaultLayout());
    expect(new LayoutStore({ storage: null }).settings).toEqual(defaultLayout());
  });

  it('adopts a saved layout (and a corrupt blob falls back to defaults)', () => {
    const st = memStorage();
    st.setItem(LAYOUT_KEY, JSON.stringify(withScale(withHandPeek(defaultLayout(), 'always'), 'creatures', 1.2)));
    const store = new LayoutStore({ storage: st });
    expect(store.scale('creatures')).toBe(1.2);
    expect(store.peek).toBe('always');
    const bad = memStorage();
    bad.setItem(LAYOUT_KEY, 'garbage');
    expect(new LayoutStore({ storage: bad }).settings).toEqual(defaultLayout());
  });
});

describe('LayoutStore — writes', () => {
  it('bump moves the scale on the step grid, clamps, persists and pulses the zone', () => {
    const { storage, t, store } = mkStore();
    store.bump('creatures', 0.1);
    expect(store.scale('creatures')).toBe(1.1);
    expect(store.flash.creatures).toBe(true);
    expect(store.flash.lands).toBe(false);
    t.fireAll();
    expect(store.flash.creatures).toBe(false);
    expect(JSON.parse(storage.getItem(LAYOUT_KEY) ?? '{}')).toEqual(store.settings);

    for (let i = 0; i < 20; i++) store.bump('lands', 0.1);
    expect(store.scale('lands')).toBe(SCALE_MAX);
    for (let i = 0; i < 20; i++) store.bump('hand', -0.1);
    expect(store.scale('hand')).toBe(SCALE_MIN);
    t.fireAll();
    store.dispose();
  });

  it('a second pulse inside the window restarts it and does not stack timers', () => {
    const { t, store } = mkStore();
    store.bump('others', 0.1);
    const n = t.count();
    store.bump('others', 0.1);
    // restart = clear the old timer, schedule exactly one new one
    expect(t.count()).toBe(n);
    t.fireAll();
    expect(store.flash.others).toBe(false);
    store.dispose();
  });

  it('setAlign and setHandPeek persist through the store', () => {
    const { storage, store } = mkStore();
    store.setAlign('lands', 'right');
    store.setHandPeek('never');
    expect(store.align('lands')).toBe('right');
    expect(store.peek).toBe('never');
    expect(JSON.parse(storage.getItem(LAYOUT_KEY) ?? '{}')).toEqual(store.settings);
    store.dispose();
  });

  it('setSteppersOnBoard persists (no pulse: hiding a stepper changes no zone geometry)', () => {
    const { storage, t, store } = mkStore();
    expect(store.steppersOnBoard).toBe(true); // the shipped default
    store.setSteppersOnBoard(false);
    expect(store.steppersOnBoard).toBe(false);
    expect(JSON.parse(storage.getItem(LAYOUT_KEY) ?? '{}')).toEqual(store.settings);
    // no dotted-outline pulse was scheduled by the toggle
    expect(t.count()).toBe(0);
    store.setSteppersOnBoard(true);
    expect(store.steppersOnBoard).toBe(true);
    t.fireAll();
    store.dispose();
  });

  it('a store built on a saved pre-toggle v1 blob reads steppersOnBoard as true (the lenient add)', () => {
    const st = memStorage();
    st.setItem(LAYOUT_KEY, JSON.stringify({ ...withScale(defaultLayout(), 'creatures', 1.3), steppersOnBoard: undefined }));
    // JSON.stringify drops the undefined key — exactly the pre-toggle blob shape
    expect(new LayoutStore({ storage: st }).steppersOnBoard).toBe(true);
  });

  it('reset returns every zone to the shipped layout and persists it', () => {
    const { storage, store } = mkStore();
    store.bump('creatures', 0.3);
    store.setAlign('hand', 'left');
    store.reset();
    expect(store.settings).toEqual(defaultLayout());
    expect(JSON.parse(storage.getItem(LAYOUT_KEY) ?? '{}')).toEqual(defaultLayout());
    store.dispose();
  });

  it('a throwing storage keeps the in-memory copy (save swallowed)', () => {
    const { t, store } = mkStore(true);
    expect(() => store.bump('creatures', 0.1)).not.toThrow();
    expect(store.scale('creatures')).toBe(1.1);
    t.fireAll();
    store.dispose();
  });
});

describe('layoutStore singleton', () => {
  it('exists and starts at the defaults (node: no storage reachable)', () => {
    expect(layoutStore).toBeInstanceOf(LayoutStore);
    expect(layoutStore.settings).toEqual(defaultLayout());
  });
});
