// offering.go implements the Offering alternative cast (CR 702.46): "Offering
// [Type] — You may cast this card any time you could cast an instant by
// sacrificing a [type] permanent. If you do, this spell's mana cost is reduced
// by the sacrificed permanent's mana cost (including color)." The corpus
// spells the same rule with the pre-2007 wording ("as though it had flash by
// sacrificing an artifact and paying the difference in mana costs between this
// and the sacrificed artifact. Mana cost includes color." — Blast-Furnace
// Hellkite).
//
// Offering is a CASTING OPTION, read directly off the K: line exactly like the
// Flash/Flashback/Bestow/Mutate/Emerge family (engine/cards/keywords.go's
// expandKeywords doc names it), so it is registered here in its own file as
// emerge.go registers kw:Emerge. It is NOT a plain alternative-cost
// substitution: the cast pays the PRINTED mana cost REDUCED by the chosen
// permanent's mana cost AND sacrifices that permanent, so it composes exactly
// like Emerge (engine/rules/emerge.go) — with the one difference that the
// reduction is colour-inclusive (each of the sacrificed cost's pips takes a
// matching coloured pip of the spell's cost first, the shortfall spills to
// generic, floored at zero — pay.CostMods.ApplyTo's one reduction body), not
// the mana-value-only reduceGeneric Emerge uses. The pieces:
//
//   - offeringSacrificeSpec resolves the printed K:Offering parameter to the
//     mandatory Sacrifice CostPart; a parameter the ordinary Sac machinery
//     cannot model (empty, colon-metadata-only, a spaced spec) withholds the
//     offer rather than guessing a spec.
//   - offeringCandidateCost applies ONLY the chosen permanent's mana cost as
//     the reduction. A permanent whose printed cost ParseCost cannot model
//     (an {X}, a hybrid pip, an unknown token) is skipped PER CANDIDATE, never
//     priced as a degraded generic.
//   - offeringOfferCost checks each candidate against the caller's actual (or
//     hypothetical) pool, exactly as emergeOfferCost does: a best-case price
//     alone must not authorize an offering whose own payment is unaffordable.
//     The payment ask repeats this candidate-specific constraint against the
//     live pool (offeringSacPayable).
//   - the offer (legal.go's hand walk) adds the "offering" cast mode at BOTH
//     timings — the permission ("any time you could cast an instant", CR
//     117.1a) is satisfied by every window a sorcery can be cast in too, and
//     the reduced price is CHEAPER than the plain cast there, so unlike
//     mayflash the sorcery-timing branch is not a redundant duplicate — and
//     beginCast's "offering" arm composes the printed cost with the mandatory
//     Sac part and marks the cast.
//   - sacAsk applies the chosen permanent's mana cost as the reduction exactly
//     once, after every Sac part is settled, before the mana window reads
//     pc.cost (applyOfferingReduction, the applyEmergeReduction shape).

package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	// The coverage census: kw:Offering is implemented as a casting option read
	// directly off the K: line (see this file's doc), registered here exactly
	// as emerge.go registers kw:Emerge.
	effects.RegisterNonAPI("kw:Offering")
}

// offeringSacrificeSpec resolves the printed K:Offering parameter to the
// mandatory additional cost: sacrifice one permanent of the named type
// (Blast-Furnace Hellkite's "Artifact", Patron of the Nezumi's "Rat"). A colon
// suffix (Forge's metadata tail, the bestow/mutate shape) is cut before the
// spec is read. A parameter the spec filter cannot be read from — empty after
// the cut, or carrying spaces (the filter grammar's own separators) — is
// withheld: pricing a guessed spec would either block a legal sacrifice or
// admit one the printed keyword does not name.
func offeringSacrificeSpec(f *cards.Face) (CostPart, bool) {
	if f == nil {
		return CostPart{}, false
	}
	s, ok := f.KeywordParam("Offering")
	if !ok {
		return CostPart{}, false
	}
	if i := strings.IndexByte(s, ':'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " \t") {
		return CostPart{}, false
	}
	return CostPart{N: 1, Spec: s}, true
}

