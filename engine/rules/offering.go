// offering.go implements Offering (CR 702.48): "[Quality] offering" is an
// optional additional cost -- sacrifice a [quality] permanent -- that reduces
// the spell's total cost by the sacrificed permanent's mana cost and lets the
// spell be cast any time its controller could cast an instant.
//
// It is modelled on Emerge (emerge.go) but is its own cast mode ("offering"),
// offered BESIDE the ordinary cast, which is unchanged:
//
//   - the offer (legal_walk_hand.go) is gated on timing-free instant speed and
//     on some sacrifice candidate leaving a payable reduced cost;
//   - beginCast's castModeOffering arm adds the mandatory Sac part to the
//     printed mana cost (the same cost the ordinary cast would pay);
//   - sacAsk poses the sacrifice before the targets (CR 702.48b, 601.2b), and
//     applyOfferingReduction subtracts the chosen permanent's mana cost
//     SYMBOL BY SYMBOL (CR 702.48c) exactly once, unlike Emerge's
//     mana-value reduction.
//
// The pieces are free functions over *Engine: the Engine method count is
// ratcheted.
package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// kwhOffering is the compiled K:Offering head offeringFace reads.
var kwhOffering = newKWHead("Offering")

// offeringFace reports whether f prints Offering: its offering cast is an
// instant-speed route, so the quiet classifier must treat the face as
// instant-speed (quiet_facts.go).
func offeringFace(f *cards.Face) bool {
	return f != nil && f.KeywordLinesHaveHead(kwhOffering.S, kwhOffering.ID)
}

// offeringQuality reads the printed K:Offering:<Type> quality. A quality that
// is not a bare ASCII type word fails closed (no offering is offered).
func offeringQuality(f *cards.Face) (string, bool) {
	if f == nil {
		return "", false
	}
	q, ok := f.KeywordParam("Offering")
	if !ok {
		return "", false
	}
	if q == "" {
		return "", false
	}
	for i := 0; i < len(q); i++ {
		if c := q[i]; (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return "", false
		}
	}
	return q, true
}

// offeringSacrificePart is the optional additional cost the offering mode
// pays: sacrifice one permanent of the quality (CR 702.48a). It is a CostPart
// so the ordinary Sac machinery (sacAsk, nonManaCastable,
// sacrificeCostCandidates) enforces it.
func offeringSacrificePart(quality string) CostPart {
	return CostPart{N: 1, Spec: "Permanent." + quality + "+YouCtrl"}
}

// offeringCost composes the offering cast's cost from the ordinary base: the
// quality's Sac part is the FIRST Sac part, ahead of any the spell's own
// SpellAbility prints.
func offeringCost(f *cards.Face, base Cost) (Cost, bool) {
	q, ok := offeringQuality(f)
	if !ok {
		return Cost{}, false
	}
	base.Sac = append(base.Sac, offeringSacrificePart(q))
	return pay.WithSpellAbilityExtras(f, base), true
}

// offeringReduce subtracts the sacrificed permanent's printed mana cost from c
// symbol by symbol (CR 702.48c): generic reduces generic; each coloured or
// colourless symbol reduces mana of that kind; a hybrid symbol reduces the
// half the cost has, else counts as excess; any excess reduces generic.
// Nothing goes below zero, and the cost's non-mana parts are untouched.
func offeringReduce(c Cost, sacManaCost string) Cost {
	s := ParseCost(sacManaCost)
	excess := s.Generic
	take := func(slot int) bool {
		if c.Colored[slot] > 0 {
			c.Colored[slot]--
			return true
		}
		return false
	}
	for slot, n := range s.Colored {
		for ; n > 0; n-- {
			if !take(slot) {
				excess++
			}
		}
	}
	either := func(a, b byte) {
		if !take(state.ManaIndex(a)) && !take(state.ManaIndex(b)) {
			excess++
		}
	}
	for _, h := range s.Hybrid {
		either(h.A, h.B)
	}
	for _, h := range s.HybridPhyrexian {
		either(h.A, h.B)
	}
	for _, col := range s.Phyrexian {
		either(col, col)
	}
	for _, tb := range s.Twobrid {
		either(tb.Col, tb.Col)
	}
	for ; s.Snow > 0; s.Snow-- {
		if c.Snow > 0 {
			c.Snow--
		} else {
			excess++
		}
	}
	return reduceGeneric(c, excess)
}

// offeringCandidateCost prices base with oid sacrificed.
func offeringCandidateCost(g *state.Game, base Cost, oid state.ObjID) Cost {
	o := g.Obj(oid)
	if o == nil || o.Face() == nil {
		return base
	}
	return offeringReduce(base, o.Face().ManaCost)
}

// offerOffering offers the "offering" cast (CR 702.48a) at instant speed:
// sacrifice a [quality] permanent to reduce the cost by its mana cost. It is
// offered only while some sacrifice candidate leaves a payable reduced cost
// (the offer gate); the ordinary cast stays offered beside it, gated by its
// own timing. A free function over the walk, not an Engine method.
func offerOffering(w *legalWalk, p state.PlayerID, id state.ObjID, f *cards.Face) {
	e := w.e
	q, ok := offeringQuality(f)
	if !ok || !e.castTargetsAvailable(p, id, f.SpellAbility()) {
		return
	}
	full, _ := offeringCost(f, pay.RawBaseCost(asPayer(e), p, id))
	for _, oid := range pay.SacrificeCostCandidates(asPayer(e), p, id, offeringSacrificePart(q), false) {
		if w.offerCastable(p, id, offeringCandidateCost(e.G, full, oid), spellScope("offering"), false) {
			w.out = append(w.out, decision.Option{Index: len(w.out), Kind: "cast",
				Label: "Cast " + f.Name + " (offering)", Obj: id, Mode: "offering"})
			return
		}
	}
}

// applyOfferingReduction folds the actual chosen sacrifice's mana cost into the
// offering cast's cost exactly once, after every Sac part is settled and before
// the mana window reads pc.cost; idempotent through offeringDone so a resumed
// mana window cannot subtract twice.
func applyOfferingReduction(g *state.Game, pc *pendingCast) {
	if !offeringCast(pc) || pc.offeringDone {
		return
	}
	pc.cost = offeringCandidateCost(g, pc.cost, pc.offeringSac)
	pc.offeringDone = true
}

// offeringCast reports whether pc is an offering cast (its mode is the mark).
func offeringCast(pc *pendingCast) bool { return castModeCodes.Code(pc.mode) == castModeOffering }

// reducesBySac reports whether pc's first Sac part is a reducing sacrifice
// (Emerge or Offering) whose candidates must be priced one by one.
func reducesBySac(pc *pendingCast) bool { return pc.emerge || offeringCast(pc) }

// sacReducedBase is the cost an Emerge or Offering cast composes in
// beginCast: emerge replaces the base with the emerge cost, offering adds the
// quality's Sac part to the ordinary base. ok is false only for Emerge's
// unpriceable shapes; a stale offering (keyword gone) keeps the plain base.
func sacReducedBase(f *cards.Face, base Cost, mode castModeCode) (Cost, bool) {
	if mode == castModeOffering {
		if oc, ok := offeringCost(f, base); ok {
			return oc, true
		}
		return base, true
	}
	return emergeBase(f)
}
