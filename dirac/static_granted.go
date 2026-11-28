package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Granted statics: S:Mode$ Continuous | AddStaticAbility$ <SVar> gives a
// static to the objects the outer static affects, each with ITSELF as the
// static's source and its own controller as "you" (gorge rules/layers.go
// AddStaticAbility registers one instance per recipient). The inner statics
// live in CardDef.Granted, not Abilities, so no printed walk (eachStatic and
// its siblings, the statM loops) ever sees one: only a reader that walks
// eachStaticG does, and kgen lowers a granted kind only once its reader has
// moved (grantedStaticKinds). Wired readers: StatNoMaxHandSize (maxHand);
// StatIncreaseCost, a generic RaiseCost (raiseCostGen and the target-dependent
// raise walks boardManaTgtDep, boardRaiseTgtDep: Callaphe's per-recipient
// "spells your opponents cast that target this cost {1} more"); StatCantSacrifice
// (sacBlockedBy).
//
// Recursion: the recipient set is read at layerKw, below layer 6, exactly as
// grantsBetween reads the recipients of a granted ability, and no characteristic
// read (chars, layers) calls eachStaticG, so a recipient that is its own source
// cannot ask for the set it is part of.

// armGrantedStatics sets the prep flags the granted statics of t need. No
// flag of its own marks a table with granted statics (a fixed field is pinned,
// layout_pins_test.go): eachGranted asks each carrier for its own Granted.
func armGrantedStatics(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Granted {
			a := &t.Cards[i].Granted[j]
			if a.Kind != kop.AbStatic {
				continue
			}
			switch a.Static.Kind {
			case kop.StatNoMaxHandSize:
				g.anyHandSize = true
			case kop.StatIncreaseCost:
				g.anyRaise = true
				g.anyRaiseTgt = g.anyRaiseTgt || raiseTgtStatic(a)
			}
		}
	}
}

// eachStaticG calls f for every static in force (eachStatic), then for every
// static a Continuous static in force grants; f returns false to stop.
func (s *State) eachStaticG(k *prep, f func(st statSrc) bool) {
	stopped := false
	s.eachStatic(k, func(st statSrc) bool {
		stopped = !f(st)
		return !stopped
	})
	if !stopped {
		s.eachGranted(k, f)
	}
}

// eachGranted yields the granted statics: per carrier in statM order, per
// granting static in ability order, per recipient in slot order. The yielded
// statSrc names the recipient (sl, obj) and its controller; gr marks it
// granted and gsrc is the grant's source, whose timestamp orders it (CR 613.7).
func (s *State) eachGranted(k *prep, f func(st statSrc) bool) {
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if s.genLost(k, src) {
			continue
		}
		sp := &s.perms[src]
		pn := 0
		if k.piles {
			pn = s.pileNAt(src) // CR 702.140d: a pile's under cards (kw_mutateunion.go)
		}
		for pj := 0; pj <= pn; pj++ {
			c := s.cardAt(src)
			if pj > 0 {
				c = s.pileUnder(k, src, pj-1)
			}
			cd := &k.t.Cards[c]
			if len(cd.Granted) == 0 {
				continue
			}
			for i := range cd.Abilities {
				a := &cd.Abilities[i]
				if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.GrantSt == 0 {
					continue
				}
				outer := statSrc{a: a, sl: src, ctrl: sp.ctrl, obj: sp.obj}
				if !s.statCond(k, outer) {
					continue
				}
				g := &cd.Granted[a.Static.GrantSt-1]
				for pl := uint8(0); pl < s.nSeats(); pl++ {
					for r := s.ctrlM[pl]; r.any(); r = r.rest() {
						rcp := r.first()
						if !s.matchPermGen(k, a.Static.Filter, rcp, sp.ctrl, src, layerKw) {
							continue
						}
						if s.genLost(k, rcp) {
							// layer 6 in timestamp order: a loss newer than the grant removes it
							if lts, _ := s.lossAt(k, rcp); s.layerTs(src) < lts {
								continue
							}
						}
						rp := &s.perms[rcp]
						if !f(statSrc{a: g, sl: rcp, ctrl: rp.ctrl, obj: rp.obj, gr: true, gsrc: src}) {
							return
						}
					}
				}
			}
		}
	}
}