// offeringPrintedCost vets the face's PRINTED mana cost for the shapes this
// build prices: only generic and WUBRGC pips. Anything else (an {X}, a hybrid,
// an unknown token) withholds the whole offer rather than subtracting a
// reduction from a cost whose true price is not known — the mayflashExtraCost
// withhold convention.
func offeringPrintedCost(f *cards.Face) (Cost, bool) {
	if f == nil {
		return Cost{}, false
	}
	c := ParseCost(f.ManaCost)
	if len(c.Unknown) > 0 || c.X > 0 || c.Life != 0 || c.Snow != 0 || c.Tap ||
		len(c.Hybrid) != 0 || len(c.Phyrexian) != 0 || len(c.Twobrid) != 0 || len(c.HybridPhyrexian) != 0 ||
		len(c.Sac) != 0 || len(c.Discard) != 0 || len(c.SubCounter) != 0 || len(c.AddCounter) != 0 ||
		len(c.Exile) != 0 || len(c.Reveal) != 0 || len(c.RevealChosen) != 0 || len(c.Behold) != 0 ||
		len(c.TapPermanent) != 0 || len(c.Blight) != 0 || c.Forage || len(c.Draw) != 0 ||
		len(c.Energy) != 0 || len(c.LifeX) != 0 || c.LifeHalfUp || len(c.DamageYou) != 0 ||
		len(c.GainLife) != 0 ||
		len(c.Return) != 0 || len(c.PutToLib) != 0 || len(c.MoveToGrave) != 0 || len(c.Mill) != 0 ||
		len(c.Evidence) != 0 || len(c.RollDice) != 0 {
		return Cost{}, false
	}
	return c, true
}

// offeringCandidateCost applies ONLY the chosen permanent's mana cost as the
// reduction, colour-inclusive: each of its pips takes a matching coloured pip
// of the cost first (CR 702.46's "including color"), the shortfall spills to
// the generic part, floored at zero. The composition is pay.CostMods.ApplyTo's
// one reduction body — the same spill semantics every cost-reduction static
// uses — so offering and statics cannot drift about what a coloured reduction
// does. A permanent whose printed cost this build cannot price (an {X}, a
// hybrid, an unknown token; tokens with no mana cost are NOT unpriceable —
// they reduce by nothing and are legal sacrifices) is skipped: ok=false tells
// the caller to drop the candidate, never to price a degraded generic.
func (e *Engine) offeringCandidateCost(base Cost, oid state.ObjID) (Cost, bool) {
	o := e.G.Obj(oid)
	if o == nil || o.Face() == nil {
		return base, false
	}
	sc := ParseCost(o.Face().ManaCost)
	if len(sc.Unknown) > 0 || sc.X > 0 || sc.Life != 0 || sc.Snow != 0 || sc.Tap ||
		len(sc.Hybrid) != 0 || len(sc.Phyrexian) != 0 || len(sc.Twobrid) != 0 || len(sc.HybridPhyrexian) != 0 ||
		sc.LifeHalfUp || sc.Forage {
		return base, false
	}
	var m pay.CostMods
	m.Reduces = []pay.CostMod{{Generic: sc.Generic, Colored: sc.Colored,
		HasColor: sc.Colored.Total() > 0}}
	m.ApplyTo(&base)
	return base, true
}

