package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Offering (CR 702.48, K:Offering:<Type>), cold: reached only when the
// table has a card with the keyword (genPrep.anyOffering). Copied from gorge
// (rules offering.go, legal_walk_hand.go, cast_subcounter.go sacAsk):
//
//   - the hand offers the "offering" cast ABOVE the sorcery-speed timing
//     gate, beside the plain cast, while some permanent of the quality its
//     caster controls leaves a payable cost once its mana cost is subtracted
//     (offerOffering). The cast rides AkMayFlash (a card never has both:
//     kgen refuses the pair) and its provenance trigPlMayFlash; the
//     card's HasOffering picks the route.
//   - the sacrifice is posed through the casualty election stage (build.cas
//     = casElected, bsCasualty), before the targets, as gorge's sacAsk poses
//     a reducing sacrifice: one DkChoose/ChCostSacrifice (gorge's
//     "sacrifice" KChoose, Min=Max=1, asked even with one candidate) over the
//     candidates whose own reduced cost is payable (emergeSacPayable). The
//     pick is a pkSac pick, settled at payment with the other costs.
//   - buildCost subtracts the picked permanent's mana cost symbol by symbol
//     (CR 702.48c, gorge offeringReduce) before the cost statics (CR 601.2f).
//   - a paid offering queues nothing (casualtyTrigger).
//
// gorge's planner prices no "offering" cast (potential_plan.go has no arm),
// so under a gorge surface the option exists only when the floating pool
// pays, the may-flash cast's rule (kw_mayflash.go), and the sacrifice ask
// lists only the candidates the floating pool pays for.

// DebugOfferingNoReduce prices the offering cast at the full mana cost
// (mutation testing only: the lockstep must catch it).
var DebugOfferingNoReduce bool

// offeringEntry reports whether e is a spell cast through its offering.
func offeringEntry(k *prep, e *stackEntry) bool {
	return e.kind == seSpell && e.trigPl == trigPlMayFlash && k.t.Cards[e.card].HasOffering
}

// offeringSacCost is the printed mana cost of permanent sl as the offering
// reduction reads it (gorge o.Face().ManaCost): a copy's copied cost, none
// for a face-down permanent or a copy of a back face.
func (s *State) offeringSacCost(k *prep, sl Slot) kop.ManaCost {
	if s.faceDown(k, sl) || s.cpxNoCost(k, sl) || s.copiesBackFace(k, sl) {
		return kop.ManaCost{}
	}
	return k.t.Cards[s.cardAt(sl)].Cost
}

// offeringReduce subtracts sac from c symbol by symbol (CR 702.48c; gorge
// offeringReduce): each coloured or colourless pip reduces a pip of its kind;
// a hybrid pip reduces a pip of its printed-first colour, else of its other;
// a monocoloured hybrid or Phyrexian pip reduces a pip of its colour; a snow
// pip reduces snow; whatever reduces nothing, and the generic part, reduces
// the generic part. Nothing goes below zero; {X} reduces nothing.
func offeringReduce(c, sac kop.ManaCost) kop.ManaCost {
	excess := int(sac.Generic)
	take := func(i int) bool {
		if c.Pips[i] > 0 {
			c.Pips[i]--
			return true
		}
		return false
	}
	for i := 0; i < kop.NumManaKinds; i++ {
		for n := sac.Pips[i]; n > 0; n-- {
			if !take(i) {
				excess++
			}
		}
	}
	if sac.Hyb[0] == kop.HybTwobrid {
		for i := 0; i < sac.TbN(); i++ {
			if !take(sac.TbCol(i)) {
				excess++
			}
		}
	} else {
		for i, h := range sac.Hyb {
			if h == 0 {
				continue
			}
			lo, hi := -1, -1
			for col := 0; col < 5; col++ {
				if h&(1<<uint(col)) != 0 {
					if lo < 0 {
						lo = col
					} else {
						hi = col
					}
				}
			}
			a, b := lo, hi
			if sac.HybOrd&(1<<uint(i)) != 0 {
				a, b = hi, lo
			}
			if !take(a) && (b < 0 || !take(b)) {
				excess++
			}
		}
	}
	for n := sac.Snow; n > 0; n-- {
		if c.Snow > 0 {
			c.Snow--
		} else {
			excess++
		}
	}
	if excess >= int(c.Generic) {
		c.Generic = 0
	} else {
		c.Generic -= uint8(excess)
	}
	return c
}

