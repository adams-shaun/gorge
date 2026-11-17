package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Granted statics: S:Mode$ Continuous | AddStaticAbility$ <SVar> gives a
// static to the objects the outer static affects, each with ITSELF as the
// static's source and its own controller as "you" (gorge rules/layers.go
// AddStaticAbility registers one instance per recipient). The inner statics
// live in CardDef.Granted, not Abilities, so no printed walk (eachStatic and
// its siblings, the statM loops) ever sees one: only a reader that walks
// eachStaticG does, and kgen lowers a granted kind only once its reader has
// moved (grantedStaticKinds). Wired readers: StatNoMaxHandSize (maxHand), StatCantSacrifice
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
			if a := &t.Cards[i].Granted[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatNoMaxHandSize {
				g.anyHandSize = true
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
		cd := &k.t.Cards[s.cardAt(src)]
		if len(cd.Granted) == 0 || s.genLost(k, src) {
			continue
		}
		sp := &s.perms[src]
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
