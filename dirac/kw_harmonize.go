package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Harmonize (CR 702.180a-b): a graveyard cast for the harmonize cost
// (CardDef.AltCost), tapping up to one untapped creature you control to
// reduce the generic part by that creature's power. The offer prices the
// best single creature (gorge harmonizePayment); the announcement is the
// convoke pick stage with one mode-7 pick (gorge ConvokeAsk, Kind
// "harmonize", Max 1, Min 0). The cast is the flashback graveyard cast
// (mode bit 2): sfFlashback gives the exile-on-leave and the
// cast-from-graveyard answers (gorge object.go's one predicate).

// DebugHarmonizeOneGeneric is the mutation switch (dirac/oracle
// TestHarmonizeMutation): a mode-7 pick subtracts 1 generic instead of the
// creature's power, so the lockstep must diverge.
var DebugHarmonizeOneGeneric bool

// harmonizeEntry reports whether the pending spell e is a harmonize cast.
func harmonizeEntry(cd *kop.CardDef, e *stackEntry) bool {
	return cd.Harmonize && e.flags&sfFlashback != 0
}

// tapPaysEntry reports whether spell entry e announces tapped permanents
// for its cost: printed or granted convoke or improvise, or a harmonize cast.
func (s *State) tapPaysEntry(k *prep, e *stackEntry) bool {
	cd := &k.t.Cards[e.card]
	return cd.Harmonize && e.flags&sfFlashback != 0 || s.tapPays(k, e.ctrl, e.card, e.obj, s.castFromHandBy(e, e.ctrl))
}

// harmonizeReduce is the generic a harmonize pick of the creature at sl
// takes off cost (gorge convokeAbsorbs: its power, at most the generic).
func (s *State) harmonizeReduce(k *prep, sl Slot, gen uint8) uint8 {
	pw := s.power(k, sl)
	if DebugHarmonizeOneGeneric {
		pw = 1
	}
	if pw <= 0 {
		return 0
	}
	if pw > int(gen) {
		return gen
	}
	return uint8(pw)
}

// harmonizeCredit returns cost less the offer-gate credit (gorge
// harmonizePayment): the one untapped, non-bestowed, non-reconfigured
// creature of p with the greatest power, ties to the first in battlefield
// order, takes min(power, Generic) off; none when Generic is 0 or no
// creature has positive power.
func (s *State) harmonizeCredit(k *prep, p uint8, cost kop.ManaCost) kop.ManaCost {
	if cost.Generic == 0 {
		return cost
	}
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(p&1, s.ctrlM[SeatIx(p)].and(s.occ), ord[:], 0)
	best := 0
	for _, sl := range ord[:n] {
		if !s.tapPayCand(k, sl, false) {
			continue
		}
		if pw := s.power(k, sl); pw > best {
			best = pw
		}
	}
	if best <= 0 {
		return cost
	}
	if best > int(cost.Generic) {
		best = int(cost.Generic)
	}
	cost.Generic -= uint8(best)
	return cost
}

// offerHarmonizeCasts offers the graveyard casts of p's Harmonize cards
// (gorge graveyardCastsOver's harmonize loop; the flashback loop's gates
// without the granted and non-mana-cost arms).
func (s *State) offerHarmonizeCasts(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	// the avail is the caller's walk picture: each cast's own delve credit
	// prices its twobrid faces and is restored after, so the next option does
	// not price with it (offerPlay's shape)
	saved := av.delve
	defer func() { av.delve = saved }()
	pl := &s.pl[p]
	have := false
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		c := s.objs[o].card
		cd := &k.t.Cards[c]
		if !cd.Harmonize || !k.cards[c].instant && !sorc && !s.castFlash(k, p, c) {
			continue
		}
		if k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
			continue
		}
		if !have {
			s.availFor(k, p, av)
			have = true
		}
		fc := s.harmonizeCredit(k, p, s.adjustCost(k, p, c, o, cd.AltCost))
		av.delve = s.tbDelveCredit(k, p, c, o, false) // the faces' credit (CR 702.66a)
		if !av.tbLeafCredited(&fc) {
			fc = s.delveOfferCredit(k, p, c, o, false, fc)
		}
		if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &fc, av) || !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) {
			continue
		}
		if s.castHeldOut(o) {
			continue
		}
		if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.g.availX&availWalk == 0 && !s.FeatPoolPays(p, o, 2) {
			// gorge's planner and lowering price no harmonize mode
			// ("unsupported"): the cast is offered only when the floating
			// pool pays the credited cost (the offspring cast's rule)
			continue
		}
		d.add(MakeAction(AkCast, uint8(o), 0, 2))
	}
}

// harmonizeOptions poses the harmonize announcement (gorge ConvokeAsk with
// Harmonize): one mode-7 option per untapped creature of power > 0 while
// the cost still has generic to reduce or an unannounced {X} (the X ask
// then prices the pick, convokeXCost), then Done (Min 0). The ask closes
// at Max 1, so a pick commits without a Done. Not posed with no option.
func (s *State) harmonizeOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if int(s.bld().pickN) >= len(s.bld().picks) {
		s.capFull(StallConvoke)
		return false
	}
	if c := s.buildCost(k); s.convokePickCount() >= 1 || c.Generic == 0 && c.X == 0 {
		return false
	}
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(e.ctrl, s.ctrlM[e.ctrl].and(s.occ), ord[:], 0)
	d.N = 0
	d.Kind, d.Sub, d.Player = DkChoose, ChGenConvoke, e.ctrl
	for _, sl := range ord[:n] {
		if !s.tapPayCand(k, sl, false) || s.pickedKind(s.perms[sl].obj, pkTapX) || s.power(k, sl) <= 0 {
			continue
		}
		if d.N >= MaxOptions-1 {
			s.capFull(StallConvoke)
			break
		}
		d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 7))
	}
	if d.N == 0 {
		return false
	}
	d.add(MakeAction(AkDone, 0, 0, 0))
	return true
}

// FeatHarmonizeCard reports whether o's card is a Harmonize card (the
// oracle canonicalises its mode-2 cast as gorge's "harmonize" option).
func (s *State) FeatHarmonizeCard(o Obj) bool {
	return s.k().t.Cards[s.objs[o].card].Harmonize
}
