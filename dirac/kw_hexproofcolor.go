package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// hexPrinted is the colours of the printed "hexproof from <colour>" keywords
// (CR 702.11d) of the permanent at sl: 0 when it has lost its abilities
// (gorge hexproofBlocksTarget reads the derived keywords). Cold: targetableGen
// calls it behind genPrep.anyHexCol.
//
//go:noinline
func (s *State) hexPrinted(k *prep, sl Slot) kop.ColorMask {
	c := k.t.Cards[s.cardAt(sl)].HexColors
	if c == 0 || s.genLost(k, sl) {
		return 0
	}
	return c
}

// hexOwnColors is the target's current colours when a permanent its
// controller controls has a StatHexproofOwnColors static matching it (CR
// 702.11d: a granted "hexproof from each of its colours"). A grant is a
// layer-6 effect, so it does not survive a later loss of the target's
// abilities (the rule kwGen uses for a Continuous keyword grant). Cold:
// targetableGen calls it behind genPrep.anyHexOwn.
//
//go:noinline
func (s *State) hexOwnColors(k *prep, sl Slot) kop.ColorMask {
	if k.gen == nil || !k.gen.anyHexOwn {
		return 0
	}
	lts, lost := s.lossAt(k, sl)
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatHexproofOwnColors {
			return true
		}
		if lost && s.grantTs(st) < lts {
			return true
		}
		if s.statCond(k, st) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			hit = true
			return false
		}
		return true
	})
	if !hit {
		return 0
	}
	return s.colorsGen(k, sl)
}

// prepHexOwn arms the own-colours hexproof scan (genPrep.anyHexOwn).
func prepHexOwn(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatHexproofOwnColors {
				g.anyHexOwn = true
				return
			}
		}
	}
}
