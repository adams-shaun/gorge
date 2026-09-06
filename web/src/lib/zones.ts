import type { CardView, PlayerView } from '../protocol';

export type ZoneName = 'graveyard' | 'exile';

export interface ZoneSummary {
  zone: ZoneName;
  cards: CardView[];
  count: number;
}

const ZONES: readonly ZoneName[] = ['graveyard', 'exile'];

function sizeOf(p: PlayerView, z: ZoneName): number {
  // Only graveyard has a `_size` field on the wire; exile carries no
  // separate size, so a null exile has nothing to fall back to and counts 0.
  return z === 'graveyard' ? p.graveyard_size : 0;
}

/**
 * zonesFor projects a player's two card-list zones off the wire view.
 * protocol.ts types PlayerView.graveyard/exile as CardView[] (non-nullable),
 * but a Go nil slice serializes to JSON `null` — the same trap visibleHand in
 * board.ts already documents — so each zone is null-guarded here, once, and
 * the null case never reaches a consumer. Both zones are always returned,
 * even at count 0, so a row layout does not jump as cards arrive.
 *
 * Cards come back most-recently-added first: the engine's Move appends to a
 * zone (events/apply.go), so the last wire element is the top of the
 * graveyard. The `_size` field is the fallback count only when the array
 * itself is null (a seat-scoped view may redact a zone); where the array is
 * present its length is authoritative even if the two disagree. graveyard is
 * the only card-list zone with a `_size` field on the wire — exile has none,
 * so a null exile counts 0.
 */
export function zonesFor(p: PlayerView): ZoneSummary[] {
  return ZONES.map((z) => {
    const cards = p[z] ?? null;
    return {
      zone: z,
      cards: cards === null ? [] : [...cards].reverse(),
      count: cards === null ? sizeOf(p, z) : cards.length,
    };
  });
}

/** countsFor is the rail's slim summary line: library and hand read the wire's `_size` fields (a hidden hand still publishes its size; the cards do not travel), and the two card-list zones prefer the array length where the array is present, falling back to the `_size` field when a seat-scoped view redacts a zone. */
export function countsFor(p: PlayerView): { library: number; hand: number; graveyard: number; exile: number } {
  const gy = p.graveyard ?? null;
  const ex = p.exile ?? null;
  return {
    library: p.library_size,
    hand: p.hand_size,
    graveyard: gy !== null ? gy.length : p.graveyard_size,
    exile: ex !== null ? ex.length : 0,
  };
}
