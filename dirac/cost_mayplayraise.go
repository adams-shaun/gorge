package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A may-play permission's non-mana RaiseCost$ (Noctis,
// Prince of Lucis "by paying 3 life in addition", Alien Symbiosis "by
// discarding a card in addition", Wickerfolk Indomitable "by paying 2 life
// and sacrificing an artifact or creature in addition"; CR 601.2f/h). A
// spell is cast under ONE permission (CR 601.2b/f, 118.9), so gorge charges
// only that permission's raise (rules/mayplay.go mayPlayRaiseCost: the first
// granting static in the walk order), adds it to the base cost before the
// cost statics apply (payment_plan.go), and withholds the offer when the
// raise cannot be paid. The kernel reads a StMayRaise static's raise from its
// source card's latent ability Static.B-1 (kop/statics.go), owes it as the
// cast's pick cost (buildPickCost) and gates the offer on its payability. The
// two engines pick the same first static in the same order, so they charge
// the same one raise. Cold code: every site is behind prep.gen.anyMayRaise.

// StallMayRaise: the cast owes its permission's RaiseCost$ BESIDE the
// spell's own additional pick cost (the pick cost is one Cost, so the kernel
// can price only one of the two). Two granting statics no longer stall: one
// permission applies and both engines price its raise.
const StallMayRaise = 129

func init() { stallNames[StallMayRaise] = "mayraise" }

// DebugMayRaiseFree is the mutation switch: the raise is neither gated nor
// paid.
var DebugMayRaiseFree bool

// DebugMayRaiseStats counts, for the tests, the raises a cast owed.
var (
	DebugMayRaiseStats bool
	MayRaiseOwed       int
)

// tableMayRaise sets genPrep.anyMayRaise.
func tableMayRaise(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.StFlags&kop.StMayRaise != 0 {
				return true
			}
		}
	}
	return false
}

// mayRaise is the raise p's cast of card object o owes: the Cost of the
// FIRST granting StMayRaise static's latent ability, with the static's slot
// and Static.B; nil when none grants one. One cast is made under one
// permission (CR 601.2b/f, 118.9), so only that permission's raise applies;
// gorge's mayPlayRaiseCost picks the first static in the same walk order, so
// the two engines never sum two grants. A second raise is ignored, not a
// stall.
func (s *State) mayRaise(k *prep, p uint8, o Obj) (c *kop.Cost, sl Slot, b int16) {
	if DebugMayRaiseFree {
		return nil, NoSlot, 0
	}
	took := false
	s.mayPlayStatics(k, p, o, func(st statSrc) bool {
		if st.a.Static.StFlags&kop.StMayRaise == 0 || took {
			return true
		}
		took = true
		card := s.objs[o].card
		if st.sl != NoSlot {
			card = s.cardAt(st.sl)
		}
		c, sl, b = &k.t.Cards[card].Abilities[st.a.Static.B-1].Cost, st.sl, st.a.Static.B
		return true
	})
	return c, sl, b
}

// mayRaiseLife is the life part of a raise: Cost.N, or Cost.Ctr beside a
// sacrifice (kgen lower_mayraise.go raiseCost).
func mayRaiseLife(c *kop.Cost) int {
	if c.Flags&kop.CostSac != 0 {
		return int(c.Ctr)
	}
	return int(c.N)
}

// mayRaisePayable reports whether p can pay raise c for card object o.
func (s *State) mayRaisePayable(k *prep, p uint8, c *kop.Cost, o Obj) bool {
	cc := *c
	if cc.Flags&kop.CostPayLife != 0 {
		if int(s.pl[p].life) < mayRaiseLife(c) {
			return false
		}
		cc.Flags &^= kop.CostPayLife
	}
	return s.pickCostPayable(k, p, &cc, o)
}

// mayRaiseOK is the offer gate: p's may-play of o owes the one raise its
// first granting permission carries, and can pay it (or owes none). An
// unpayable raise withholds the offer (gorge mayPlayRaiseCost reads the same
// first raise).
func (s *State) mayRaiseOK(k *prep, p uint8, o Obj) bool {
	c, _, _ := s.mayRaise(k, p, o)
	return c == nil || s.mayRaisePayable(k, p, c, o)
}

