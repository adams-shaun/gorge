import type { CardView, PlayerView, View } from '../protocol';
import { manaSymbols, type ManaSymbol } from './mana';

/**
 * castable.ts answers ONE question for the auto-pass logic: after tapping
 * everything this seat could tap, does any card in the seat's own hand become
 * castable? It exists because of the engine's float-then-cast payment model
 * (rules/cast.go's castable gates every hand card on `Cost.payable(pool, ...)`
 * -- the FLOATING pool only): after a land drop the priority window carries
 * nothing but a tap-for-mana "activate" option, so every window classifier
 * that counts only option kinds (autopilot.actionable) reads the window as
 * empty and auto-passes it -- and the Lava Spike the player is holding is
 * never reachable. The wire already carries everything needed to see through
 * that: PlayerView.Pool (floating), PlayerView.Available (what tapping
 * produces right now, a conservative fixed-colour lower bound from
 * rules/mana_available.go) and CardView.ManaCost (Forge notation, parsed by
 * mana.ts).
 *
 * The test here is deliberately a CASTABILITY question, not a full legality
 * question: a card that is affordable but uncastable for a reason the client
 * cannot see (no legal target, a cast restriction) still stops the window.
 * That is the safe direction -- wrongly withholding a pass costs one idle
 * stop; wrongly eating the window the player wanted is the bug this module
 * fixes. The conservative lower bound cuts the other way: a source that can
 * tap for "one U or one R" contributes NOTHING to Available (the engine
 * cannot promise a fixed colour), so a card only that source can pay for does
 * not stop the window -- a miss, never a false stop.
 *
 * Pure functions only: no Svelte, no I/O, no wall clock. The seat's hand is a
 * hidden zone (CR 400.2) the View carries only for the viewer's own seat, so
 * every entry point fails closed (false) on anything it cannot read.
 */

/** A mana unit the seat could spend: a colour face or colourless ("C"). */
type ManaUnit = 'W' | 'U' | 'B' | 'R' | 'G' | 'C';

/**
 * One coloured requirement the resolver backtracks over: the mana faces that
 * can satisfy it (a hybrid pip lists both), whether two life may pay it
 * (Phyrexian), and whether falling back to generic is allowed (a twobrid pip
 * is "two generic OR the colour").
 */
interface Pip {
  faces: ManaUnit[];
  lifeOK: boolean;
  twobrid?: boolean;
}

/** isLand reports whether the card's Types line carries the Land type word. */
function isLand(card: CardView): boolean {
  return (card.types ?? '').split(/\s+/).includes('Land');
}

/**
 * isInstantSpeed reports whether the card could be cast at a moment's notice
 * under the engine's own timing rules (rules/legal.go's instantSpeed test):
 * the Types line carries the Instant type word, or the derived keyword list
 * carries Flash (the exact spelling the engine grants and projects --
 * view/view.go's cardView copies ch.Keywords verbatim, and rules/legal.go
 * tests HasKeyword(id, "Flash"), so the wire word is exactly "Flash").
 *
 * This is the timing half the plain castability question deliberately does
 * NOT answer: castableAfterTap is a CASTABILITY question by doctrine, so a
 * sorcery in hand makes every mana-only window stop-worthy in the player's
 * own turn -- but an auto-pass rule that stops for a SORCERY whenever the
 * opponent does anything would defeat the very auto-pass it serves (the
 * casual opponentSpell 'if-respondable' arm). A response to a resolving
 * spell must be instant-speed; this is the one extra gate respondableAfterTap
 * applies on top of castability.
 */
function isInstantSpeed(card: CardView): boolean {
  if ((card.types ?? '').split(/\s+/).includes('Instant')) return true;
  return (card.keywords ?? []).some((k) => k === 'Flash');
}

/**
 * spendable folds Pool and Available into one map: mana floating NOW plus
 * mana tapping would add. Every Available unit is spendable at most once --
 * the maps are per-colour counts of distinct sources, so plain addition is
 * the whole fold. An absent map (Available is omitted when empty on the
 * wire) contributes nothing.
 */
function spendable(p: PlayerView): Partial<Record<ManaUnit, number>> {
  const out: Partial<Record<ManaUnit, number>> = {};
  const add = (m: Record<string, number> | undefined) => {
    if (!m) return;
    for (const k of Object.keys(m)) {
      if (k === 'W' || k === 'U' || k === 'B' || k === 'R' || k === 'G' || k === 'C') {
        out[k] = (out[k] ?? 0) + m[k];
      }
    }
  };
  add(p.pool);
  add(p.available);
  return out;
}

/**
 * affordable is the cost-side half of the engine's own resolver
 * (rules/mana.go resolveMana), ported to the client's parsed symbols: the
 * coloured pips (including a strict {C}, which generic must not steal) are
 * assigned first, a Phyrexian pip falls back to two life, a twobrid pip falls
 * back to two generic, and the generic requirement is paid last from
 * whatever the pips left -- so coloured mana is never stranded on generic
 * while a pip still needs it. "Whatever the pips left" is the live remainder:
 * the generic test reads the spend map AFTER the pip reservations consumed
 * their units (mirroring rules/mana.go resolveMana, whose base case checks
 * `rem.Total() >= c.Generic` on the mutated pool), not a total saved before
 * them -- a stale total would report {1}{R} affordable from one R because
 * the R consumed for the pip would still pay the generic. Deterministic
 * (fixed pip order, fixed face order, no map iteration reaching the answer)
 * and pure over its inputs.
 *
 * A variable pip (X/Y/Z) is unaffordable BY DEFINITION here: the client
 * cannot know the value the caster would announce, so a card carrying one
 * never makes a window stop-worthy -- the safe direction, since the engine
 * also prices such a cost at whatever X resolves to. A snow ({S}) or unknown
 * pip is likewise refused: {S} needs a snow source the wire does not name and
 * an unknown pip is a parse failure that must not invent affordability.
 */
