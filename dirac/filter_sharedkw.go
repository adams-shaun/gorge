package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The keyword-presence reads behind the SharedKeywordsZone$ PumpAll lowering
// (dirac/kgen/lower_sharedpumpall.go). Each reads the permanent's derived
// keyword set, so a printed keyword, a Continuous static's grant and an
// until-end-of-turn override record all answer, and a "loses all abilities"
// effect removes the printed one. They are the condition half of one
// conditional grant per keyword: the lowering asks whether any battlefield
// permanent matching SharedRestrictions$ has the spelling.

// filterChainEqual reports whether the two filter chains hold the same links
// in the same order, ignoring each link's Or pointer (it is the chain). Both
// chains are built for the battlefield, so Zone agrees.
func filterChainEqual(t *kop.Table, a, b kop.FilterID) bool {
	for a != 0 && b != 0 {
		fa, fb := t.Filters[a], t.Filters[b]
		fa.Or, fb.Or = 0, 0
		if fa != fb {
			return false
		}
		a, b = t.Filters[a].Or, t.Filters[b].Or
	}
	return a == 0 && b == 0
}

// hasLandwalkSpec: the permanent at sl has a landwalk whose land filter is
// spec among its derived keywords (gorge rules/combat/block.go LandwalkEvades
// over the layer-6 keyword list): its printed CardDef.Landwalk unless it lost
// its abilities, a Continuous static's Static.Landwalk (a graveyard card's
// too), or an override record's Override.Landwalk (a Pump's grant). A grant
// older than a loss of all abilities grants nothing (the rule hasIslandwalk
// uses).
//
//go:noinline
func (s *State) hasLandwalkSpec(k *prep, sl Slot, spec kop.FilterID) bool {
	if f := k.t.Cards[s.cardAt(sl)].Landwalk; f != 0 && !s.genLost(k, sl) && filterChainEqual(k.t, f, spec) {
		return true
	}
	lts, lost := s.lossAt(k, sl)
	hit := false
	grant := func(st statSrc) bool {
		ab := st.a
		if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatContinuous || ab.Static.Landwalk == 0 ||
			lost && s.grantTs(st) < lts || !filterChainEqual(k.t, ab.Static.Landwalk, spec) {
			return true
		}
		if s.statCond(k, st) && s.matchPermGen(k, ab.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			hit = true
			return false
		}
		return true
	}
	s.eachStatic(k, grant)
	if !hit && k.gyCont {
		s.eachGyStatic(k, 0, grant)
	}
	if !hit {
		s.eachOverrideTs(k, sl, func(o *kop.Override, ts uint16) {
			if o.Landwalk != 0 && (!lost || ts >= lts) && filterChainEqual(k.t, o.Landwalk, spec) {
				hit = true
			}
		})
	}
	return hit
}

// hasProtQuality: the permanent at sl has a protection quality whose source
// filter is spec among its derived keywords: its printed CardDef.ProtFrom
// unless it lost its abilities, a Continuous static's Static.ProtFrom, or an
// override record's Override.ProtFrom (a Pump's grant). Colour qualities are
// hasProtColor's; this reads the spec-shaped ones (protection from artifacts,
// from creatures, ...).
//
//go:noinline
func (s *State) hasProtQuality(k *prep, sl Slot, spec kop.FilterID) bool {
	if f := k.t.Cards[s.cardAt(sl)].ProtFrom; f != 0 && !s.genLost(k, sl) && filterChainEqual(k.t, f, spec) {
		return true
	}
	lts, lost := s.lossAt(k, sl)
	hit := false
	grant := func(st statSrc) bool {
		ab := st.a
		if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatContinuous || ab.Static.ProtFrom == 0 ||
			lost && s.grantTs(st) < lts || !filterChainEqual(k.t, ab.Static.ProtFrom, spec) {
			return true
		}
		if s.statCond(k, st) && s.matchPermGen(k, ab.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			hit = true
			return false
		}
		return true
	}
	s.eachStatic(k, grant)
	if !hit && k.gyCont {
		s.eachGyStatic(k, 0, grant)
	}
	if !hit && k.gen.anyProtGrant {
		s.eachOverrideTs(k, sl, func(o *kop.Override, ts uint16) {
			if o.ProtFrom != 0 && (!lost || ts >= lts) && filterChainEqual(k.t, o.ProtFrom, spec) {
				hit = true
			}
		})
	}
	return hit
}

// hasProtColor: the permanent at sl has protection from any colour in mask
// among its derived keywords: printed ProtColors (and a chosen-colour grant,
// protPrinted), the KwProtBlack and KwProtRed bits (a printed or granted
// spelling), or the colour clauses of a Continuous static's or override
// record's ProtFrom (grantedProtColKw). "Protection from everything" is its
// own keyword, read through PredHasKeyword.
func (s *State) hasProtColor(k *prep, sl Slot, mask kop.ColorMask) bool {
	if mask == 0 {
		return false
	}
	return s.protColours(k, sl)&mask != 0
}

// hasProtEachColor: the permanent at sl has protection from every colour
// ("Protection from each color", CR 702.16's colour spellings of the family
// word).
func (s *State) hasProtEachColor(k *prep, sl Slot) bool {
	const all = kop.ColW | kop.ColU | kop.ColB | kop.ColR | kop.ColG
	return s.protColours(k, sl)&all == all
}

// protColours is the derived protection-from-colour mask the two colour
// presence words read.
func (s *State) protColours(k *prep, sl Slot) kop.ColorMask {
	col := s.protPrinted(k, sl)
	kw := s.kw(k, sl)
	if kw&kop.KwProtBlack != 0 {
		col |= kop.ColB
	}
	if kw&kop.KwProtRed != 0 {
		col |= kop.ColR
	}
	if k.gen.anyProtGrant {
		col |= s.grantedProtColours(k, sl)
	}
	return col
}

// grantedProtColours is the colours of a Continuous static's or override
// record's colour protection qualities on sl (gorge lists each as a
// "Protection from <colour>" keyword): protPrinted drops them for white, blue
// and green, which have no KwMask bit, so the clauses are read directly.
func (s *State) grantedProtColours(k *prep, sl Slot) kop.ColorMask {
	lts, lost := s.lossAt(k, sl)
	var col kop.ColorMask
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.ProtFrom == 0 ||
			lost && s.grantTs(st) < lts {
			return true
		}
		if s.statCond(k, st) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			col |= protColKwOf(k, a.Static.ProtFrom)
		}
		return true
	})
	s.eachOverrideTs(k, sl, func(o *kop.Override, ts uint16) {
		if o.ProtFrom != 0 && (!lost || ts >= lts) {
			col |= protColKwOf(k, o.ProtFrom)
		}
	})
	return col
}
