/**
 * storage is the ONE shared localStorage handle every persistent client
 * store routes through: playsettings, layoutsettings, remembered decisions,
 * logshown, the art cache and the oracle cache. Before fb-20260917T232814Z
 * each module carried its own identical private `safeStorage()` copy — the
 * guard existed six times and no single "storage is unavailable" signal
 * existed anywhere, which is why a browser that refuses site data degraded
 * every store to memory-only in silence (the player-visible symptom: every
 * preference resets on every reload, with nothing on screen saying why).
 *
 * The scope here is deliberately the same as the copies it replaces: BOTH
 * helpers are per-browser, per-origin — a cookie would give a player
 * nothing localStorage does not already give (same lifetime, same origin
 * scope, a 100× smaller limit and request overhead), so the fix for
 * "preferences don't persist" is surfacing the refusal, not switching
 * mechanism. Per-store KEYS and SCOPES are untouched (renaming
 * `gorge.playsettings.v1` would orphan saved settings; the yields store
 * stays sessionStorage — its session scope is a deliberate review-r2
 * contract, and it keeps its own safeSessionStorage copy in seatpanel).
 */

/**
 * PROBE_KEY is the throwaway key storageWritable() round-trips to test
 * writability. Namespaced under `gorge.` like every real store key so a
 * stubbed/shared storage shows the same behaviour as a real one; it is
 * removed immediately, so a working browser never carries it.
 */
export const PROBE_KEY = 'gorge.storage.probe';

/**
 * safeStorage is localStorage where it exists and is reachable; null under
 * a runtime with no `localStorage` global (node tests, a non-browser
 * embedding) and in a browser that throws on ACCESS to site data. A browser
 * that refuses site data by THROWING ON WRITES keeps the handle — reads
 * still work there, and the writes each store already swallows — which is
 * exactly the case storageWritable() exists to surface.
 */
export function safeStorage(): Storage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage;
  } catch {
    return null;
  }
}

/**
 * storageWritable is the one availability signal: true when preferences can
 * actually persist on this origin — the shared handle exists AND a test
 * write round-trips (set + remove). A refusing browser (private mode, ITP,
 * an embedded webview) and a full quota both fail the probe; a browser with
 * working storage passes it.
 *
 * Probed LIVE on every call, never memoised: the panel renders it once per
 * mount, so the cost is two storage calls, and a per-page memo would leak
 * one test's stub into the next (vitest runs each file in one module
 * registry) while giving nothing back — the answer cannot meaningfully
 * change inside one page lifetime except by a quota boundary, and a fresh
 * render re-probes anyway.
 */
export function storageWritable(): boolean {
  const storage = safeStorage();
  if (storage === null) return false;
  try {
    storage.setItem(PROBE_KEY, '1');
    storage.removeItem(PROBE_KEY);
    return true;
  } catch {
    return false;
  }
}