function affordable(cost: ManaSymbol[], spend: Partial<Record<ManaUnit, number>>, life: number): boolean {
  let generic = 0;
  const pips: Pip[] = [];
  for (const s of cost) {
    switch (s.kind) {
      case 'generic':
        generic += s.value;
        break;
      case 'colour':
        pips.push({ faces: [s.colour], lifeOK: false });
        break;
      case 'colourless':
        pips.push({ faces: ['C'], lifeOK: false });
        break;
      case 'hybrid':
        pips.push({ faces: [s.a, s.b], lifeOK: false });
        break;
      case 'phyrexian':
        pips.push({ faces: [s.colour], lifeOK: true });
        break;
      case 'phyrexianHybrid':
        pips.push({ faces: [s.a, s.b], lifeOK: true });
        break;
      case 'twobrid':
        pips.push({ faces: [s.colour], lifeOK: false, twobrid: true });
        break;
      default:
        return false; // variable (X/Y/Z), snow, unknown: refused
    }
  }
  const rec = (i: number, lifeLeft: number, genericLeft: number): boolean => {
    if (i === pips.length) {
      // The pips already consumed their units from `spend` (the recursion
      // decrements in place and restores on backtrack), so summing it NOW is
      // exactly what the pip assignment left for the generic requirement.
      const remain = Object.values(spend).reduce((n, v) => n + (v ?? 0), 0);
      return remain >= genericLeft;
    }
    const p = pips[i];
    for (const f of p.faces) {
      if ((spend[f] ?? 0) > 0) {
        spend[f] = (spend[f] ?? 0) - 1;
        if (rec(i + 1, lifeLeft, genericLeft)) return true;
        spend[f] = (spend[f] ?? 0) + 1;
      }
    }
    if (p.lifeOK && lifeLeft >= 2 && rec(i + 1, lifeLeft - 2, genericLeft)) return true;
    if (p.twobrid && rec(i + 1, lifeLeft, genericLeft + 2)) return true;
    return false;
  };
  return rec(0, life, generic);
}

/**
 * cardAffordableAfterTap is the shared money test: would this ONE card be
 * affordable once the seat's Available mana joined its Pool. A card with no
 * printed cost does not count (a zero-cost card is already offered by the
 * engine whenever it is castable, so it never needs this window-stop path);
 * everything else goes through affordable(). The land exclusion lives in the
 * two callers below, which each add their own timing question on top.
 */
function cardAffordableAfterTap(p: PlayerView, card: CardView): boolean {
  if (!card.mana_cost) return false;
  const cost = manaSymbols(card.mana_cost);
  if (cost.length === 0) return false;
  return affordable(cost, spendable(p), p.life);
}

/**
 * cardCastableAfterTap reports whether ONE hand card would be affordable once
 * the seat's Available mana joined its Pool. A land never counts (a land drop
 * is the action that CREATES the mana, never a spell that spends it).
 */
export function cardCastableAfterTap(p: PlayerView, card: CardView): boolean {
  if (isLand(card)) return false;
  return cardAffordableAfterTap(p, card);
}

/**
 * cardRespondableAfterTap is the instant-speed variant: it reports whether ONE
 * hand card would both be affordable after tapping AND castable at response
 * speed -- an Instant, or a card carrying the Flash keyword (isInstantSpeed
 * above). This is the predicate an opponent-spell/trigger response rule wants:
 * a sorcery that becomes affordable after tapping must NOT stop a window the
 * player can only watch (casual's if-respondable would otherwise stop for the
 * whole game), while a Mana Leak in hand with two untapped Islands is exactly
 * the answer the player is holding. Same fail-closed shape as the plain
 * variant: a card the cost or timing question cannot answer YES to never
 * stops anything.
 */
export function cardRespondableAfterTap(p: PlayerView, card: CardView): boolean {
  if (isLand(card)) return false;
  if (!isInstantSpeed(card)) return false;
  return cardAffordableAfterTap(p, card);
}

/**
 * castableAfterTap reports whether ANY nonland card in the seat's own hand
 * becomes castable after tapping. Fails closed (false) whenever the hand is
 * not readable: a seat the view does not carry, or another seat's hand (a
 * JSON null on the wire -- the hand is present only for the viewer's own
 * seat), or a malformed view. It does NOT consult whose turn it is -- the
 * stop fires wherever the caller's own stop rules already consult the step.
 */
export function castableAfterTap(view: View, seat: number): boolean {
  const p = view.players?.find((pl) => pl.seat === seat);
  if (!p || !Array.isArray(p.hand)) return false;
  return p.hand.some((c) => cardCastableAfterTap(p, c));
}

/**
 * respondableAfterTap is the instant-speed sibling of castableAfterTap: it
 * reports whether ANY hand card would be castable after tapping AND at
 * response speed (Instant type or Flash keyword -- cardRespondableAfterTap
 * above). It answers the question an opponent-object response rule asks --
 * "could this seat still interact with the spell on the stack if it stopped
 * and tapped out?" -- which the option-kind test alone cannot: the engine
 * prices a cast against the FLOATING pool only, so the window where the
 * opponent's spell sits on the stack carries only tap offers even when the
 * player holds a castable counterspell and the untapped lands to pay for it.
 * The hand scan, fail-closed shape and seat scoping are identical to
 * castableAfterTap's (a hand the view does not carry reads as false).
 */
export function respondableAfterTap(view: View, seat: number): boolean {
  const p = view.players?.find((pl) => pl.seat === seat);
  if (!p || !Array.isArray(p.hand)) return false;
  return p.hand.some((c) => cardRespondableAfterTap(p, c));
}
