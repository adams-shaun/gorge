/**
 * yields is the game-scoped "always pass for this ability" set (prio6).
 *
 * WHAT A YIELD IS: one key naming ONE recurring stack entry — an opponent's
 * triggered ability the player is tired of answering, e.g. their Blood
 * Artist's drain. The key is `${controller}:${name}:${text}` off the wire's
 * StackView (controller, the source's face name, the ability's own text) —
 * the same three facts the stack tile renders, so what the player yielded is
 * exactly what they saw. The key deliberately excludes the object id: a
 * yield survives the source leaving and re-entering the battlefield, which
 * is the whole point ("this game", not "this copy").
 *
 * SCOPE: per GAME — one table's one match (the table id + the match
 * number), in memory + sessionStorage. Keying by the match number is what
 * makes the set game-scoped rather than table-scoped: a reload of the same
 * match re-reads the same sessionStorage record (the yields are "for this
 * game"), while a NEW match on the same table gets a fresh key and starts
 * clean — a yield granted in game N never auto-passes the same ability in
 * game N+1 (review r2). sessionStorage (not localStorage) is the mirror, so
 * a new browser session starts clean too, matching MTGO's per-session
 * yields. The in-memory map is the source of truth while the tab lives;
 * the sessionStorage mirror is written through on every change. A browser
 * that refuses site data keeps the memory copy only (the swallowed-throw
 * pattern logshown.ts and stops.ts use).
 *
 * decide() (lib/autopilot) consumes a ReadonlySet of these keys: when the
 * TOP stack entry's key is in the set, the opponent-object rule is skipped
 * for it (the step stops still apply). The seat panel also renders a
 * "yielding" marker on entries whose key is in the set, and GAME OPTIONS
 * carries the "Clear yields" action that empties the set.
 */

const PREFIX = 'gorge.yields.';

/** module-level in-memory copy, keyed by table id + match; survives component remounts within the tab. */
const memory = new Map<string, Set<string>>();

/** scopeKey is the store's identity for one game: the table id and the match number. */
function scopeKey(table: string, match: number): string {
  return `${table}:${match}`;
}

/** yieldKey is a stack entry's yield identity: controller, source name, ability text. */
export function yieldKey(controller: number, name: string, text: string): string {
  return `${controller}:${name}:${text}`;
}

/** stackYieldKey is yieldKey over the wire's StackView shape (the fields the stack tile renders). */
export function stackYieldKey(s: { controller: number; name: string; text: string }): string {
  return yieldKey(s.controller, s.name, s.text);
}

/** yieldsStorageKey is the sessionStorage key for one game's (table + match) yield set. */
export function yieldsStorageKey(table: string, match: number): string {
  return `${PREFIX}${scopeKey(table, match)}`;
}

/** parseYields reads the stored form (a JSON array of keys); anything else is corrupt and yields an empty set. */
function parseYields(raw: string | null): Set<string> {
  if (raw === null) return new Set();
  try {
    const v: unknown = JSON.parse(raw);
    if (!Array.isArray(v) || !v.every((k) => typeof k === 'string')) return new Set();
    return new Set(v as string[]);
  } catch {
    return new Set();
  }
}

/**
 * loadYields returns the game's (table + match) yield set, from the memory
 * map when this tab already has one, else from sessionStorage, else empty.
 * A new match on the same table is a different scope and reads empty. The
 * returned set IS the memory entry: mutating it does not persist — write
 * through saveYields.
 */
export function loadYields(table: string, match: number, storage: Storage | null): ReadonlySet<string> {
  const key = scopeKey(table, match);
  const hit = memory.get(key);
  if (hit !== undefined) return hit;
  let stored: Set<string>;
  try {
    stored = parseYields(storage?.getItem(yieldsStorageKey(table, match)) ?? null);
  } catch {
    stored = new Set();
  }
  memory.set(key, stored);
  return stored;
}

/** saveYields writes the set to both layers; a storage throw is swallowed (memory stays the source of truth). */
export function saveYields(table: string, match: number, yields: ReadonlySet<string>, storage: Storage | null): void {
  memory.set(scopeKey(table, match), new Set(yields));
  try {
    storage?.setItem(yieldsStorageKey(table, match), JSON.stringify([...yields]));
  } catch {
    /* private mode or quota: keep the in-memory copy */
  }
}
