package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kcounters: the unfeaturized counter kinds (kop.CtrCharge..) and Station.
//
// The new kinds share the perm's sparse counter slots (ck is a uint8 kind,
// cn its count): no State field changes. They sit above kop.CtrLevel, so
// seeCtr never marks them seen (perm.cseen and FeatCountersListed stay
// inside the frozen feature layout) and ProjObj.Counters never carries them;
// ProjObj.CountersX does, for the oracle and the search API.

// projCtrX records n counters of unfeaturized kind c on po.
//
//go:noinline
func projCtr(po *ProjObj, c uint8, n uint16) {
	if c != 0 && c < uint8(kop.NumCounterKinds2) { // CtrLevel is no counter (kw_class.go)
		po.Counters[c] = n
	} else if c >= uint8(kop.CtrCharge) {
		projCtrX(po, c, n)
	}
}

func projCtrX(po *ProjObj, c uint8, n uint16) {
	if c < uint8(kop.NumCounterKinds3) {
		po.CountersX[c-uint8(kop.CtrCharge)] = n
	}
}

// Station (CR 702.184a; gorge rules/station.go, cards/kw_station.go). The
// kernel ability is an activated one flagged kop.AfStation: CostSorcery and
// CostTapX N=1 over another untapped creature you control (canActivate's
// CostTapX gate is gorge's offer: the sorcery window and a candidate), its
// AkActivate canonicalised as gorge's special action "station". gorge asks
// the pick BEFORE the ability object exists (askStation, then the Tap and
// the KeywordAbilityPush), so the kernel poses it at bsPush, ahead of the
// push: one ChStation pick (gorge's KChoose "station", Min=Max=1), always
// asked, even with one candidate. The pick is recorded as the cost's pkTapX
// tap (payPicks taps it with the cost, its Taps triggers included) and rides
// the entry as its trigger object: the program reads the creature's power
// at resolution (refPT: on the battlefield its current power, elsewhere
// its printed one, or its CDA's, CR 604.3, as gorge's stationAmount), clamped at 0 by kgen.

// prepKC sets genPrep.anyStation: some ability is a Station ability.
func prepKC(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			g.anyStation = g.anyStation || t.Cards[i].Abilities[j].AFlags&kop.AfStation != 0
		}
	}
}

// stationPick poses the Station tap pick of the ability being built
// (buildStep's bsPush, before the push). false: not a Station ability, or
// its pick is answered (the picked creature becomes the entry's trigger
// object) and the push proceeds.
//
//go:noinline
func (s *State) stationPick(k *prep, d *Decision) bool {
	e := &s.bld().e
	if e.kind != seAbility {
		return false
	}
	a := s.buildAbility(k)
	if a == nil || a.AFlags&kop.AfStation == 0 {
		return false
	}
	if s.bld().pickN > 0 {
		e.trigObj = s.bld().picks[0]
		return false
	}
	src := s.srcSlot()
	d.N = 0
	for m := s.ctrlM[e.ctrl]; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].flags&pfTapped != 0 || !s.matchPerm(k, a.Cost.Filter, sl, e.ctrl, src) {
			continue
		}
		d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
	}
	if d.N == 0 {
		return false // canActivate's CostTapX gate makes this unreachable
	}
	s.bld().pickK, s.bld().pickLeft = pkTapX, 1
	d.Kind, d.Sub, d.Player = DkChoose, ChStation, e.ctrl
	return true
}
