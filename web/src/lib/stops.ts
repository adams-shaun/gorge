import type { Stops } from './autopilot';

/**
 * stops persists a seat's stop sets in localStorage, keyed per table AND
 * per seat so two seats in one browser do not share settings. Every access
 * is wrapped in try/catch -- private mode throws, and a throw must never
 * escape this module: the in-memory copy keeps working and the next save
 * tries again. Every access pattern follows images.ts.
 */

const PREFIX = 'gorge.stop.';

export function stopsKey(table: string, seat: number): string {
  return `${PREFIX}${table}.${seat}`;
}

/**
 * defaultStops is the starting point a new seat gets: stop on your own
 * main phases and declare-attackers, and on the opponent's combat steps.
 * It is a starting point, not a claim about correct play.
 */
export function defaultStops(): Stops {
  return {
    yours: new Set(['main1', 'declare-attackers', 'main2']),
    opponents: new Set(['declare-attackers', 'declare-blockers']),
  };
}

/** parseStops decodes a stored value, or null when it is absent or corrupt. */
function parseStops(raw: string | null): Stops | null {
  if (raw === null) return null;
  let j: unknown;
  try {
    j = JSON.parse(raw);
  } catch {
    return null;
  }
  if (j === null || typeof j !== 'object') return null;
  const o = j as Record<string, unknown>;
  if (!Array.isArray(o.yours) || !Array.isArray(o.opponents)) return null;
  if (!o.yours.every((s) => typeof s === 'string') || !o.opponents.every((s) => typeof s === 'string')) return null;
  return { yours: new Set(o.yours), opponents: new Set(o.opponents) };
}

/** loadStops returns the seat's saved stops, or the defaults on absent or corrupt storage. */
export function loadStops(storage: Storage | null, table: string, seat: number): Stops {
  try {
    const parsed = parseStops(storage?.getItem(stopsKey(table, seat)) ?? null);
    return parsed ?? defaultStops();
  } catch {
    return defaultStops();
  }
}

/** saveStops writes the seat's stops. A throw (private mode, quota) is swallowed: the caller's in-memory copy is the source of truth until a later save lands. */
export function saveStops(storage: Storage | null, table: string, seat: number, stops: Stops): void {
  try {
    const value = JSON.stringify({ yours: [...stops.yours], opponents: [...stops.opponents] });
    storage?.setItem(stopsKey(table, seat), value);
  } catch {
    /* private mode or quota: keep the in-memory copy */
  }
}
