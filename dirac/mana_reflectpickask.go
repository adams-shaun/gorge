package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The reflected pick of a planned tap. An all-at-once source under one reflect
// carrier (Azorius Chancery under Mana Flare) makes its whole production and
// one more mana of ONE of the types it produced (CR 106.12a). Gorge's carrier
// asks the controller which, "Choose a colour of mana to reflect", one option
// per produced type in WUBRGC order ({C} only when the carrier widens the
// type, so a production with a single type beside {C} has no choice and no
// ask). The plan prices the pick as a unit of the produced mask (srcEachPick,
// planEachPick), which names a colour of its own; the tap itself asks.
//
// The park is the mana sacrifice's (repl_exilemark.go manaSacParks): plan
// stops after the tap, the production floated and the pick's guess taken
// back, and ctlReflectPick poses the colour. The answer floats the mana and
// goes on with the payment, which plans again against the pool, so the
// sources already tapped stay tapped (the build's mana step through
// bflPlanCut; a resolution's payment through payManaResume).

// reflMask's colour bits are 0..4 (WUBRG); bits 5..7 count the asks still owed
// after the one posed (two carriers over an all-at-once source ask twice).
const (
	reflColMask   = 1<<reflOwedShift - 1
	reflOwedShift = 5
	reflOwedMax   = 7
)

// reflectPickPark settles the reflected pick of the plan unit ty, whose tap at
// colour col the plan has just floated, and reports whether plan must stop for
// the ask. A pick with one candidate floats that colour; one a probe or a
// payment that cannot wait for an answer reaches stalls (StallTapExtra).
func (s *State) reflectPickPark(p uint8, ty *utype, col int) bool {
	m := ty.mask &^ (1 << kop.C)
	if m == 0 || s.isFork() {
		s.capFull(StallTapExtra)
		return false
	}
	if m&(m-1) == 0 {
		s.pl[p].pool[col] -= ty.n
		s.poolAdd(p, bits.TrailingZeros8(m), int(ty.n))
		return false
	}
	if !s.manaSacParks() || ty.n > reflOwedMax+1 {
		s.capFull(StallTapExtra)
		return false
	}
	s.pl[p].pool[col] -= ty.n // the plan's guess; the answers float the colours picked
	// one ask per carrier (ty.n): the asks still owed after this one ride the
	// mask's bits above the colours
	s.g.reflMask, s.g.reflP = m|(ty.n-1)<<reflOwedShift, p
	if s.buildLive() && s.bld().bfl&bflManaStep != 0 {
		s.bld().bfl |= bflPlanCut
	}
	return true
}

// reflectPickPending: a planned tap waits on its pick's colour.
func (s *State) reflectPickPending() bool { return s.g.reflMask != 0 }

// manaAskWaiting sets the control state of the ask a planned payment stopped
// on and reports it: a sacrificed source's order ask (ctlExileOrder) or a
// tap's reflected pick (ctlReflectPick).
func (s *State) manaAskWaiting() bool {
	switch {
	case s.parkedWaiting():
		s.ctl = ctlExileOrder
	case s.reflectPickPending():
		s.ctl = ctlReflectPick
	default:
		return false
	}
	return true
}

// reflectPickOptions poses the pick: one DkChoose/ChGenManaChoice, mode = a
// colour of the production (W U B R G = 0..4).
func (s *State) reflectPickOptions(d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChGenManaChoice, s.g.reflP
	for c := uint8(0); c < 5; c++ {
		if s.g.reflMask&reflColMask&(1<<c) != 0 {
			d.add(MakeAction(AkChoose, 0, 0, c))
		}
	}
}

// applyReflectPick floats the colour picked and goes on with the payment.
func (s *State) applyReflectPick(k *prep, a Action) {
	s.poolAdd(s.g.reflP, int(a.Mode()), 1)
	if owed := s.g.reflMask >> reflOwedShift; owed > 0 {
		s.g.reflMask = s.g.reflMask&reflColMask | (owed-1)<<reflOwedShift
		s.ctl = ctlReflectPick // the next carrier's pick: the next Step asks again
		return
	}
	s.g.reflMask, s.g.reflP = 0, 0
	if s.res.active {
		s.ctl = ctlResolve
		s.payManaResume(k)
		return
	}
	s.ctl = ctlBuild
}
