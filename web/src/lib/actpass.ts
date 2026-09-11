import type { Decision } from '../protocol';

/**
 * actpass is the "pass after acting" preference: after this seat takes a
 * real action on a priority window (a cast, an ability, a land drop), the
 * seat's NEXT priority window is passed for it once, without a second
 * click. It is NOT auto: it answers exactly one window, only after the
 * player visibly took an action, and every firing is preceded by a human
 * click the player chose to make.
 *
 * The preference is persisted per table AND per seat — the same keying the
 * stop sets use (stops.ts is the pattern) — because it is a durable
 * preference, not an opt-in hand-over: unlike auto it never decides a
 * window the player did not already act on, so persisting it does not leave
 * the game playing itself. OFF is stored explicitly ('0') so a player who
 * turned it off keeps that choice across reloads rather than falling back
 * to the default.
 *
 * The arm/answer loop itself lives in SeatPanelState (seatpanel.svelte.ts);
 * this module is only the storage. Every access is wrapped in try/catch —
 * private mode throws, and a throw must never escape: the in-memory copy
 * keeps working and the next save tries again.
 */

const PREFIX = 'gorge.actpass.';

export function actPassKey(table: string, seat: number): string {
  return `${PREFIX}${table}.${seat}`;
}

/** loadActPass reads the seat's saved preference, or false on absent or corrupt storage. */
export function loadActPass(storage: Storage | null, table: string, seat: number): boolean {
  try {
    return storage?.getItem(actPassKey(table, seat)) === '1';
  } catch {
    return false;
  }
}

/** saveActPass writes the seat's preference. A throw (private mode, quota) is swallowed: the caller's in-memory copy is the source of truth until a later save lands. */
export function saveActPass(storage: Storage | null, table: string, seat: number, on: boolean): void {
  try {
    storage?.setItem(actPassKey(table, seat), on ? '1' : '0');
  } catch {
    /* private mode or quota: keep the in-memory copy */
  }
}

/**
 * actedOption reports whether the posted `choices` (wire indices) contain at
 * least one real action on a priority decision — an option whose kind is
 * neither pass nor concede (cast, ability, play_land, ...). The kind comes
 * off the wire option itself, resolved by index; a choice that names no
 * option on the decision is not an action. This is the arming test for
 * pass-after-acting, and it deliberately mirrors `actionable`'s kind test
 * in autopilot.ts (per-choice rather than per-decision).
 */
export function actedOption(d: Decision, choices: number[]): boolean {
  if (d.kind !== 'priority') return false;
  return choices.some((i) => {
    const o = d.options.find((opt) => opt.index === i);
    return o !== undefined && o.kind !== 'pass' && o.kind !== 'concede';
  });
}
