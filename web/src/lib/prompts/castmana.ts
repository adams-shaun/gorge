import type { CardView, Decision, Option, View } from '../../protocol';
import { findCardAnywhere } from '../board';
import { manaAmountText, paymentCostText } from '../announcepay';

/**
 * castmana.ts is the client's shape helpers for the MANUAL cast-payment
 * windows: the mid-cast CR 601.2g window the engine poses while a cast is
 * in progress (rules/cast_paymentplan.go manaWindowAsk — one "activate"
 * option per untapped mana source, the source's beyond-tap cost carried on
 * Option.cost, then "done"), and the sacrifice election one of those
 * activations can hand back (rules/pay/manacostwalk.go's
 * ManaCostChoiceStages — every option a "sacrifice" candidate with an Obj).
 *
 * The engine's decision sequence is unchanged and every answer stays one of
 * the options the decision offered, posted by its own wire index: this
 * module only decides which panel draws the ask and what its rows read.
 *
 * Detection is by DECISION SHAPE, never by prompt text. The one structural
 * signal separating the cast window from the sibling windows that pose the
 * very same activate+done shape (rules/cumulative.go paymentManaAskClass's
 * cumulative-upkeep / echo / triggered-cost windows, ward.go's ward window,
 * pay/unlessflow.go's unless window) is where the decision's Source sits:
 * a cast in progress has its spell on the STACK, every sibling payment's
 * source is a battlefield object. That check needs the view, so
 * isCastManaWindow takes one (the announced select-mana window needs none —
 * its `mana_payment` readout is its own structural signal).
 *
 * The sacrifice election's shape (all "sacrifice" options with an Obj,
 * min===max) is shared with unrelated engine asks (edicts, sub-counter and
 * morph costs), so it is tied to a mana-source activation too: its Source
 * is a mana-producing battlefield permanent while the payer has a spell
 * on the stack (see isManaSacrificeAsk).
 */

/**
 * isCastManaWindow reports whether d is the mid-cast mana window: a
 * single-pick `choose` with no `mana_payment`, whose options are exactly
 * one "activate" per source (each with its object) plus one "done", and
 * whose source resolves to an entry on the view's stack — the spell being
 * paid for. view is required: without it the sibling windows above are
 * indistinguishable, and the helper answers false (the generic list keeps
 * them, exactly as today).
 */
export function isCastManaWindow(d: Decision | null, view: View | null): boolean {
  if (d === null || d.kind !== 'choose' || d.mana_payment != null) return false;
  if (d.min !== 1 || d.max !== 1) return false;
  if (d.options.length < 2) return false;
  let done = 0;
  for (const o of d.options) {
    if (o.kind === 'done') done++;
    else if (o.kind !== 'activate' || o.obj === undefined) return false;
  }
  if (done !== 1) return false;
  if (view === null || d.source === undefined || d.source === 0) return false;
  return view.stack.some((s) => s.id === d.source || s.card?.id === d.source);
}

/**
 * isManaSacrificeAsk reports whether d is a mana ability's sacrifice
 * election during a cast: a `choose` whose every option is a "sacrifice"
 * candidate with its object, asked of an exact number (min === max, the
 * walk's election shape), AND whose Source is the mana source being
 * activated — a battlefield permanent that produces mana (CardView.produces)
 * — while the asking seat has a spell on the stack. Every other engine
 * sacrifice election (an edict or "sacrifice N" resolving, a sub-counter or
 * morph-turn-up cost) poses the same option shape, but its Source is the
 * resolving spell or ability on the stack or a non-mana permanent, so it
 * stays on the generic list. view is required: without it the election is
 * indistinguishable and the helper answers false.
 */
export function isManaSacrificeAsk(d: Decision | null, view: View | null): boolean {
  if (d === null || view === null || d.kind !== 'choose' || d.min !== d.max || d.options.length === 0) return false;
  if (!d.options.every((o) => o.kind === 'sacrifice' && o.obj !== undefined)) return false;
  if (d.source === undefined || d.source === 0) return false;
  const src = view.players.flatMap((p) => p.battlefield).find((c) => c.id === d.source);
  if (src === undefined || !src.produces) return false;
  return view.stack.some((s) => s.kind === 'spell' && s.controller === d.player);
}

/** manaWindowDone finds the window's single "done" option, or null. */
export function manaWindowDone(d: Decision): Option | null {
  return d.options.find((o) => o.kind === 'done') ?? null;
}

export interface CastManaRow {
  obj: number;
  name: string;
  options: Option[];
  /** mana is the source's production pips ("C C"), null when the view does not show the card. */
  mana: string | null;
  /** any marks an "add any colour" source (a Phyrexian Altar). */
  any: boolean;
}

