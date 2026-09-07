import type { CardView, PlayerView, StackView } from '../protocol';

/**
 * The commander rules the client is allowed to know, as structured data —
 * never words the engine put in a label. Everything here keys off the wire's
 * own fields (`command`, `commanders`, `commander_casts`, `cmd_damage`,
 * `mana_cost`) and exists so components render CR 903.8's tax and CR 903.10's
 * clock without doing arithmetic in markup.
 */

/** CR 903.10: 21 cumulative damage from ANY single commander is lethal, regardless of life total. */
export const LETHAL_CMD_DAMAGE = 21;
/** The danger line: 19–20 from one commander is two or fewer points from lethal, and must read that way even at a healthy life total. */
export const CRIT_CMD_DAMAGE = 19;
/** CR 903.8: each prior command-zone cast adds {2} to the commander's next cast's total cost. */
export const TAX_PER_CAST = 2;

/** The zone a commander currently sits in, resolved from the wire's zone lists. */
export type CommanderZone = 'command' | 'battlefield' | 'graveyard' | 'exile' | 'hand' | 'stack' | 'library';

/**
 * How the board's command-zone tile draws a commander. Three states, and the
 * state is a property of WHERE the card is, never of what it is. The tile is
 * the command zone itself — a place on the board, not a picture of a card —
 * so the box is always there, with a little dotted border, and what is IN it
 * depends on where the commander actually is:
 *
 *   command      it is sitting in the command zone. The art is greyed: the
 *                card is here, in the box, but not in play, and greying says
 *                "in the zone". It is the only state a next cast can be
 *                priced for, so the CR 903.8 tax chip stays with it.
 *   battlefield  the real permanent is drawn in the creatures row two inches
 *                away, and it is in play — so the box shows FULL art, the
 *                same card visible twice because both are true of one object.
 *   away         anywhere else — graveyard, exile, hand, the stack, or a zone
 *                this viewer cannot see into at all. An EMPTY box: no art,
 *                because the card is not somewhere you can look, but still
 *                fully inspectable, because reading an opponent's commander
 *                while it sits somewhere nobody can browse is the whole point
 *                of drawing it on the board.
 */
export type CommanderPresence = 'command' | 'battlefield' | 'away';

/**
 * presenceOf collapses the seven zones a commander can be in into the three
 * states the board tile has. Everything that is neither the command zone nor
 * the battlefield is one state: a commander in the graveyard and a commander
 * in a library the viewer cannot see are read the same way — it is not here,
 * and here is where you can still read it.
 */
export function presenceOf(zone: CommanderZone): CommanderPresence {
  if (zone === 'command') return 'command';
  if (zone === 'battlefield') return 'battlefield';
  return 'away';
}

/** One commander of one seat, as the command-zone reader needs it. */
export interface CommanderStatus {
  /** index into the parallel wire arrays `commanders`/`commander_casts` */
  index: number;
  commander: CardView;
  /** prior command-zone casts of this commander (`commander_casts[index]`) */
  casts: number;
  /** CR 903.8: the additional {2}-per-cast on the next command-zone cast — the derived number, never the raw count */
  tax: number;
  /** where the commander currently sits, resolved off the wire's zone lists */
  zone: CommanderZone;
  /** the three-way state the board tile draws — presenceOf(zone) */
  presence: CommanderPresence;
  /** true while the commander is in the command zone — the only state a next cast is offered from */
  inZone: boolean;
}

/** One commander-damage tall, as the damage-taker's reader needs it. Each commander is its own entry — never summed. */
export interface CommanderDamageEntry {
  /** the commander's object id — stable across zones, keys the wire's `cmd_damage` map */
  id: number;
  /** the commander's name, resolved from the damage-DEALER's roster (the taker's own view does not carry it) */
  name: string;
  /** the seat the commander belongs to — the reader's per-opponent scan order */
  fromSeat: number;
  /** cumulative commander damage the taker has taken from this commander (CR 903.10) */
  amount: number;
  /** amount >= LETHAL_CMD_DAMAGE */
  lethal: boolean;
  /** amount >= CRIT_CMD_DAMAGE (two or fewer points from lethal) */
  crit: boolean;
}

/** taxOf is CR 903.8's arithmetic, done once, here: {2} for every prior command-zone cast. */
export function taxOf(casts: number): number {
  return casts * TAX_PER_CAST;
}

/**
 * nextCastCost is the cost the reader decides with: the printed cost plus the
 * CR 903.8 tax as its own trailing generic pip, so the printed card is never
 * rewritten and the tax stays visible as an addition. Zero casts returns the
 * printed cost untouched. The result is still valid Forge notation, so
 * ManaSymbols renders it like any other cost.
 */
export function nextCastCost(cost: string | undefined, casts: number): string {
  const tax = taxOf(casts);
  if (tax <= 0 || !cost) return cost ?? '';
  return `${cost} ${tax}`;
}

