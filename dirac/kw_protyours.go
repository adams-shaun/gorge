package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Protection from the colors of permanents you control (kop.StatProtYourColors,
// Empty-Shrine Kannushi; gorge rules/protection_colors_you_ctrl.go
// sourceSharesControlledColor). The quality is relative to the protected
// permanent's controller, so it is not a filter over the source
// (Static.ProtFrom): the source is protected from when it shares a colour with
// some battlefield permanent that controller controls, every colour read live
// (colorsGen), the protected permanent itself among them.

// protYours: a StatProtYourColors static grants sl protection, and the source
// (the permanent at srcSl, else card srcCard as a spell or a departed source)
// shares a colour with a permanent sl's controller controls. A grant is a
// layer-6 effect, so it does not survive a later loss of sl's abilities.
// Cold: protQuality calls it behind genPrep.anyProtGrant.
//
//go:noinline
func (s *State) protYours(k *prep, sl, srcSl Slot, srcCard CardID) bool {
	lts, lost := s.lossAt(k, sl)
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatProtYourColors {
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
		return false
	}
	var src kop.ColorMask
	if srcSl != NoSlot && s.occ.has(srcSl) {
		src = s.colorsGen(k, srcSl)
	} else {
		src = k.t.Cards[srcCard].Colors
	}
	if src == 0 {
		return false
	}
	ctrl := s.perms[sl].ctrl
	var own kop.ColorMask
	for m := s.occ; m.any(); m = m.rest() {
		if o := m.first(); s.perms[o].ctrl == ctrl {
			own |= s.colorsGen(k, o)
		}
	}
	return src&own != 0
}

// prepProtYours arms the granted-protection reads (the targeting, block and
// attach sites through anyProtF, protQuality through anyProtGrant) and the
// damage sites (rbProtCol) for a card with a StatProtYourColors static.
func prepProtYours(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatProtYourColors {
				g.anyProtGrant, g.anyProtF = true, true
				g.replBits[i] |= rbProtCol
			}
		}
	}
}
