/**
 * Visual effects (GPU filters): a per-browser view setting, on / off,
 * default on. Off drops the zoomed card's drop-shadow.
 *
 * The client uses no backdrop-filter at all, whatever this setting says. On
 * an AMD iGPU (Granite Ridge, Mesa radeonsi 25.2, Chrome 156) a board
 * carrying backdrop-filter made Chrome's GPU process raise amdgpu page
 * faults within 1.5-3 minutes; the ring resets that follow wedge the whole
 * desktop. Measured 2026-10-30 with a guarded repro on a live 4-seat table:
 * faults at 93 s and 180 s with backdrop-filter, a full 10 minutes clean
 * with only backdrop-filter removed. A web page cannot fix the driver, so
 * the client never issues the paint.
 *
 * `?fx=off` / `?fx=on` in the URL sets the value and saves it, so one link
 * reproduces either side of an A/B. Client-side only: the value lives in
 * this browser under `gorge.fx.v1`.
 */
export type FxMode = 'on' | 'off';

export const FX_MODES: readonly FxMode[] = ['on', 'off'];
export const FX_MODE_LABELS: Record<FxMode, string> = { on: 'On', off: 'Off' };
export const DEFAULT_FX_MODE: FxMode = 'on';
export const FX_KEY = 'gorge.fx.v1';
/** FX_OFF_CLASS goes on <html> while effects are off; the filtered rules are scoped under it. */
export const FX_OFF_CLASS = 'fx-off';

export function isFxMode(v: unknown): v is FxMode {
  return typeof v === 'string' && (FX_MODES as readonly string[]).includes(v);
}

/** fxFromURL reads a `fx=` override from a query string; anything else is no override. */
export function fxFromURL(search: string): FxMode | null {
  try {
    const v = new URLSearchParams(search).get('fx');
    return isFxMode(v) ? v : null;
  } catch {
    return null;
  }
}

/** loadFxMode reads the saved mode; a missing, unreadable or foreign blob is the default. */
export function loadFxMode(storage: Storage | null): FxMode {
  try {
    const raw = storage?.getItem(FX_KEY);
    if (!raw) return DEFAULT_FX_MODE;
    const blob = JSON.parse(raw) as { version?: unknown; mode?: unknown };
    return blob && blob.version === 1 && isFxMode(blob.mode) ? blob.mode : DEFAULT_FX_MODE;
  } catch {
    return DEFAULT_FX_MODE;
  }
}

export function saveFxMode(storage: Storage | null, mode: FxMode): void {
  try {
    storage?.setItem(FX_KEY, JSON.stringify({ version: 1, mode }));
  } catch {
    // A browser refusing site data keeps the choice for this page only.
  }
}
