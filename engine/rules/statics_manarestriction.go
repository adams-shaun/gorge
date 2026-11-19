package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// The S:Mode$ ManaRestriction static's implemented shape (the stat:ManaRestriction
// label cards.ManaRestrictionZeroSpendShape certifies): Hogaak, Arisen
// Necropolis's "You can't spend mana to cast this spell" -- ValidSource$
// Card,Emblem | InvertValidSource$ True | EffectZone$ All.
//
// The reader is manaSpendCastBan, and the enforcement is a payment-level
// one: the pending cast records it (newCast's pc.noMana), every descriptor
// paymentForCast builds for the cast carries pay.Descriptor.NoMana, and
// pay.AvailableFor answers the empty pool for such a payment. Convoke's
// creature taps and Delve's graveyard exiles are not mana, so they alone pay
// the cost; the offer gate (offerCastable, which prices feasibility over the
// credited cost with the empty pool) therefore offers the cast only when its
// credits can cover the whole thing, and no mana can ever be charged for it.
// The announcement gates keep the credits honest:
// validateCastContributions' manaSpendConvokeCovers (the one legal-answer rule
// the lockstep host reads through CastContributionsAdmitted) requires the
// announced Convoke to leave no more than the Delve potential, announceFeasible
// folds the announced contributions before pricing a pip face, and delveAsk
// defers past the pip announcements and forces the shortfall as exiles. The
// mode's source-restricted shapes (Myr Superion, Imperiosaur) need per-source
// pool provenance and stay registered as stat:ManaRestriction.OtherShape.
func init() {
	effects.RegisterNonAPI("stat:ManaRestriction")
}

// manaSpendCastBan reports whether casting the spell id is forbidden from
// spending mana: the card's own face carries a live stat:ManaRestriction
// static of the implemented zero-spend shape (cards.ManaRestrictionZeroSpendShape,
// "You can't spend mana to cast this spell" -- Hogaak). The static is read
// from the cast card's face, not activeStatics: activeStatics walks the
// battlefield, while the restriction names the cast of its own source and the
// source sits in hand, graveyard or the command zone while it is being cast
// (its EffectZone$ All applies from every zone). A spell-only read: an
// activated ability of the same card is not a cast.
func manaSpendCastBan(g *state.Game, id state.ObjID) bool {
	o := g.Obj(id)
	if o == nil {
		return false
	}
	f := o.Face()
	if f == nil {
		return false
	}
	for i := range f.Statics {
		if cards.ManaRestrictionZeroSpendShape(f.Statics[i]) {
			return true
		}
	}
	return false
}

// manaSpendBoard is the narrow surface manaSpendConvokeCovers reads: the
// composed CR 601.2f total (manaToPay), the Delve keyword read and the game.
type manaSpendBoard interface {
	Game() *state.Game
	manaToPay(pc *pendingCast) Cost
	hasCastDelve(id state.ObjID, ability bool) bool
}

// manaSpendOfferBoard is the narrow surface manaSpendOfferCovers reads.
type manaSpendOfferBoard interface {
	Game() *state.Game
	IsCreature(id state.ObjID) bool
	objColors(o *state.Object) string
	hasCastConvoke(id state.ObjID) bool
}

// manaSpendOfferCovers is the zero-spend shape's OFFER-side credit question:
// can the cast's whole cost be paid by its Convoke taps and its Delve
// potential alone, with no mana at any point? The coloured requirement is
// matched against the untapped creatures (the announcement's taps) by
// maximum bipartite matching -- a hybrid pip accepts either of its colours
// (CR 702.51a: the creature pays "one mana of that creature's colour") and a
// plain pip its own colour, while a twobrid pip counts only at its
// two-generic face -- and every unmatched creature then gives one generic;
// the remaining generic is the Delve potential's to cover, and any leftover
// refuses. Components
// the credit cannot price (an unfixed {X}, a Phyrexian or hybrid-Phyrexian
// pip, snow mana) refuse outright, the same fail-closed arm the offer gate
// withholds on. A waterbend tap credit is not modelled (no corpus carrier),
// so such a cost under-credits and stays withheld -- the conservative
// direction, never an offer the payment would refuse.
func manaSpendOfferCovers(e manaSpendOfferBoard, p state.PlayerID, id state.ObjID, c Cost, delve int32) bool {
	if c.X > 0 || c.XMin > 0 || c.Snow > 0 || len(c.Phyrexian) > 0 || len(c.HybridPhyrexian) > 0 {
		return false
	}
	g := e.Game()
	type slot struct {
		colors  string
		generic int32 // >0 marks a twobrid pip's two-generic face
	}
	slots := make([]slot, 0, int(c.Colored.Total())+len(c.Hybrid)+len(c.Twobrid))
	for _, letter := range pipLetters {
		for n := int32(0); n < c.Colored[state.ManaIndex(letter)]; n++ {
			slots = append(slots, slot{colors: string(letter)})
		}
	}
	for _, hp := range c.Hybrid {
		slots = append(slots, slot{colors: string(hp.A) + string(hp.B)})
	}
	// A twobrid pip stays at its two-generic face. Its colour face is NOT
	// offerable: convokeAsk's zero-spend colour options enumerate the
	// unresolved HYBRID pips only (cast_payment.go's convokeAsk), so no
	// colour contribution can ever be announced against a twobrid pip, and
	// crediting the colour face here would promise a payment the
	// announcement cannot make -- the offer would open a cast whose
	// announcement is refused. delveAsk reserves the pip at the same
	// two-generic face, so the Delve potential alone must cover it. No corpus
	// carrier pairs the zero-spend shape with a twobrid pip, so this is the
	// conservative reading until one exists.
	for range c.Twobrid {
		slots = append(slots, slot{generic: 2})
	}
	// The candidates are the untapped creatures the convoke announcement can
	// tap -- the same eligibility convokeCountCredit prices with (CR
	// 702.51a; a bestowed or reconfigured attached permanent is not a
	// separate permanent to tap). A card without Convoke credits nothing:
	// its whole cost must come from the Delve potential alone.
	var cands []string
	if e.hasCastConvoke(id) {
		for _, cid := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(cid)
			if o == nil || o.Tapped || o.PhasedOut || o.Face() == nil || o.BestowedAttached() || o.ReconfiguredAttached() {
				continue
			}
			if !e.IsCreature(cid) {
				continue
			}
			cands = append(cands, e.objColors(o))
		}
	}
	assign := make([]int, len(cands))
	for i := range assign {
		assign[i] = -1
	}
	var match func(si int, seen []bool) bool
	match = func(si int, seen []bool) bool {
		for ci := range cands {
			if seen[ci] || assign[ci] >= 0 {
				continue
			}
			ok := false
			for _, r := range slots[si].colors {
				for _, cr := range cands[ci] {
					if cr == r {
						ok = true
					}
				}
			}
			if !ok {
				continue
			}
			seen[ci] = true
			if assign[ci] == -1 || match(assign[ci], seen) {
				assign[ci] = si
				return true
			}
		}
		return false
	}
	genericNeed := c.Generic
	for si := range slots {
		seen := make([]bool, len(cands))
		if match(si, seen) {
			continue
		}
		// An unmatched coloured unit or hybrid pip has no payment left (the
		// pool is empty); an unmatched twobrid pip falls to its two-generic
		// face.
		if slots[si].generic <= 0 {
			return false
		}
		genericNeed += slots[si].generic
	}
	free := int32(0)
	for ci := range cands {
		if assign[ci] == -1 {
			free++
		}
	}
	if genericNeed > free {
		genericNeed -= free
	} else {
		genericNeed = 0
	}
	return genericNeed <= int32(delve)
}