/**
 * castManaRows groups the window's "activate" options by source, in the
 * order the engine offered them (never re-sorted: the offer order is the
 * board order the engine walks, and the digit hotkeys read the same order
 * off the decision's generic option list). A row's mana is the card's own
 * projected production (CardView.produces, a card fact every seat sees),
 * not a re-derivation of what the activation will add.
 */
export function castManaRows(d: Decision, view: View | null): CastManaRow[] {
  const rows: CastManaRow[] = [];
  const at = new Map<number, CastManaRow>();
  for (const o of d.options) {
    if (o.kind !== 'activate' || o.obj === undefined) continue;
    let row = at.get(o.obj);
    if (row === undefined) {
      const card = view ? findCardAnywhere(view, o.obj) : null;
      row = {
        obj: o.obj,
        name: card?.name ?? `Source ${o.obj}`,
        options: [],
        mana: card?.produces ? manaAmountText(card.produces.colour) : null,
        any: card?.produces?.any ?? false,
      };
      at.set(o.obj, row);
      rows.push(row);
    }
    row.options.push(o);
  }
  return rows;
}

const COST_KINDS: Record<string, string> = { Sac: 'Sacrifice', Discard: 'Discard', Draw: 'Draw', tapXType: 'Tap' };

/**
 * activationCostText reads a beyond-tap activation's Option.cost marker
 * (rules/cost/format.go FormatCost: "T Sac<1/Lion's Eye Diamond>",
 * "T PayLife<1>") as payment words. Presentation only — the wire marker is
 * unchanged and the engine validates the answer, never this text; a token
 * this reader does not know is shown as itself. A cost part's spec may
 * carry spaces ("Lion's Eye Diamond"), so this is one pass of anchored
 * replacements over the whole string, not a token split.
 */
export function activationCostText(marker: string): string {
  if (marker === '') return '';
  let t = marker
    .replace(/(^|\s)T(?=\s|$)/g, '$1Tap')
    .replace(/(^|\s)Q(?=\s|$)/g, '$1Untap')
    .replace(/PayLife<(\d+|X)>/g, 'Pay $1 life')
    .replace(/PayEnergy<(\d+|X)>/g, 'Pay $1 energy');
  t = t.replace(/([A-Za-z]+)<(\d+|X)\/([^>]*)>/g, (raw, kind: string, n: string, spec: string) => {
    const word = COST_KINDS[kind];
    return word === undefined ? raw : `${word} ${n} ${spec}`;
  });
  return t;
}

export interface CastManaReadout {
  /** card is the spell being paid for, null when the view does not show it. */
  card: CardView | null;
  /** cost is the spell's offer-time cost in Forge notation, null when unknown. */
  cost: string | null;
  /** pool is the payer's floating mana as pips ("" when none). */
  pool: string;
  /** owed is a display-only estimate: the cost minus the floating pool, pip
   *  by pip. The engine's own pricing (cost reduction statics, a PayLife
   *  grant, an unfixed {X}) is the authority; the cell only reads the two
   *  public facts the wire already carries. */
  owed: string | null;
}

const SLOTS = ['W', 'U', 'B', 'R', 'G', 'C'] as const;

/**
 * castManaReadout assembles the window's cost readout from the two public
 * facts on the wire: the stack spell's printed/offer-time cost and the
 * payer's floating pool. There is deliberately no per-activation ledger on
 * this wire (unlike the announced window's `mana_payment`), so "still owed"
 * is an estimate and is labelled as such in the panel.
 */
export function castManaReadout(d: Decision, view: View | null): CastManaReadout {
  const card = view && d.source !== undefined ? findCardAnywhere(view, d.source) : null;
  const cost = card?.effective_mana_cost || card?.mana_cost || null;
  const poolMap = view?.players.find((p) => p.seat === d.player)?.pool ?? {};
  const poolCounts = SLOTS.map((s) => poolMap[s] ?? 0);
  const generic = Number.parseInt(cost ?? '', 10);
  let owed: string | null = null;
  if (cost !== null) {
    const pips: number[] = [0, 0, 0, 0, 0, 0];
    let genericOwed = Number.isNaN(generic) ? 0 : generic;
    for (const tok of cost.split(' ')) {
      const at = SLOTS.indexOf(tok as (typeof SLOTS)[number]);
      if (at >= 0) pips[at]++;
    }
    let leftover = 0;
    for (let i = 0; i < 6; i++) {
      const paid = Math.min(pips[i], poolCounts[i]);
      pips[i] -= paid;
      leftover += poolCounts[i] - paid;
    }
    // Floating mana of any kind can settle a generic requirement (CR
    // 107.3b), so every pool pip not spent on a coloured one counts.
    genericOwed = Math.max(0, genericOwed - leftover);
    owed = paymentCostText({ generic: genericOwed, mana: pips as PaymentCostMana });
  }
  return {
    card,
    cost,
    pool: manaAmountText(poolCounts),
    owed,
  };
}

type PaymentCostMana = [number, number, number, number, number, number];
