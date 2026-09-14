import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Decision, Option } from '../protocol';
import { SeatPanelState } from './seatpanel.svelte';
import { REMEMBERED_STORAGE_KEY, emptyRemembered, rememberKey } from './remembered';

// The remembered-answer feature (fb-20260914T062319Z-88b4069a part B) at the
// SeatPanelState level: the checkbox write path (B4), the adopt-time
// auto-answer with its guard and note (B5), and the management write paths.
// The STORE itself (keying, cap, corruption) is pinned in lib/remembered.test.ts.

const { postIntentMock, fetchPendingMock } = vi.hoisted(() => ({ postIntentMock: vi.fn(), fetchPendingMock: vi.fn() }));
vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  postIntent: postIntentMock,
  fetchPending: fetchPendingMock,
}));

const ctx = { seat: 0, token: 'tok' };

const yes = (i: number): Option => ({ index: i, kind: 'yes', label: 'Yes — Bloodghast: Whenever a land enters, Bloodghast may return from the graveyard', player: 0 });
const no = (i: number): Option => ({ index: i, kind: 'no', label: 'No', player: 0 });

/** the REAL engine prompt shape (rules/trigger_queue.go askTriggerOptional). */
const PROMPT = 'Put this optional triggered ability on the stack? — Bloodghast: Whenever a land enters, Bloodghast may return from the graveyard';
const OTHER_PROMPT = "Apply this triggered ability's effect? — Bloodghast: Whenever a land enters, Bloodghast may return from the graveyard";

/** optional is a trigger_optional decision in the engine's measured wire shape. */
const optional = (seq: number, prompt = PROMPT): Decision =>
  ({ seq, player: 0, kind: 'trigger_optional', prompt, min: 1, max: 1, options: [yes(0), no(1)] });

/** quiet is a decision of another kind with the same min/max shape (the gate must hold on kind, not shape). */
const quietModes = (seq: number): Decision =>
  ({ seq, player: 0, kind: 'modes', prompt: PROMPT, min: 1, max: 1, options: [{ index: 0, kind: 'mode', label: 'Mode A', player: 0 }, { index: 1, kind: 'mode', label: 'Mode B', player: 0 }] });

/** fakeStorage is a record-and-inspect Storage (the prio6 fixture pattern). */
function fakeStorage(): Storage {
  const map = new Map<string, string>();
  return {
    get length() { return map.size; },
    clear: () => map.clear(),
    getItem: (k: string) => map.get(k) ?? null,
    key: (i: number) => [...map.keys()][i] ?? null,
    removeItem: (k: string) => { map.delete(k); },
    setItem: (k: string, v: string) => { map.set(k, v); },
  };
}

async function settle(predicate: () => boolean, maxTicks = 200): Promise<void> {
  for (let i = 0; i < maxTicks; i++) {
    if (predicate()) return;
    await Promise.resolve();
  }
  throw new Error(`settle: condition still false after ${maxTicks} microtask ticks`);
}

beforeEach(() => {
  postIntentMock.mockReset();
  fetchPendingMock.mockReset();
  postIntentMock.mockResolvedValue(undefined);
});
afterEach(() => {
  vi.restoreAllMocks();
});

describe('the remember checkbox write path (B4)', () => {
  it('a checked box on a trigger_optional click stores the chosen index under the full-prompt key and persists it', async () => {
    const storage = fakeStorage();
    const p = new SeatPanelState('t1', 1, ctx, storage, null);
    p.adoptView(optional(1));
    expect(p.rememberChoice).toBe(false); // default OFF
    p.rememberChoice = true;
    p.click(0); // Yes — min==max==1, the click IS the answer
    await settle(() => p.postedSeq === 1);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
    expect(postIntentMock.mock.calls[0][2].choices).toEqual([0]);
    expect(p.remembered.entries).toHaveLength(1);
    expect(p.remembered.entries[0].key).toBe(rememberKey('trigger_optional', PROMPT));
    expect(p.remembered.entries[0].choice).toBe(0);
    expect(p.remembered.entries[0].label).toBe('Bloodghast: Whenever a land enters, Bloodghast may return from the graveyard');
    // the store reached localStorage
    const back = JSON.parse(storage.getItem(REMEMBERED_STORAGE_KEY)!) as typeof p.remembered;
    expect(back.entries).toEqual(p.remembered.entries);
  });

  it('an UNchecked click stores nothing', async () => {
    const p = new SeatPanelState('t1', 1, ctx, fakeStorage(), null);
    p.adoptView(optional(1));
    p.click(1); // No, box untouched
    await settle(() => p.postedSeq === 1);
    expect(p.remembered.entries).toHaveLength(0);
    expect(postIntentMock).toHaveBeenCalledTimes(1);
  });

  it('a checked box on another KIND stores nothing (B1 — only trigger_optional is rememberable)', async () => {
    const p = new SeatPanelState('t1', 1, ctx, fakeStorage(), null);
    p.adoptView(quietModes(1));
    p.rememberChoice = true;
    p.click(0);
    await settle(() => p.postedSeq === 1);
    expect(p.remembered.entries).toHaveLength(0);
  });

  it('the checkbox resets on every newly adopted decision: a stale tick never remembers the next ask', async () => {
    const p = new SeatPanelState('t1', 1, ctx, fakeStorage(), null);
    p.adoptView(optional(1));
    p.rememberChoice = true;
    p.click(0);
    await settle(() => p.postedSeq === 1);
    p.adoptView(optional(2, OTHER_PROMPT));
    expect(p.rememberChoice).toBe(false);
    p.click(0);
    await settle(() => p.postedSeq === 2);
    expect(p.remembered.entries).toHaveLength(1); // only the first answer
  });
});