// mayRaiseSpend is mayRaiseOK at the cast: it also records the raise's
// static in the build for buildPickCost.
func (s *State) mayRaiseSpend(k *prep, p uint8, o Obj) bool {
	c, sl, b := s.mayRaise(k, p, o)
	if c == nil {
		return true
	}
	if !s.mayRaisePayable(k, p, c, o) {
		return false
	}
	if ac := spellAddCost(k, s.objs[o].card); ac.Flags&(pickCosts|kop.CostPayLife) != 0 || k.t.Cards[s.objs[o].card].HasAltAdd {
		s.capFull(StallMayRaise) // the spell's own additional pick cost would share the one pick Cost
	}
	s.bld().raiseSl, s.bld().raiseAb = sl, uint8(b)
	if DebugMayRaiseStats {
		MayRaiseOwed++
	}
	return true
}

// mayRaisePick is the raise the entry being built owes, or nil.
func (s *State) mayRaisePick(k *prep, e *stackEntry) *kop.Cost {
	b := s.bld()
	if b.raiseAb == 0 {
		return nil
	}
	card := e.card
	if b.raiseSl != NoSlot {
		card = s.cardAt(b.raiseSl)
	}
	return &k.t.Cards[card].Abilities[b.raiseAb-1].Cost
}

// mayRaiseLifeOwed reports whether the cast being built owes a raise with a
// life part. gorge's planner admits only a fixed sacrifice or discard beside
// the mana (pay.PlanNonManaAdmissible: "cost:life"), so such a cast is
// lowered, not planned.
func (s *State) mayRaiseLifeOwed(k *prep) bool {
	rc := s.mayRaisePick(k, &s.bld().e)
	return rc != nil && rc.Flags&kop.CostPayLife != 0
}

// mayRaiseLandHeld reports whether a LAND card object o is withheld from p's
// may-play offers: some granting static carries a raise, payable or not. A
// land play is free, with no cost site to charge the raise, so gorge
// withholds the land whole (rules/legal_mayplay.go mayPlayLandIds ->
// mayPlayLandGranted: mayPlayRaiseCost's hasRaise). Noctis, Prince of Lucis's
// Artifact.YouCtrl filter matches an artifact land in the graveyard.
func (s *State) mayRaiseLandHeld(k *prep, p uint8, o Obj) bool {
	if !k.cards[s.objs[o].card].land {
		return false
	}
	held := false
	s.mayPlayStatics(k, p, o, func(st statSrc) bool {
		held = st.a.Static.StFlags&kop.StMayRaise != 0
		return !held
	})
	return held
}

// mayRaiseSeen marks in seen every card object priorityMayPlay would ask the
// statics about whose raise withholds the offer: a land a raising permission
// grants (mayRaiseLandHeld), or a raise p cannot pay (mayRaiseOK). Marking
// them up front keeps the checks out of priorityMayPlay's try closure, whose
// inlining the FDN pool bench depends on (gate).
func (s *State) mayRaiseSeen(k *prep, p uint8, zones uint16, seen *objBits) {
	chk := func(o Obj) {
		if seen.has(o) {
			return
		}
		if ob := &s.objs[o]; zones&(1<<ob.zone) == 0 &&
			(ob.zone != kop.ZoneGraveyard || ob.owner() != p || k.gen.sCls[ob.card]&sclsGyMay == 0) &&
			(ob.zone != kop.ZoneExile || ob.owner() != p || k.gen.sCls[ob.card]&sclsExMay == 0) {
			return
		}
		if s.mayRaiseLandHeld(k, p, o) || !s.mayRaiseOK(k, p, o) {
			seen.set(o)
		}
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		q := &s.pl[pp]
		for i := 0; i < int(q.graveN); i++ {
			chk(q.grave[i])
		}
		for i := 0; i < int(q.exileN); i++ {
			chk(q.exile[i])
		}
	}
	pl := &s.pl[p]
	if zones&(1<<kop.ZoneLibrary) != 0 {
		i := pl.libLo
		if DebugLibTopAnyCard {
			i++
		}
		if i < pl.libHi {
			chk(pl.lib[i])
		}
	}
	for i := 0; i < int(pl.handN); i++ {
		chk(pl.hand[i])
	}
}
