import type { CardView, Decision, Option } from '../protocol';

/**
 * arrange.ts is the pure half of the arrange popup (brief Job 4). A KArrange
 * decision (kind 'arrange') is the engine's ordered-subset ask — a pure
 * reorder of the top N library cards (Min == Max == N), or a Scry/Surveil
 * (Min 0, Max N): the picked options in answer order become the keep pile
 * that goes back on top in that order, and the unpicked options go to the
 * destination named by the shared Option.Kind ("bottom", "graveyard") in
 * OFFERED order (rules/arrange.go handleArrange). These helpers derive the
 * keep/pool split and the drag-reorder move; the component only wires them
 * to the DOM.
 */

/** arrangeDestination is pile B's destination in the UI's own words, from the shared Option.Kind (the same vocabulary rules/arrange.go switches on). */
export function arrangeDestination(d: Decision): string {
  const kind = d.options[0]?.kind;
  switch (kind) {
    case 'graveyard': return 'the graveyard';
    case 'exile': return 'exile';
    case 'hand': return 'the hand';
    default: return 'the bottom of the library';
  }
}

/** ArrangeSplit is one arrange decision's two piles: keep, in PICKED order (pile A, back on top in this order), and pool, in OFFERED order (pile B, to the destination). */
export interface ArrangeSplit {
  keep: Option[];
  pool: Option[];
}

/**
 * arrangeSplit derives the two piles from the decision and the picked
 * indices (the SeatPanelState `picked` array, which for a KArrange decision
 * IS the keep pile in order). A caller seeding a surface from the seat's
 * picked array passes it through arrangeSeed first: a pure reorder must open
 * with EVERY card kept (offered order where the seat has not yet picked).
 * Pile A is the picked options in picked order; pile B is the rest in
 * offered order. An index in `picked` that names no
 * option of this decision is ignored, never invented — the split is always
 * built from the wire's own options.
 */
export function arrangeSplit(d: Decision, picked: readonly number[]): ArrangeSplit {
  const byIndex = new Map(d.options.map((o) => [o.index, o]));
  const keep: Option[] = [];
  for (const i of picked) {
    const o = byIndex.get(i);
    if (o !== undefined) keep.push(o);
    byIndex.delete(i);
  }
  const pool: Option[] = d.options.filter((o) => byIndex.has(o.index));
  return { keep, pool };
}

/**
 * arrangeSeed is the picked set an arrange surface STARTS from, given the
 * decision and the seat's current picked array (the SeatPanelState `picked`):
 *
 *  - a scry/surveil (min < max): the picked prefix as-is — the player's
 *    inline click-order work, which the popup continues;
 *  - a PURE REORDER (min == max == len(options), every card must be kept):
 *    the picked prefix is honoured when non-empty, and the options the seat
 *    has NOT picked follow in OFFERED order — so a fresh ask opens with every
 *    card in the keep row in offered order (the reorder surface the ask is),
 *    and a partial inline pick continues in the modal instead of stranding
 *    the unpicked cards in a pool the ask does not have.
 *
 * Indices naming no option of this decision, and repeats, are dropped — the
 * seed is always a subset of the wire's own options.
 */
export function arrangeSeed(d: Decision, picked: readonly number[]): number[] {
  const seen = new Set<number>();
  const valid: number[] = [];
  for (const i of picked) {
    if (!seen.has(i) && d.options.some((o) => o.index === i)) {
      seen.add(i);
      valid.push(i);
    }
  }
  if (!(d.min === d.max && d.max === d.options.length)) return valid;
  const rest = d.options.filter((o) => !seen.has(o.index)).map((o) => o.index);
  return [...valid, ...rest];
}

/**
 * arrangeOrder is the keep pile's option indices in the split's keep order —
 * exactly what the arrange popup's submit writes back into `picked`, so the
 * posted `choices` are the final visual order. The old click-order answer
 * posted the same array for the same final ordering (a click appends the
 * clicked index, so clicking in the final order builds the same picked
 * array); this is the byte-equivalence the brief pins.
 */
export function arrangeOrder(split: ArrangeSplit): number[] {
  return split.keep.map((o) => o.index);
}

/**
 * arrangeFromOrder rebuilds the two piles from a keep ORDER (option indices in
 * keep order): the inverse of arrangeOrder for a hand-built order. Options
 * named by the order become the keep pile in that order; the rest stay in the
 * pool in offered order. An order index that names no option is ignored.
 */
export function arrangeFromOrder(d: Decision, order: readonly number[]): ArrangeSplit {
  const byIndex = new Map(d.options.map((o) => [o.index, o]));
  const keep: Option[] = [];
  for (const i of order) {
    const o = byIndex.get(i);
    if (o !== undefined) {
      keep.push(o);
      byIndex.delete(i);
    }
  }
  return { keep, pool: d.options.filter((o) => byIndex.has(o.index)) };
}

/**
 * moveWithin reorders a list by moving the item at `from` to position `to`
 * (the keep pile's drag/drop and click-to-place move; a pure array op so the
 * ordering logic is testable without a DOM drag event).
 */
export function moveWithin<T>(list: readonly T[], from: number, to: number): T[] {
  if (from < 0 || from >= list.length || to < 0 || to >= list.length || from === to) return [...list];
  const out = [...list];
  const [item] = out.splice(from, 1);
  out.splice(to, 0, item);
  return out;
}

/**
 * arrangeCard synthesizes the CardView an arrange option renders as a face.
 * The wire does NOT carry CardViews for library cards (the view projects a
 * hidden library as a count; the DECISION is the payload that carries these
 * cards, and its options carry `Obj` and the card's name as `Label`), so the
 * face is built from the label: real art resolves by exact name through the
 * art proxy (lib/images), and the no-art blank shows the name. Only
 * CardImage's read fields are filled; nothing else reads this card.
 */
export function arrangeCard(d: Decision, o: Option): CardView {
  const id = o.obj ?? -1;
  return {
    id,
    name: o.label,
    types: '',
    printing: { name: o.label },
    token: `#${id}`,
    tapped: false,
    power: 0,
    toughness: 0,
    damage: 0,
    attacking: false,
    controller: o.player ?? d.player,
    owner: o.player ?? d.player,
    summon_sick: false,
  };
}