// offeringOfferCost checks each candidate against the caller's actual (or
// hypothetical) pool, the emergeOfferCost shape: among the candidates whose
// own reduced payment the caller accepts, the one whose reduced price is
// CHEAPEST (the largest sacrifice) is what the offer is gated on. A
// best-case price alone must not authorize an offering whose own payment is
// unaffordable; the payment ask repeats this candidate-specific constraint
// against the live pool.
func (e *Engine) offeringOfferCost(p state.PlayerID, id state.ObjID, f *cards.Face, payable func(Cost) bool) (Cost, bool) {
	spec, ok := offeringSacrificeSpec(f)
	if !ok {
		return Cost{}, false
	}
	if _, ok := offeringPrintedCost(f); !ok {
		return Cost{}, false
	}
	// The printed mana cost is the base — never a substitution (CR 702.46
	// reduces it; it does not replace it) — plus the mandatory sacrifice and
	// the spell's own additional costs (CR 118.9: an alternative arrangement
	// replaces the mana cost only). The base is read through castOfferBase,
	// the ONE printed-cost reader the plain and may-flash offers use, so the
	// three cannot drift about what a printed cost prices; beginCast's
	// "offering" arm composes the same cost (its switch entry is
	// pay.RawBaseCost) plus this Sac part.
	base := e.castOfferBase(p, id)
	base.Sac = append(base.Sac, spec)
	base = pay.WithSpellAbilityExtras(f, base)
	best := int32(-1)
	var priced Cost
	for _, oid := range pay.SacrificeCostCandidates(asPayer(e), p, id, spec, false) {
		candidate, ok := e.offeringCandidateCost(base, oid)
		if !ok || !payable(candidate) {
			continue
		}
		if total := candidate.Generic + candidate.Colored.Total(); best < 0 || total < best {
			best, priced = total, candidate
		}
	}
	return priced, best >= 0
}

// At the sacrifice ask use the same reduction and pool feasibility as the
// offer, but the pending cast's captured modifiers (the card is on the stack
// now). Only mana varies by candidate; the common non-mana parts were checked
// at the offer and are settled independently by sacAsk. A candidate this
// build cannot price is not offered as a sacrifice.
func (e *Engine) offeringSacPayable(pc *pendingCast, oid state.ObjID) bool {
	candidate, ok := e.offeringCandidateCost(pc.cost, oid)
	if !ok {
		return false
	}
	delve := int32(0)
	if e.hasCastDelve(pc.card, pc.isAbility()) {
		delve = int32(len(e.G.Zone(state.ZGraveyard, pc.player)))
	}
	return e.manaFeasiblePriced(pc.player, paymentFor(pc.card, false, Cost{}), candidate, pc.mods, pc.taxGeneric, delve, nil)
}

// applyOfferingReduction folds the actual chosen sacrifice's mana cost into
// the offering cast's cost exactly once. Called from sacAsk after every Sac
// part has been settled and before the mana window reads pc.cost, so the
// reduction is part of the cost the payment charges while still composing
// under the CR 601.2f modifiers manaToPay applies afterwards. Idempotent
// through offeringDone so the continueCast re-entries a suspended mana window
// makes cannot subtract twice. An unpriceable chosen sacrifice folds nothing
// (it cannot reach here: sacAsk only offers candidates
// offeringSacPayable admitted).
func (e *Engine) applyOfferingReduction(pc *pendingCast) {
	if !pc.offering || pc.offeringDone {
		return
	}
	if candidate, ok := e.offeringCandidateCost(pc.cost, pc.offeringSac); ok {
		pc.cost = candidate
	}
	pc.offeringDone = true
}

// offeringTimingOK is the timing gate the hand walk relaxes to: the keyword
// must be present and the face's own activation-phase restrictions
// (ActivationPhases$, PlayerTurn$, ...) must hold, exactly the
// mayflashTimingOK shape — the instant/sorcery-speed check spellTimingOK
// makes is deliberately omitted, because the permission ("any time you could
// cast an instant", CR 702.46a) IS the flash window, and every sorcery window
// satisfies it as well (CR 117.1a: an instant can be cast whenever a sorcery
// can).
func (e *Engine) offeringTimingOK(p state.PlayerID, f *cards.Face) bool {
	if f == nil {
		return false
	}
	if _, ok := f.KeywordParam("Offering"); !ok {
		return false
	}
	return e.activationPhasesOK(p, f.SpellAbility())
}

