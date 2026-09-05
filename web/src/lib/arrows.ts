import type { View } from '../protocol';

export type End = { obj: number } | { seat: number };
export interface Arrow { from: End; to: End; kind: 'target' | 'attack' | 'block' }

/** arrowsFor reads relationships the server already resolved; it decides nothing about legality. */
export function arrowsFor(view: View): Arrow[] {
  const out: Arrow[] = [];
  for (const s of view.stack) {
    for (const t of s.targets) out.push({ from: { obj: s.id }, to: t.is_player ? { seat: t.player } : { obj: t.obj ?? 0 }, kind: 'target' });
  }
  for (const p of view.players) {
    for (const c of [...p.battlefield].sort((a, b) => a.id - b.id)) {
      if (c.attacking && c.attacking_player !== undefined && c.attacking_player !== null) out.push({ from: { obj: c.id }, to: { seat: c.attacking_player }, kind: 'attack' });
    }
  }
  for (const p of view.players) {
    for (const c of [...p.battlefield].sort((a, b) => a.id - b.id)) {
      for (const b of c.blocked_by ?? []) out.push({ from: { obj: b }, to: { obj: c.id }, kind: 'block' });
    }
  }
  return out;
}
