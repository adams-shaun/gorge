import { safeStorage } from '../storage';

/**
 * pref.ts is the client-side protocol setting: which wire a seated table
 * plays over. Native (gorge's own stream) is the default; ManaBrew and
 * SpellBench v2 are opt-ins. It is a property of the player's browser, never
 * sent to the server, and read once when a table mounts (a change applies on
 * the next table load). A server that doesn't offer the chosen wire plays
 * every table over Native and the stored choice waits, untouched.
 */

export type WireProtocol = 'native' | 'manabrew' | 'sbv2';

export const PROTOCOL_KEY = 'gorge.protocol.v1';

export const PROTOCOLS: { value: WireProtocol; label: string }[] = [
  { value: 'native', label: 'Native' },
  { value: 'manabrew', label: 'ManaBrew' },
  { value: 'sbv2', label: 'SpellBench v2' },
];

export function protocolLabel(p: WireProtocol): string {
  return PROTOCOLS.find((x) => x.value === p)?.label ?? p;
}

export function loadProtocol(store: Storage | null = safeStorage()): WireProtocol {
  try {
    const stored = store?.getItem(PROTOCOL_KEY);
    return PROTOCOLS.find((p) => p.value === stored)?.value ?? 'native';
  } catch {
    return 'native';
  }
}

/** wireOffered reads the <meta name="gorge-manabrew"> or <meta name="gorge-sbv2"> tag gorged serves when that wire's routes share this origin. */
export function wireOffered(p: WireProtocol): boolean {
  if (p === 'native') return true;
  return typeof document !== 'undefined' && document.querySelector(`meta[name="gorge-${p}"]`) !== null;
}

/** tableProtocol is the wire a table loads over: the setting, where this server can serve it. */
export function tableProtocol(store: Storage | null = safeStorage()): WireProtocol {
  const p = loadProtocol(store);
  return wireOffered(p) ? p : 'native';
}

export function saveProtocol(p: WireProtocol, store: Storage | null = safeStorage()): void {
  try {
    if (p === 'native') store?.removeItem(PROTOCOL_KEY);
    else store?.setItem(PROTOCOL_KEY, p);
  } catch {
    // A browser refusing site data keeps the default; storage.ts surfaces that.
  }
}

/**
 * activeWire records which wire the mounted table actually plays over (the
 * route sets it; a failed probe leaves it 'native'), so the setting can tell
 * a pending change from the live one. outOfScope marks a table the chosen wire
 * can't serve, which a reconnect would not change.
 */
export const activeWire: { current: WireProtocol | null; outOfScope: boolean } = { current: null, outOfScope: false };