// offeringCastCost composes the Offering cast's cost (CR 702.46): the PRINTED
// mana cost (never a substitution -- the keyword reduces it) plus the
// mandatory sacrifice of a permanent of the named type, whose mana cost
// reduces the total. The reduction is NOT applied here -- the permanent is
// not chosen until sacAsk settles the Sac part -- so this marks nothing
// itself; applyOfferingReduction folds the chosen permanent's mana cost out
// of the cast's cost once the choice is in. cost is the same pay.RawBaseCost
// the offer gate priced, so the two stages cannot drift; the spell's own
// additional costs fold here as the emerged arm's emergeBase folds them. A
// stale option whose keyword is gone falls back to the empty cost.
func offeringCastCost(f *cards.Face, cost Cost) Cost {
	spec, ok := offeringSacrificeSpec(f)
	if !ok {
		return Cost{}
	}
	cost.Sac = append(cost.Sac, spec)
	return pay.WithSpellAbilityExtras(f, cost)
}

// offerOffering appends the "offering" cast option for a hand face (CR 702.46)
// when the best sacrifice candidate is payable and targetsOK reports the
// plain SpellAbility has targets. Called from handWalk on both timing paths:
//
// At instant timing, when the ordinary timing gate fails, a face printed with
// the keyword may still be cast at instant speed ("any time you could cast an
// instant") by sacrificing a permanent of the named type; the cast's price is
// the printed mana cost reduced by that permanent's mana cost
// (colour-inclusive), which is CHEAPER than the plain cast. Unlike mayflash
// there is no plain duplicate to suppress, because the plain cast is not
// legal there at all.
//
// At sorcery timing the keyword's permission already covers every sorcery
// window, so the offer is added there as well -- it is NOT a redundant
// duplicate of the plain cast, because the sacrifice makes the offering price
// CHEAPER than the plain one.
//
// offeringOfferCost composes the printed mana cost with the mandatory Sac
// part and prices each sacrifice candidate's colour-inclusive reduction;
// sacAsk permits only candidates whose own reduced cost remains payable.
// Unsupported printed cost shapes are withheld; the plain cast is unaffected.
// targetsOK is lazy so the census runs only once the price check passes.
//
// ph is the face's keyword census (nothing to offer without phOffering);
// instant selects the failed-timing-gate path, which also requires the
// keyword's own timing check (offeringTimingOK).
func (w *legalWalk) offerOffering(ph printedHeads, p state.PlayerID, id state.ObjID, f *cards.Face, instant bool, targetsOK func() bool, out *[]decision.Option) {
	if !ph.has(phOffering) || instant && !w.e.offeringTimingOK(p, f) {
		return
	}
	if _, ok := w.e.offeringOfferCost(p, id, f, func(c Cost) bool {
		return w.offerCastable(p, id, c, spellScope("offering"), false)
	}); ok && targetsOK() {
		*out = append(*out, decision.Option{Index: len(*out), Kind: "cast",
			Label: "Cast " + f.Name + " (offering)", Obj: id, Mode: "offering"})
	}
}

// sacReducesCost reports whether the cast's Sac part reduces the total cost
// (Emerge CR 702.118a, Offering CR 702.46), so sacAsk announces it before the
// targets (CR 601.2f).
func (pc *pendingCast) sacReducesCost() bool { return pc.emerge || pc.offering }

// recordCostReducingSac remembers the sacrifice an Emerge or Offering cast's
// "sacrifice" answer chose, before the Sac part settles (SacPart == 0), for
// the reduction folded out of the cast's cost.
func (pc *pendingCast) recordCostReducingSac(chosen []decision.Option) {
	if pc.SacPart != 0 || len(chosen) != 1 {
		return
	}
	if pc.emerge {
		pc.emergeSac = chosen[0].Obj
	}
	if pc.offering {
		pc.offeringSac = chosen[0].Obj
	}
}
