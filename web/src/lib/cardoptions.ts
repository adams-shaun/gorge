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
 *
 * post's `holdPriority` (prio3) threads the Ctrl modifier from the tile
 * click into SeatPanelState.click's `{ holdPriority }`: Ctrl held while
 * submitting a cast/ability must not arm pass-after-acting for that one
 * action. The route's own boardOptions.post is what finally reaches
 * panel.click, so this signature is the contract every tile affordance
 * (direct icon, radial wheel, long list menu — CardTile, CommanderTile,
 * IdentityBar, HandFan) speaks.
 */
export interface CardOptions {
  /** Object the pending decision resolves for. When absent, ordinary
   * priority/options windows produce no preview relationships. */
  source?: number;
  byObj: Map<number, Option[]>;
  /** byPlayer indexes the same decision's player-target options (see
   *  optionsByPlayer) by the targeted seat, so a seat with nothing else on
   *  the board to target (an opponent, a spell that can only hit a player)
   *  still gets marked. Built alongside byObj from the same decision. */
  byPlayer: Map<number, Option[]>;
  picked: number[];
  tone: OptionTone;
  /** Object whose next local choice should open immediately after a direct
   *  card action posted the preceding decision. */
  autoOpenObj?: number;
  post: (index: number, expectFollowUp?: boolean, holdPriority?: boolean) => void;
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
  autoOpen?: boolean;
  post: (index: number, expectFollowUp?: boolean, holdPriority?: boolean) => void;
}

/**
 * The badge's scenario icon, derived from the option's WIRE kind — a pure
 * wire-kind → glyph mapping (R-E4-2: display, not rules). A one-option card
 * acts directly instead of opening a one-row menu. The wire can distinguish a
 * cast, the dedicated mana activation (`activate`, whose server label is
 * "Tap … for mana"), a target candidate (`permanent` object / `player` seat),
 * an attacker declaration and a blocker declaration. A general `ability`
 * option does not carry its cost, so it stays neutral rather than guessing
 * whether that ability taps; every other kind (sacrifice, discard, x, name,
 * …) is equally neutral.
 */
export type SingleActionIcon = 'cast' | 'tap' | 'target' | 'attack' | 'block' | 'action';

/**
 * scenarioIconOf maps ONE option's wire kind to its scenario icon. Exactly the
 * four scenarios the reporter named plus the two pre-existing ones: cast, tap,
 * target (bullseye), attacker (sword), blocker (shield), neutral.
 */
export function scenarioIconOf(kind: string): SingleActionIcon {
  switch (kind) {
    case 'cast': return 'cast';
    case 'activate': return 'tap';
    case 'permanent':
    case 'player': return 'target';
    case 'attacker': return 'attack';
    case 'block': return 'block';
    default: return 'action';
  }
}

export function singleActionIcon(option: Option): SingleActionIcon {
  return scenarioIconOf(option.kind);
}

/**
 * ACTION_GLYPHS is the icon → text-glyph table the badges render. Plain text
 * glyphs in the existing register (↻ ✦ ›), not colour emoji: the two new
 * pictographs carry an explicit U+FE0E text-presentation selector so a font
 * that owns both a text and an emoji face (U+2694 ⚔ is in Noto Color Emoji)
 * keeps the monochrome one. Verified against the fonts this deployment's
 * browsers fall back to (IBM Plex Mono lacks all three; DejaVu Sans Mono has
 * ◎ and ⚔, Noto Sans Symbols/FreeSans have ⛨) — see the task report.
 */
export const ACTION_GLYPHS: Record<SingleActionIcon, string> = {
  cast: '✦',
  tap: '↻',
  target: '◎',
  attack: '⚔\uFE0E',
  block: '⛨\uFE0E',
  action: '›',
};

/** The count-badge noun for each icon: "3 targets for Ari", "2 blocks for
 *  Bear" — wording that names the scenario, not just "actions". */
const SCENARIO_NOUN: Record<SingleActionIcon, string> = {
  cast: 'plays',
  tap: 'activations',
  target: 'targets',
  attack: 'attacks',
  block: 'blocks',
  action: 'actions',
};

export interface TileScenario {
  icon: SingleActionIcon;
  /** Count-badge noun, e.g. "targets" for a target list. */
  noun: string;
}

/**
 * tileScenario derives the scenario a TILE's option list names, for the count
 * badge: the scenario only when EVERY option in the list maps to the same
 * icon — a list that genuinely mixes scenarios (a cast next to an ability,
 * a target next to a block) gets null and the badge falls back to the bare
 * neutral count, because a mixed icon would claim a scenario the list does
 * not have. The rule never lies and never needs to know a decision kind.
 */