describe('the adopt-time auto-answer (B5)', () => {
  it('the NEXT identical prompt auto-answers the remembered choice with an autoLog note', async () => {
    const storage = fakeStorage();
    const p = new SeatPanelState('t1', 1, ctx, storage, null);
    p.adoptView(optional(1));
    p.rememberChoice = true;
    p.click(0);
    await settle(() => p.postedSeq === 1);

    // A new match state would carry the same prompt at a new seq.
    const fresh = new SeatPanelState('t1', 2, ctx, storage, null);
    expect(fresh.pending).toBeNull();
    fresh.adoptView(optional(9));
    await settle(() => fresh.postedSeq === 9);
    expect(postIntentMock).toHaveBeenCalledTimes(2); // writer's manual click + the auto-answer
    expect(postIntentMock.mock.calls[1][2].choices).toEqual([0]);
    expect(fresh.pending).toBeNull();
    const note = fresh.autoLog.find((n) => n.text.includes('remembered choice'));
    expect(note?.text).toBe('Answered optional trigger from a remembered choice — Bloodghast: Whenever a land enters, Bloodghast may return from the graveyard');
  });

  it('a non-matching prompt does not auto-answer', async () => {
    const storage = fakeStorage();
    const p = new SeatPanelState('t1', 1, ctx, storage, null);
    p.adoptView(optional(1));
    p.rememberChoice = true;
    p.click(0);
    await settle(() => p.postedSeq === 1);

    const fresh = new SeatPanelState('t1', 2, ctx, storage, null);
    fresh.adoptView(optional(9, OTHER_PROMPT));
    await settle(() => fresh.pending?.seq === 9);
    expect(postIntentMock).toHaveBeenCalledTimes(1); // writer's manual click; nothing for the non-matching prompt
    expect(fresh.pending?.seq).toBe(9);
  });

  it('a different KIND with the same prompt never auto-answers (B1)', async () => {
    const p = new SeatPanelState('t1', 1, ctx, null, null);
    p.remembered = { version: 1, entries: [{ key: rememberKey('trigger_optional', PROMPT), choice: 0, label: 'l', savedAt: 1 }] };
    p.adoptView(quietModes(1));
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.pending?.seq).toBe(1);
  });

  it('a rejected auto-answer is never retried forever (the rememberedSeq guard)', async () => {
    const storage = fakeStorage();
    const writer = new SeatPanelState('t1', 1, ctx, storage, null);
    writer.adoptView(optional(1));
    writer.rememberChoice = true;
    writer.click(0);
    await settle(() => writer.postedSeq === 1);

    postIntentMock.mockRejectedValueOnce(new Error('stale seq'));
    fetchPendingMock.mockResolvedValue(null);
    const p = new SeatPanelState('t1', 2, ctx, storage, null);
    p.adoptView(optional(9));
    await settle(() => p.error === 'stale seq'); // the post failed and surfaced
    await settle(() => p.pending === null); // the recovery refetch adopted "nothing pending"
    // The same decision comes back: the guard refuses a seq already auto-answered.
    p.adoptView(optional(9));
    expect(postIntentMock).toHaveBeenCalledTimes(2); // writer's click + the rejected auto-answer; no third attempt
    expect(p.pending?.seq).toBe(9);
  });

  it('a remembered index the decision no longer offers is not applied — the ask stays manual', async () => {
    const p = new SeatPanelState('t1', 1, ctx, null, null);
    p.remembered = { version: 1, entries: [{ key: rememberKey('trigger_optional', PROMPT), choice: 5, label: 'l', savedAt: 1 }] };
    p.adoptView(optional(1));
    expect(postIntentMock).not.toHaveBeenCalled();
    expect(p.pending?.seq).toBe(1);
  });
});

describe('the management write paths (B4)', () => {
  it('removeRemembered deletes one entry and persists; clearRemembered empties and persists', async () => {
    const storage = fakeStorage();
    const p = new SeatPanelState('t1', 1, ctx, storage, null);
    p.adoptView(optional(1));
    p.rememberChoice = true;
    p.click(0);
    await settle(() => p.postedSeq === 1);
    const key = rememberKey('trigger_optional', PROMPT);

    p.removeRemembered('no-such-key');
    expect(p.remembered.entries).toHaveLength(1); // a miss changes nothing
    p.removeRemembered(key);
    expect(p.remembered.entries).toHaveLength(0);
    expect(loadStore(storage)).toEqual(emptyRemembered());

    // seed a fresh entry through the real path, then clear all
    p.adoptView(optional(2));
    p.rememberChoice = true;
    p.click(1);
    await settle(() => p.postedSeq === 2);
    expect(p.remembered.entries).toHaveLength(1);
    p.clearRemembered();
    expect(p.remembered.entries).toHaveLength(0);
    expect(loadStore(storage)).toEqual(emptyRemembered());
  });

  it('a corrupt localStorage value degrades to an empty store for the panel state', () => {
    const storage = fakeStorage();
    storage.setItem(REMEMBERED_STORAGE_KEY, 'not json');
    const p = new SeatPanelState('t1', 1, ctx, storage, null);
    expect(p.remembered).toEqual(emptyRemembered());
    // and the empty store is not re-saved over the corrupt key by the load
    expect(storage.getItem(REMEMBERED_STORAGE_KEY)).toBe('not json');
  });
});

/** loadStore reads the raw persisted store for assertions. */
function loadStore(storage: Storage): unknown {
  const raw = storage.getItem(REMEMBERED_STORAGE_KEY);
  return raw === null ? null : JSON.parse(raw);
}
