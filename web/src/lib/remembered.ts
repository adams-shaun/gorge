/**
 * remembered is the per-player store of "remember this answer for identical
 * future prompts" choices (fb-20260914T062319Z-88b4069a, part B).
 *
 * WHAT IT IS: the client remembers one answered option index per prompt, so
 * an optional triggered ability that recurs every turn (1496 raw corpus T:
 * lines carry OptionalDecider$) is answered once and auto-answered ever
 * after — with the management UI (PlaySettingsPanel) able to forget one or
 * all. The answer itself is posted through the ordinary seat API exactly
 * like a manual click (SeatPanelState.post), so a remembered answer has
 * zero determinism/replay impact: no engine, wire, view or protocol change
 * is involved.
 *
 * THE KEY (B2) is the full prompt string, prefixed by the decision kind:
 * `kind + '\u0000' + prompt`. The engine embeds both the source card's name
 * and the trigger's TriggerDescription$ in the prompt (triggerLabel,
 * rules/trigger_queue.go), so an equal prompt IS "the same prompt from the
 * same source card" — including two copies of the same card, which the name
 * disambiguates. The engine's two shapes never collide because their
 * question prefixes differ ("Put this optional triggered ability on the
 * stack?" vs "Apply this triggered ability's effect?").
 *
 * SCOPE (B1): only `trigger_optional` decisions are rememberable. The store
 * itself is keyed by the whole kind+prompt pair, so the kind gate lives at
 * the affordance (rememberable / the panel's checkbox), not in the data.
 *
 * PERSISTENCE (B3): ONE global localStorage key,
 * `gorge.remembered-decisions.v1` — deliberately a separate key from
 * `gorge.playsettings.v1`, which is versioned/preset-governed; remembered
 * answers are an open-ended map. Entries are ordered oldest→newest and
 * capped at REMEMBERED_CAP (evict oldest). A corrupt value degrades to an
 * empty store — corrupt means defaults, not a partial merge (the
 * playsettings rule). The storage is per browser per player, which matches
 * the report's "instruct client".
 */

export const REMEMBERED_CAP = 200;

/** The ONE global localStorage key (B3). */
export const REMEMBERED_STORAGE_KEY = 'gorge.remembered-decisions.v1';

/** The only decision kind the remember affordance is offered for (B1). */
export const REMEMBERABLE_KINDS: readonly string[] = ['trigger_optional'];

/** rememberable reports whether a decision kind may offer the remember checkbox and be auto-answered. */
export function rememberable(kind: string): boolean {
  return REMEMBERABLE_KINDS.includes(kind);
}

export interface RememberedEntry {
  /** rememberKey — the decision kind + prompt pair this answer belongs to. */
  key: string;
  /** the remembered option index (the wire index of "yes" or "no"). */
  choice: number;
  /** display label: the prompt's own "<label>" suffix (card name + trigger text), not the full question. */
  label: string;
  /** epoch ms when the answer was remembered. */
  savedAt: number;
}

/** RememberedStore is the whole store: entries ordered oldest→newest, keys unique (a re-remembered prompt REPLACES its entry and moves to the end). */
export interface RememberedStore {
  version: 1;
  entries: RememberedEntry[];
}

/** rememberKey is the memory identity of one ask: the full prompt string under its decision kind (B2). The NUL separator cannot appear in a prompt, so kind and prompt cannot blur into each other. */
export function rememberKey(kind: string, prompt: string): string {
  return kind + '\u0000' + prompt;
}

/**
 * triggerPromptLabel is the display half of a trigger_optional prompt: the
 * engine spells both shapes "…? — <label>" (triggerLabel — card name +
 * TriggerDescription$), so the label after the "? — " separator is what a
 * player recognises. The split anchors on the QUESTION's "? — ", not the
 * last " — ": a Miracle label itself contains an em dash ("Miracle — reveal
 * …"), and splitting at the last separator would clip it. A prompt without
 * the separator is shown whole.
 */