// hybridPipCarries reports whether any unresolved hybrid pip of m carries
// colour c ({B/G} is carried by B and by G).
func hybridPipCarries(m Cost, c byte) bool {
	for _, p := range m.Hybrid {
		if p.A == c || p.B == c {
			return true
		}
	}
	return false
}

// manaSpendConvokeCovers reports whether the cast's announced Convoke-family
// payments, folded into the composed total manaToPay, leave a remainder the
// Delve potential alone can pay -- the zero-spend shape's Submit-time rule for
// the Convoke announcement (CR 702.51a: a creature is tapped for a reduction
// the total cost still needs). The fold is applyConvoke's arithmetic over
// exactly the payments validateCastContributions is judging, plus its
// unresolved-pip leniency: a colour contribution covers one unresolved hybrid
// pip of a colour it carries (each pip absorbs one contribution), so a pip
// left uncovered refuses -- its only payment would be mana. The remaining
// conditions: no coloured requirement may survive (Delve covers generic
// only), and the folded generic plus every still-unresolved twobrid pip at
// its two-generic face must not exceed the exiles the Delve ask can still
// announce. A cost component this rule cannot price (Phyrexian,
// HybridPhyrexian, snow, an unfixed X) refuses -- the fail-closed direction;
// the offer gate withholds such a cast outright (offerCastableUsingExtra's
// NoMana arm), so no seat is ever left stranded with an announcement every
// answer of which is refused.
func manaSpendConvokeCovers(e manaSpendBoard, pc *pendingCast, pays []convokePayment) bool {
	m := e.manaToPay(pc)
	if m.Snow > 0 || len(m.Phyrexian) > 0 || len(m.HybridPhyrexian) > 0 || m.X > 0 || m.XMin > 0 {
		return false
	}
	pips := int32(len(m.Hybrid))
	for _, pay := range pays {
		if pay.Color != 0 {
			i := state.ManaIndex(pay.Color)
			if m.Colored[i] > 0 {
				m.Colored[i]--
				continue
			}
			if pips <= 0 || !hybridPipCarries(m, pay.Color) {
				return false
			}
			pips--
			continue
		}
		reduce := pay.Power
		if reduce <= 0 {
			reduce = 1
		}
		m.Generic -= reduce
		if m.Generic < 0 {
			m.Generic = 0
		}
	}
	if m.Colored.Total() > 0 || pips > 0 {
		return false
	}
	need := m.Generic + 2*int32(len(m.Twobrid))
	gy := 0
	if !pc.isAbility() && e.hasCastDelve(pc.card, false) {
		gy = len(e.Game().Zone(state.ZGraveyard, pc.player))
	}
	return need <= int32(gy)
}

// manaSpendAnnouncementShape is the zero-spend shape's Convoke announcement
// size for the composed total m (manaToPay, before the ask's hybrid face
// expansion): the coloured taps it needs (one per coloured unit and per
// unresolved hybrid pip) and the fewest taps in all -- those plus the generic
// the Delve potential cannot cover. It is manaSpendConvokeCovers's coverage
// rule read the other way round: an announcement of fewer taps, or with more
// coloured ones than slots, is exactly one that rule refuses.
func manaSpendAnnouncementShape(e manaSpendBoard, pc *pendingCast, m Cost) (colour, taps int) {
	colour = int(m.Colored.Total()) + len(m.Hybrid)
	generic := m.Generic + 2*int32(len(m.Twobrid))
	gy := int32(0)
	if !pc.isAbility() && e.hasCastDelve(pc.card, false) {
		gy = int32(len(e.Game().Zone(state.ZGraveyard, pc.player)))
	}
	return colour, colour + int(max(generic-gy, 0))
}
