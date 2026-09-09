import type { Decision, Option } from '../protocol';

/**
 * cardoptions.ts is the ONE mechanism behind three symptoms (task ui21):
 * "abilities tied to a card should be viewable and actionable from options on
 * that card", "highlight cards with valid options", and "valid targets should
 * get highlighted". They are one fact — *the pending decision offers you
 * something involving this card* — and that fact is already on the wire:
 * every decision's Option carries `obj`, the board object the option
 * concerns, whether it is a cast, an activation, a target, an attacker or a
 * blocker. This module groups a decision's options by that `obj`.
 *
 * R-E4-2 (the client is rules-ignorant): the index grouping is a pure
 * re-grouping of options the server already sent. Nothing here, and nothing
 * the board does with it, decides what an ability is, what a legal target
 * is, or what a blocker is. A target option and a cast option are the same
 * shape on the wire, so ONE index serves both "highlight cards with valid
 * options" and "highlight valid targets". No `kind` is special-cased.
 *
 * R-E4-1 (never resolve an option by position): every function here keys
 * options by their own `index` (carried through from the decision). The tile
 * posts the option's own index — never a position in a list the client
 * rebuilt.
 *
 * Pure functions only: no Svelte, no DOM, no storage. `cardoptions.test.ts`
 * builds decisions by hand and asserts nothing about creatures or combat.
 */

/**
 * OptionTone is the board marking's register — the SAME two senses the seat
 * panel already paints with (toneOf in seatpanel.svelte.ts): `initiative`
 * when the game is BLOCKED on this decision (a target, a block, a mode — no
 * pass is possible), `offered` when a window is open and declining is a
 * legal answer (a cast/activate priority window that also carries a pass).
 * If the board marking uses the same distinction it will read correctly for
 * free; see Table.svelte, which resolves tone via toneOf and hands it in.
 */
export type OptionTone = 'initiative' | 'offered' | 'idle';

/**
 * CardOptions is what the board gets for the pending decision: the options
 * indexed by obj, the seat's picked indices (in click order — for a
 * permutation answer the ORDER is the answer), the tone the marking should
 * wear, and the post callback that hands one option index back to the seat
 * panel (the single answer path, R-E4-1).
 */
export interface CardOptions {
  byObj: Map<number, Option[]>;
  picked: number[];
  tone: OptionTone;
  post: (index: number) => void;
}

/**
 * TileOptions is what ONE tile renders: the options its object carries, the
 * already-picked ordinals for that object (in click order — the panel's
 * `{pickedAt + 1}` idiom), the tone, and the post callback. It is null when
 * the pending decision offers this object nothing — which is exactly the
 * "no badge, no mark" state, so the tile never learns a rule to decide it.
 */
export interface TileOptions {
  list: Option[];
  pickedOrder: number[];
  tone: OptionTone;
  post: (index: number) => void;
}

/**
 * optionsByObj indexes a decision's options by Option.obj, preserving wire
 * order within each object. Options with no `obj` — pass, concede, a mode
 * with no source — deliberately belong to no card and are NOT indexed: they
 * stay reachable only from the seat panel, never lost, just not card-anchored.
 * A null decision (no pending ask for this seat, or a spectator) yields an
 * empty map.
 */
export function optionsByObj(decision: Decision | null): Map<number, Option[]> {
  const m = new Map<number, Option[]>();
  if (decision === null) return m;
  for (const o of decision.options) {
    if (o.obj === undefined) continue;
    const list = m.get(o.obj);
    if (list === undefined) m.set(o.obj, [o]);
    else list.push(o);
  }
  return m;
}

/** cardOptions returns the options a decision offers concerning `obj`, in
 *  wire order, or null when it offers none on this object. */
export function cardOptions(map: ReadonlyMap<number, Option[]>, obj: number): Option[] | null {
  return map.get(obj) ?? null;
}

/** hasCardOptions is the cheap "does this object have options" a tile asks
 *  every render — the highlight mark and the affordance gate. */
export function hasCardOptions(map: ReadonlyMap<number, Option[]>, obj: number): boolean {
  const list = map.get(obj);
  return list !== undefined && list.length > 0;
}

/**
 * pickedOrdinalsOf is this object's picked options as their CLICK-ORDER
 * ordinals (1-based, the seat panel's `{pickedAt + 1}` idiom): for each pick
 * in the seat's picked list that belongs to one of `ids`, the position it
 * was picked at. A tile shows these so a player sees what they have already
 * picked and in which order — the same number the panel paints. This is
 * display data only; the option's own index is what gets posted, and that is
 * carried by `list`, never recovered from an ordinal.
 */
function pickedOrdinalsOf(ids: ReadonlySet<number>, picked: readonly number[]): number[] {
  const out: number[] = [];
  for (let i = 0; i < picked.length; i++) {
    if (ids.has(picked[i])) out.push(i + 1);
  }
  return out;
}

/**
 * optionSetFor is the pure per-object reduction: this object's options and
 * the picked ordinals among them (in click order). When the decision offers
 * this object nothing it returns null, so a tile that is not involved in the
 * pending decision carries no affordance and no mark.
 */
export function optionSetFor(
  byObj: ReadonlyMap<number, Option[]>,
  obj: number,
  picked: readonly number[],
): { list: Option[]; pickedOrder: number[] } | null {
  const list = byObj.get(obj);
  if (list === undefined || list.length === 0) return null;
  const ids = new Set(list.map((o) => o.index));
  return { list, pickedOrder: pickedOrdinalsOf(ids, picked) };
}

/**
 * optionSetForMany folds a whole stack group (several interchangeable
 * permanents) into ONE option set, in member order then wire order. A stack
 * collapses to one tile, so the tile speaks for every member: if any member
 * is offered anything, the pile is. Because stack members are
 * interchangeable (same identity and state — see stackIdentical), the
 * concatenation is the pile's options, and each option still carries its own
 * index and obj, so posting one is R-E4-1-correct whichever member it came
 * from.
 */
export function optionSetForMany(
  byObj: ReadonlyMap<number, Option[]>,
  objs: readonly number[],
  picked: readonly number[],
): { list: Option[]; pickedOrder: number[] } | null {
  const list: Option[] = [];
  for (const obj of objs) {
    const l = byObj.get(obj);
    if (l !== undefined) list.push(...l);
  }
  if (list.length === 0) return null;
  const ids = new Set(list.map((o) => o.index));
  return { list, pickedOrder: pickedOrdinalsOf(ids, picked) };
}

/** tileOptions hands a single tile its rendered option set plus the tone and
 *  the post callback — or null when the decision offers this object nothing. */
export function tileOptions(bundle: CardOptions, obj: number): TileOptions | null {
  const set = optionSetFor(bundle.byObj, obj, bundle.picked);
  if (set === null) return null;
  return { list: set.list, pickedOrder: set.pickedOrder, tone: bundle.tone, post: bundle.post };
}

/** tileOptionsMany hands a collapsed-stack tile the pile's combined option
 *  set — or null when no member is offered anything. */
export function tileOptionsMany(bundle: CardOptions, objs: readonly number[]): TileOptions | null {
  const set = optionSetForMany(bundle.byObj, objs, bundle.picked);
  if (set === null) return null;
  return { list: set.list, pickedOrder: set.pickedOrder, tone: bundle.tone, post: bundle.post };
}
