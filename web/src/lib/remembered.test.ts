import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  REMEMBERED_CAP,
  REMEMBERED_STORAGE_KEY,
  emptyRemembered,
  loadRemembered,
  rememberChoiceFor,
  rememberKey,
  rememberable,
  saveRemembered,
  triggerPromptLabel,
  withRemember,
  withoutRemember,
} from './remembered';

// The store tests pin the whole contract of the remembered-answer feature
// (fb-20260914T062319Z-88b4069a part B): the keying (B2), the storage shape
// and cap (B3), the corruption rule, and the only rememberable kind (B1).

/** fakeStorage is a record-and-inspect Storage (the prio6 fixture pattern). */
function fakeStorage(): Storage & { dump(): Map<string, string> } {
  const map = new Map<string, string>();
  return {
    get length() { return map.size; },
    clear: () => map.clear(),
    getItem: (k: string) => map.get(k) ?? null,
    key: (i: number) => [...map.keys()][i] ?? null,
    removeItem: (k: string) => { map.delete(k); },
    setItem: (k: string, v: string) => { map.set(k, v); },
    dump: () => map,
  };
}

/** throwingStorage refuses every write and read, like a private-mode browser. */
const throwingStorage = {
  get length(): number { throw new Error('denied'); },
  clear: () => { throw new Error('denied'); },
  getItem: () => { throw new Error('denied'); },
  key: () => { throw new Error('denied'); },
  removeItem: () => { throw new Error('denied'); },
  setItem: () => { throw new Error('denied'); },
} as Storage;

// The two REAL engine prompt shapes (rules/trigger_queue.go: askTriggerOptional
// and askOptionalAtResolution, over triggerLabel). B2's unit test pins the key
// contract on these exact strings.
const PLACEMENT_PROMPT = 'Put this optional triggered ability on the stack? — Bloodghast: Whenever a land enters, Bloodghast may return from the graveyard';
const RESOLUTION_PROMPT = "Apply this triggered ability's effect? — Miracle — reveal Thunderous Wrath and cast it for {R}";

describe('rememberKey — the keying contract (B2)', () => {
  it('is the decision kind plus the FULL prompt string, NUL-separated, for both engine shapes', () => {
    expect(rememberKey('trigger_optional', PLACEMENT_PROMPT)).toBe('trigger_optional\u0000' + PLACEMENT_PROMPT);
    expect(rememberKey('trigger_optional', RESOLUTION_PROMPT)).toBe('trigger_optional\u0000' + RESOLUTION_PROMPT);
    // the two engine shapes never collide: their question prefixes differ
    expect(rememberKey('trigger_optional', PLACEMENT_PROMPT)).not.toBe(rememberKey('trigger_optional', RESOLUTION_PROMPT));
  });
});

describe('triggerPromptLabel — the display half of an engine prompt', () => {
  it('extracts the <label> after the " — " separator from both real shapes', () => {
    expect(triggerPromptLabel(PLACEMENT_PROMPT)).toBe('Bloodghast: Whenever a land enters, Bloodghast may return from the graveyard');
    expect(triggerPromptLabel(RESOLUTION_PROMPT)).toBe('Miracle — reveal Thunderous Wrath and cast it for {R}');
  });

  it('returns the whole prompt when there is no separator', () => {
    expect(triggerPromptLabel('Choose one.')).toBe('Choose one.');
  });
});

describe('rememberable — the kind gate (B1)', () => {
  it('is true only for trigger_optional', () => {
    expect(rememberable('trigger_optional')).toBe(true);
    for (const kind of ['priority', 'trigger_order', 'modes', 'choose', 'target', 'mulligan', 'arrange']) {
      expect(rememberable(kind)).toBe(false);
    }
  });
});