/**
 * zoneOf pins a commander's current zone from its seat's zone lists. The
 * roster CardView projects the object's CURRENT zone state but carries no
 * zone field, so presence in a list decides. Order matters: a commander on
 * the battlefield is not simultaneously in the command zone. `hand` only
 * resolves for the viewer's own seat (the wire nulls other seats' hands); a
 * commander that appears in none of its seat's visible lists — a spell on
 * the stack is in `stackIds`, not a zone list, and anything truly hidden
 * (library) can only degrade — reports 'library'.
 */
function zoneOf(id: number, zoneIds: Record<Exclude<CommanderZone, 'stack' | 'library'>, Set<number>>, stackIds: ReadonlySet<number>): CommanderZone {
  if (zoneIds.command.has(id)) return 'command';
  if (zoneIds.battlefield.has(id)) return 'battlefield';
  if (zoneIds.graveyard.has(id)) return 'graveyard';
  if (zoneIds.exile.has(id)) return 'exile';
  if (zoneIds.hand.has(id)) return 'hand';
  if (stackIds.has(id)) return 'stack';
  return 'library';
}

/**
 * commandZoneOf projects one seat's command zone for the board's command
 * area: every roster commander with its CR 903.8 tax, its current zone and
 * the presence state its tile draws, in genesis order. `commanders` is the
 * roster built once at genesis and never shrunk, so it — not `command` — is
 * what is iterated: a commander that has left the zone still has a tile, and
 * that is the state the tile exists to show.
 *
 * `command` (the zone) is a public wire field for every seat and every
 * viewer, so inZone is decided purely from the wire — a seat-scoped reader
 * sees every seat's command zone exactly as a spectator does. An empty roster
 * (a Constructed game) yields an empty list, and the caller draws NOTHING for
 * it: no tile, no slot, no heading, no empty frame.
 */
export function commandZoneOf(p: PlayerView, stackIds: ReadonlySet<number> = new Set()): CommanderStatus[] {
  const zoneIds = {
    command: new Set((p.command ?? []).map((c) => c.id)),
    battlefield: new Set((p.battlefield ?? []).map((c) => c.id)),
    graveyard: new Set((p.graveyard ?? []).map((c) => c.id)),
    exile: new Set((p.exile ?? []).map((c) => c.id)),
    hand: new Set((p.hand ?? []).map((c) => c.id)),
  };
  const roster = p.commanders ?? [];
  const casts = p.commander_casts ?? [];
  const out: CommanderStatus[] = [];
  for (let index = 0; index < roster.length; index++) {
    const commander = roster[index];
    if (commander === undefined) continue;
    const n = casts[index] ?? 0;
    const zone = zoneOf(commander.id, zoneIds, stackIds);
    out.push({
      index,
      commander,
      casts: n,
      tax: taxOf(n),
      zone,
      presence: presenceOf(zone),
      inZone: zone === 'command',
    });
  }
  return out;
}

/**
 * commanderDamageOf resolves the wire's `cmd_damage` map — keyed by the
 * damage-dealing commander's object id — into per-commander entries for the
 * damage taker's own readout. The taker's roster does not contain the
 * dealers' commanders, so names come from a match-wide roster lookup
 * (`players`). Every commander is its own entry with its own amount and its
 * own crit/lethal state: 11 from one commander and 11 from another are two
 * entries, never the 22 that misleads. Entries are ordered by the dealer's
 * seat for deterministic rendering. Nil/absent damage yields [].
 */
export function commanderDamageOf(taker: PlayerView, players: PlayerView[]): CommanderDamageEntry[] {
  const roster = new Map<number, { name: string; seat: number }>();
  for (const p of players) {
    for (const c of p.commanders ?? []) {
      roster.set(c.id, { name: c.name, seat: p.seat });
    }
  }
  const out: CommanderDamageEntry[] = [];
  for (const [id, amount] of Object.entries(taker.cmd_damage ?? {})) {
    const n = Number(id);
    const src = roster.get(n);
    out.push({
      id: n,
      name: src?.name ?? `Commander ${n}`,
      fromSeat: src?.seat ?? -1,
      amount,
      lethal: amount >= LETHAL_CMD_DAMAGE,
      crit: amount >= CRIT_CMD_DAMAGE,
    });
  }
  out.sort((a, b) => a.fromSeat - b.fromSeat || a.id - b.id);
  return out;
}

/** stackIdsOf collects the object ids sitting on the stack — a cast commander is a spell there, not in any zone list. */
export function stackIdsOf(stack: StackView[]): Set<number> {
  const ids = new Set<number>();
  for (const s of stack) {
    if (s.card !== undefined && s.card !== null) ids.add(s.card.id);
    if (s.source !== undefined) ids.add(s.source);
  }
  return ids;
}