// offeringCand reports whether sl may be sacrificed for p's offering of card
// c: a permanent of the quality p controls that can be sacrificed and is not
// already committed to another cost.
func (s *State) offeringCand(k *prep, c CardID, p uint8, sl Slot) bool {
	return s.matchPerm(k, k.t.Cards[c].OfferingF, sl, p, NoSlot) && !s.sacBlocked(k, sl) && !s.picked(s.perms[sl].obj)
}

// offerOffering offers hand card o's offering cast (gorge offerOffering):
// above the sorcery-speed timing gate, once, when some candidate leaves the
// reduced cost payable.
//
//go:noinline
func (s *State) offerOffering(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if s.castHeldOut(o) || k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
		return // gorge castSuppressed / castRestricted
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) {
		return // castTargetsAvailable; offerCastable's non-mana census
	}
	gorgeSurface := s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.g.availX&availWalk == 0
	cd := &k.t.Cards[c]
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if !s.offeringCand(k, c, p, sl) {
			continue
		}
		cost := s.adjustCost(k, p, c, o, offeringReduce(cd.Cost, s.offeringSacCost(k, sl))) // spellScope("offering")
		if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &cost, av) {
			continue
		}
		if gorgeSurface && !s.sneakPoolPays(k, p, c, o, &cost) {
			// gorge's planner and lowering price no offering mode: the
			// cast is offered only when the floating pool pays
			continue
		}
		d.add(MakeAction(AkMayFlash, TCode(o), 0, 0))
		return
	}
}

// beginOffering casts hand card o through its offering: an ordinary cast,
// never lowered (castFace), marked so buildCost reduces it, with the
// sacrifice election open.
//
//go:noinline
func (s *State) beginOffering(k *prep, o Obj) {
	s.beginCast(k, o, castFace, 0)
	s.bld().e.trigPl = trigPlMayFlash
	s.bld().cas = casElected
}

// offeringOptions poses the offering's sacrifice (gorge sacAsk on the
// reducing Sac part): the candidates whose reduced cost stays payable, in
// battlefield order. false = none remains (the cast keeps its full cost).
func (s *State) offeringOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	d.N = 0
	var av avail
	s.availFor(k, e.ctrl, &av)
	gorgeSurface := s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0
	cd := &k.t.Cards[e.card]
	for m := s.ctrlM[e.ctrl]; m.any(); m = m.rest() {
		sl := m.first()
		if !s.offeringCand(k, e.card, e.ctrl, sl) {
			continue
		}
		cost := s.adjustCost(k, e.ctrl, e.card, e.obj, offeringReduce(cd.Cost, s.offeringSacCost(k, sl)))
		if s.castAffordsR(k, e.ctrl, e.card, s.restrOriginOfEntry(e), &cost, &av) && (!gorgeSurface || s.sneakPoolPays(k, e.ctrl, e.card, e.obj, &cost)) {
			d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
		}
	}
	if d.N == 0 {
		s.bld().cas = 0
		return false
	}
	d.Kind, d.Sub, d.Player = DkChoose, ChCostSacrifice, e.ctrl
	return true
}

// offeringReduced is buildCost's offering arm: c less the picked sacrifice's
// mana cost, once the election is answered.
func (s *State) offeringReduced(k *prep, c kop.ManaCost) kop.ManaCost {
	if s.bld().cas != casPaid || DebugOfferingNoReduce {
		return c
	}
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] != pkSac {
			continue
		}
		ob := &s.objs[s.bld().picks[i]]
		if ob.zone == kop.ZoneBattlefield {
			return offeringReduce(c, s.offeringSacCost(k, ob.slot))
		}
		return offeringReduce(c, k.t.Cards[ob.card].Cost)
	}
	return c
}