describe('withRemember / rememberChoiceFor — the map', () => {
  it('stores a choice under the full-prompt key and looks it back up', () => {
    let store = emptyRemembered();
    store = withRemember(store, rememberKey('trigger_optional', PLACEMENT_PROMPT), 0, 'Bloodghast: …', 1000);
    expect(rememberChoiceFor(store, 'trigger_optional', PLACEMENT_PROMPT)).toBe(0);
    expect(rememberChoiceFor(store, 'trigger_optional', RESOLUTION_PROMPT)).toBeNull();
    // a different kind with the same prompt is a different key
    expect(rememberChoiceFor(store, 'modes', PLACEMENT_PROMPT)).toBeNull();
  });

  it('re-remembering the same prompt REPLACES the entry and moves it to the newest end', () => {
    let store = emptyRemembered();
    store = withRemember(store, rememberKey('trigger_optional', PLACEMENT_PROMPT), 0, 'A', 1000);
    store = withRemember(store, rememberKey('trigger_optional', RESOLUTION_PROMPT), 1, 'B', 2000);
    store = withRemember(store, rememberKey('trigger_optional', PLACEMENT_PROMPT), 1, 'A', 3000);
    expect(store.entries.map((e) => [e.key === rememberKey('trigger_optional', PLACEMENT_PROMPT), e.choice]))
      .toEqual([[false, 1], [true, 1]]);
    expect(rememberChoiceFor(store, 'trigger_optional', PLACEMENT_PROMPT)).toBe(1);
  });

  it('caps at 200 entries and evicts the OLDEST', () => {
    let store = emptyRemembered();
    for (let i = 0; i < REMEMBERED_CAP + 10; i++) {
      store = withRemember(store, rememberKey('trigger_optional', `p${i}`), i % 2, `label ${i}`, i);
    }
    expect(store.entries).toHaveLength(REMEMBERED_CAP);
    // the first ten keys (p0..p9) were evicted; p10 is now the oldest
    expect(rememberChoiceFor(store, 'trigger_optional', 'p9')).toBeNull();
    expect(rememberChoiceFor(store, 'trigger_optional', 'p10')).toBe(0);
    expect(store.entries[0].label).toBe('label 10');
    expect(store.entries[REMEMBERED_CAP - 1].label).toBe(`label ${REMEMBERED_CAP + 9}`);
  });

  it('withoutRemember forgets one key and leaves the rest', () => {
    let store = emptyRemembered();
    store = withRemember(store, rememberKey('trigger_optional', 'p1'), 0, 'A', 1);
    store = withRemember(store, rememberKey('trigger_optional', 'p2'), 1, 'B', 2);
    store = withoutRemember(store, rememberKey('trigger_optional', 'p1'));
    expect(rememberChoiceFor(store, 'trigger_optional', 'p1')).toBeNull();
    expect(rememberChoiceFor(store, 'trigger_optional', 'p2')).toBe(1);
  });
});

describe('loadRemembered / saveRemembered — persistence (B3)', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('round-trips through the ONE global key', () => {
    const s = fakeStorage();
    const store = withRemember(emptyRemembered(), rememberKey('trigger_optional', PLACEMENT_PROMPT), 1, 'label', 42);
    saveRemembered(s, store);
    expect(s.dump().has(REMEMBERED_STORAGE_KEY)).toBe(true);
    const back = loadRemembered(s);
    expect(back.entries).toEqual(store.entries);
  });

  it('absent storage yields the empty store', () => {
    expect(loadRemembered(null)).toEqual(emptyRemembered());
    expect(loadRemembered(fakeStorage())).toEqual(emptyRemembered());
  });

  it('a corrupt value degrades to an empty store (not a partial merge)', () => {
    for (const raw of [
      'not json',
      '[]',
      '{"version":2,"entries":[]}',
      '{"version":1,"entries":"nope"}',
      '{"version":1,"entries":[{"key":"k","choice":2,"label":"l","savedAt":1}]}',
      '{"version":1,"entries":[{"key":"k","choice":0,"label":7,"savedAt":1}]}',
      '{"version":1,"entries":[{"key":"k","choice":0,"label":"l"}]}',
      '{"version":1,"entries":[{"key":"","choice":0,"label":"l","savedAt":1}]}',
      '{"version":1,"entries":[{"key":"k","choice":0,"label":"l","savedAt":"x"}]}',
      '{"version":1,"entries":["junk"]}',
      '{"version":1,"entries":[{"key":"k","choice":0,"label":"l","savedAt":1}],"extra":true,"entries":[]}',
    ]) {
      const s = fakeStorage();
      s.setItem(REMEMBERED_STORAGE_KEY, raw);
      expect(loadRemembered(s), `raw: ${raw}`).toEqual(emptyRemembered());
    }
  });

  it('a throwing storage degrades to empty on read and is swallowed on write', () => {
    expect(loadRemembered(throwingStorage)).toEqual(emptyRemembered());
    expect(() => saveRemembered(throwingStorage, withRemember(emptyRemembered(), 'k', 0, 'l', 1))).not.toThrow();
  });
});