export function tileScenario(tile: Pick<TileOptions, 'list'>): TileScenario | null {
  if (tile.list.length === 0) return null;
  const first = scenarioIconOf(tile.list[0].kind);
  for (const option of tile.list) {
    if (scenarioIconOf(option.kind) !== first) return null;
  }
  return { icon: first, noun: SCENARIO_NOUN[first] };
}

/**
 * A collapsed pile of interchangeable mana sources can act like one physical
 * button when every offered action has identical object-independent wire
 * semantics. The option with the lowest wire index wins; member/list order is
 * not option identity (R-E4-1).
 */
export function singleTapOptionOf(tile: TileOptions): Option | null {
  if (tile.list.length === 0 || tile.list.some((o) => o.kind !== 'activate')) return null;

  const semantics = (option: Option) => JSON.stringify(
    Object.entries(option)
      .filter(([key]) => key !== 'index' && key !== 'obj')
      .sort(([a], [b]) => a.localeCompare(b)),
  );
  const firstShape = semantics(tile.list[0]);
  if (tile.list.some((o) => semantics(o) !== firstShape)) return null;

  return tile.list.reduce((first, option) => option.index < first.index ? option : first);
}

/** Post one displayed option by its WIRE index (R-E4-1), never list position. `holdPriority` (Ctrl held) skips pass-after-acting for this one action. */
export function postTileOption(tile: TileOptions, option: Option, holdPriority = false): void {
  tile.post(option.index, false, holdPriority);
}

/** Post a direct action by its WIRE index (R-E4-1), never list position. `holdPriority` (Ctrl held) skips pass-after-acting for this one action. */
export function postSingleAction(tile: TileOptions, expectFollowUp = false, holdPriority = false): void {
  const option = tile.list.length === 1 ? tile.list[0] : singleTapOptionOf(tile);
  if (option === undefined || option === null) return;
  if (expectFollowUp) tile.post(option.index, true, holdPriority);
  else tile.post(option.index, false, holdPriority);
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

/**
 * optionsByPlayer indexes a decision's options by Option.player, but ONLY
 * for options that target a PLAYER rather than an object — kind `"player"`,
 * the server's own label for a target candidate with no `obj` (rules/stack.go
 * legalTargetCandidates: a player candidate carries `obj: 0`, an object
 * candidate carries `kind: "permanent"`). This is deliberately narrower than
 * "every option missing obj": pass/concede/a sourceless mode also carry no
 * obj but their `player` field names the ACTING seat, not something offered
 * to be targeted — indexing those by player would wrongly mark whoever's
 * turn it is as a legal target. A spell whose only legal target is an
 * opponent (no creatures on board to target) previously marked NOTHING on
 * the board at all, because optionsByObj alone has no way to represent a
 * targetable player — this is the index that closes that gap.
 */
export function optionsByPlayer(decision: Decision | null): Map<number, Option[]> {
  const m = new Map<number, Option[]>();
  if (decision === null) return m;
  for (const o of decision.options) {
    if (o.kind !== 'player') continue;
    const list = m.get(o.player);
    if (list === undefined) m.set(o.player, [o]);
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
  return {
    list: set.list,
    pickedOrder: set.pickedOrder,
    tone: bundle.tone,
    autoOpen: bundle.autoOpenObj === obj,
    post: bundle.post,
  };
}

/** tileOptionsMany hands a collapsed-stack tile the pile's combined option
 *  set — or null when no member is offered anything. */
export function tileOptionsMany(bundle: CardOptions, objs: readonly number[]): TileOptions | null {
  const set = optionSetForMany(bundle.byObj, objs, bundle.picked);
  if (set === null) return null;
  return {
    list: set.list,
    pickedOrder: set.pickedOrder,
    tone: bundle.tone,
    autoOpen: bundle.autoOpenObj !== undefined && objs.includes(bundle.autoOpenObj),
    post: bundle.post,
  };
}

/** playerOptions hands a seat's identity box its rendered option set — the
 *  same shape a card tile gets, so IdentityBar can reuse CardTile's own
 *  single-action/menu affordance — or null when the decision offers this
 *  seat nothing to be targeted by. */
export function playerOptions(bundle: CardOptions, seat: number): TileOptions | null {
  const set = optionSetFor(bundle.byPlayer, seat, bundle.picked);
  if (set === null) return null;
  return { list: set.list, pickedOrder: set.pickedOrder, tone: bundle.tone, autoOpen: false, post: bundle.post };
}