export function triggerPromptLabel(prompt: string): string {
  const at = prompt.indexOf('? — ');
  return at >= 0 ? prompt.slice(at + 4) : prompt;
}

/** emptyRemembered is the fresh / degraded store. */
export function emptyRemembered(): RememberedStore {
  return { version: 1, entries: [] };
}

/**
 * rememberChoiceFor looks a decision's remembered answer up. Absent → null
 * (the ask stays manual). Scanning newest-first: entries are unique by key
 * and ordered oldest→newest, so the last match is the current answer.
 */
export function rememberChoiceFor(store: RememberedStore, kind: string, prompt: string): number | null {
  const key = rememberKey(kind, prompt);
  for (let i = store.entries.length - 1; i >= 0; i -= 1) {
    if (store.entries[i].key === key) return store.entries[i].choice;
  }
  return null;
}

/**
 * withRemember returns the store with one answer remembered: an existing
 * entry for the same key is replaced (and moved to the newest end), the new
 * entry is appended, and the oldest entries fall off past REMEMBERED_CAP.
 * Pure — the caller persists the result.
 */
export function withRemember(store: RememberedStore, key: string, choice: number, label: string, savedAt: number): RememberedStore {
  const entries = store.entries.filter((e) => e.key !== key);
  entries.push({ key, choice, label, savedAt });
  return { version: 1, entries: entries.length > REMEMBERED_CAP ? entries.slice(entries.length - REMEMBERED_CAP) : entries };
}

/** withoutRemember returns the store with one key forgotten. Pure. */
export function withoutRemember(store: RememberedStore, key: string): RememberedStore {
  return { version: 1, entries: store.entries.filter((e) => e.key !== key) };
}

const CHOICES: readonly number[] = [0, 1];

/**
 * validate returns the store from a parsed value, or null when it is
 * corrupt: wrong version, a non-array entry list, an entry with a
 * missing/wrong-typed field or a choice that is not 0 or 1. Corrupt means
 * empty, not a partial merge — a half-understood blob should not half-apply
 * (the playsettings rule).
 */
function validate(v: unknown): RememberedStore | null {
  if (typeof v !== 'object' || v === null || Array.isArray(v)) return null;
  const o = v as Record<string, unknown>;
  if (o.version !== 1 || !Array.isArray(o.entries)) return null;
  const entries: RememberedEntry[] = [];
  for (const e of o.entries) {
    if (typeof e !== 'object' || e === null || Array.isArray(e)) return null;
    const r = e as Record<string, unknown>;
    if (typeof r.key !== 'string' || r.key === '') return null;
    if (typeof r.choice !== 'number' || !CHOICES.includes(r.choice)) return null;
    if (typeof r.label !== 'string') return null;
    if (typeof r.savedAt !== 'number' || !Number.isFinite(r.savedAt)) return null;
    entries.push({ key: r.key, choice: r.choice, label: r.label, savedAt: r.savedAt });
  }
  return { version: 1, entries };
}

/**
 * loadRemembered reads the global key. Absent, corrupt, wrong-version or a
 * throwing storage all yield the empty store; a throw must never escape —
 * private-mode browsers throw on getItem too (the loadSettings pattern).
 */
export function loadRemembered(storage: Storage | null): RememberedStore {
  try {
    const raw = storage?.getItem(REMEMBERED_STORAGE_KEY) ?? null;
    if (raw === null) return emptyRemembered();
    return validate(JSON.parse(raw)) ?? emptyRemembered();
  } catch {
    return emptyRemembered();
  }
}

/** saveRemembered writes the global key. A throw (private mode, quota) is swallowed: the caller's in-memory copy is the source of truth until a later save lands. */
export function saveRemembered(storage: Storage | null, store: RememberedStore): void {
  try {
    storage?.setItem(REMEMBERED_STORAGE_KEY, JSON.stringify(store));
  } catch {
    /* private mode or quota: keep the in-memory copy */
  }
}
