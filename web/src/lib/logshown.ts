/**
 * logshown persists whether the transcript (the rules log) is shown, per
 * table. It follows the stops.ts pattern exactly — load/save through a
 * Storage handle (never localStorage directly in a component), safeStorage()
 * guard for SSR and private mode, and a swallowed throw so a browser that
 * refuses site data keeps the in-memory copy.
 *
 * The DEFAULT differs by view (Task ui21): a SEATED player is playing, not
 * reading the log, so it starts hidden; a SPECTATOR leans on the log to
 * follow a game (and its scrubbing), so it stays shown. The default is
 * recorded here rather than in the component — a seat's view is a different
 * thing from a spectator's, and the two scopes get separate stored values so
 * toggling in one does not flip the other.
 */

const PREFIX = 'gorge.log.';

export type LogScope = 'seat' | 'spectator';

export function logShownKey(table: string, scope: LogScope): string {
  return `${PREFIX}${table}.${scope}`;
}

/** defaultLogShown is the starting point for a fresh scope: hidden for a
 *  seated player, shown for a spectator. It is a starting point, not a claim
 *  about how much log anyone wants. */
export function defaultLogShown(scope: LogScope): boolean {
  return scope === 'spectator';
}

/** safeStorage is localStorage where it exists and is reachable; null under
 *  SSR and in a browser that refuses site data. Same guard as stops.ts. */
function safeStorage(): Storage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage;
  } catch {
    return null;
  }
}

/**
 * loadLogShown returns the saved choice, or null when it is absent or
 * corrupt — null meaning "use the default" rather than collapsing onto a
 * value, so a spectator's first-ever visit is shown (the old behaviour) and
 * a seat's first-ever visit is hidden.
 */
export function loadLogShown(storage: Storage | null, table: string, scope: LogScope): boolean | null {
  try {
    const raw = storage?.getItem(logShownKey(table, scope));
    if (raw === null) return null;
    return raw === '1' ? true : raw === '0' ? false : null;
  } catch {
    return null;
  }
}

/** saveLogShown writes the choice. A throw (private mode, quota) is
 *  swallowed: the caller's in-memory copy is the source of truth. */
export function saveLogShown(storage: Storage | null, table: string, scope: LogScope, shown: boolean): void {
  try {
    storage?.setItem(logShownKey(table, scope), shown ? '1' : '0');
  } catch {
    /* private mode or quota: keep the in-memory copy */
  }
}

export { safeStorage };
