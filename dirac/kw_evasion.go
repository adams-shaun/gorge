package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// landwalkEvades: attacker a's printed landwalk (CardDef.Landwalk) makes it
// unblockable while the defending player (blocker b's controller)
// controls a land matching the filter (CR 702.14; gorge rules/combat
// block.go LandwalkEvades, over its current keywords: none once it lost
// its abilities).
//
//go:noinline
func (s *State) landwalkEvades(k *prep, b, a Slot) bool {
	if f := k.t.Cards[s.cardAt(a)].Landwalk; f != 0 && !s.genLost(k, a) && !s.landwalkRemoved(k, a, f, 0) && s.landwalkHit(k, f, b, a, a) {
		return true
	}
	if s.landwalkGranted(k, b, a) || s.landwalkPumped(k, b, a) {
		if DebugLandwalkStats != nil {
			DebugLandwalkStats.GrantedEvades++
		}
		return true
	}
	return false
}

// landwalkPumped: an override record on attacker a carrying a Landwalk filter
// (a Pump KW$ Landwalk:<spec>, Piracy Charm) matches a land the defending
// player controls. Like a static's grant it is a layer-6 effect, so a record
// older than a's loss of all abilities grants nothing.
//
//go:noinline
func (s *State) landwalkPumped(k *prep, b, a Slot) bool {
	if !s.fxM.has(a) {
		return false
	}
	lts, lost := s.lossAt(k, a)
	hit := false
	s.eachOvRec(a, func(r ovRec) {
		if f := k.t.Overrides[r.idx-1].Landwalk; f != 0 && !hit && (!lost || r.ts >= lts) &&
			!s.landwalkRemoved(k, a, f, r.ts) && s.landwalkHit(k, f, b, a, a) {
			hit = true
		}
	})
	return hit
}

// useRemoveLandwalk arms the landwalk-removal scan: a table with any
// kop.Override.RemoveLandwalk record takes the check at every landwalk read
// (kw_evasion.go landwalkRemoved). A table without one skips it.
var useRemoveLandwalk = newGenUse(tableHasRemoveLandwalk)

// tableHasRemoveLandwalk reports whether some override record removes a
// parameterised landwalk (RemoveKeyword$ Landwalk:Island, Mystic Decree).
func tableHasRemoveLandwalk(t *kop.Table) bool {
	for i := range t.Overrides {
		if t.Overrides[i].RemoveLandwalk != 0 {
			return true
		}
	}
	return false
}

// landwalkRemoved reports whether a layer-6 removal removes the landwalk whose
// land filter is f from sl (kop.Override.RemoveLandwalk: RemoveKeyword$
// Landwalk:Island, Mystic Decree). grantTs is the grant's layer timestamp (0
// for the printed keyword at the base): a removal older than the grant leaves
// it, as does one tied with it (timestamps of two layer-6 effects are
// distinct, so a tie cannot happen), and only a newer one takes it; a removal older than a loss of all abilities does not apply.
//
//go:noinline
func (s *State) landwalkRemoved(k *prep, sl Slot, f kop.FilterID, grantTs uint16) bool {
	if f == 0 || k.gen == nil || !k.gen.uses.has(useRemoveLandwalk) {
		return false
	}
	lts, lost := s.lossAt(k, sl)
	removed := false
	s.eachOverrideTs(k, sl, func(o *kop.Override, ts uint16) {
		if removed || o.RemoveLandwalk == 0 || !filterChainEqual(k.t, o.RemoveLandwalk, f) {
			return
		}
		if lost && ts < lts {
			return
		}
		if grantTs != 0 && ts <= grantTs {
			return
		}
		removed = true
	})
	if removed {
		return true
	}
	check := func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Override <= 0 {
			return true
		}
		o := &k.t.Overrides[a.Static.Override-1]
		if o.RemoveLandwalk == 0 || !filterChainEqual(k.t, o.RemoveLandwalk, f) {
			return true
		}
		ts := s.grantTs(st)
		if lost && ts < lts {
			return true
		}
		if grantTs != 0 && ts <= grantTs {
			return true
		}
		if s.statCond(k, st) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			removed = true
			return false
		}
		return true
	}
	s.eachStatic(k, check)
	if !removed && k.gyCont {
		s.eachGyStatic(k, 0, check)
	}
	return removed
}

// landwalkHit: the defending player (blocker b's controller) controls a land
// matching the landwalk filter f of attacker a. src is the filter's source
// permanent: the attacker for a printed or pumped landwalk, the granting
// static's host for a Continuous grant, so a ChosenType land spec (Traveler's
// Cloak's "landwalk of the chosen type") reads the Aura's choice.
func (s *State) landwalkHit(k *prep, f kop.FilterID, b, a, src Slot) bool {
	if k.gen != nil && k.gen.uses.has(useIgnoreLandWalk) {
		return s.landwalkHitLinks(k, f, b, a, src) // static_ignorelandwalk.go
	}
	def := s.perms[b].ctrl
	for m := s.ctrlM[def]; m.any(); m = m.rest() {
		sl := m.first()
		if s.typesAt(k, sl)&kop.TLand != 0 && s.matchPerm(k, f, sl, s.perms[a].ctrl, src) {
			return true
		}
	}
	return false
}

// landwalkGranted: a Continuous static's Landwalk, granted to attacker a,
// matches a land the defending player controls (CR 702.14; Goblin King's
// mountainwalk). A grant is a layer-6 effect, so it does not survive a later
// loss of a's abilities (the rule kwGen and protGranted use).
//
//go:noinline
func (s *State) landwalkGranted(k *prep, b, a Slot) bool {
	lts, lost := s.lossAt(k, a)
	hit := false
	f := func(st statSrc) bool {
		ab := st.a
		if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatContinuous || ab.Static.Landwalk == 0 {
			return true
		}
		if lost && s.grantTs(st) < lts {
			return true
		}
		if s.landwalkRemoved(k, a, ab.Static.Landwalk, s.grantTs(st)) {
			return true
		}
		src := st.sl // the host's choice; a graveyard grant (no slot) reads the attacker
		if src == NoSlot {
			src = a
		}
		if s.statCond(k, st) && s.matchPermGen(k, ab.Static.Filter, a, st.ctrl, st.sl, layerKw) &&
			s.landwalkHit(k, ab.Static.Landwalk, b, a, src) {
			hit = true
			if DebugLandwalkStats != nil && k.t.Filters[ab.Static.Filter].Flags&kop.FAttachedBySrc != 0 {
				DebugLandwalkStats.AttachedEvades++ // an Aura's or an Equipment's grant
			}
			return false
		}
		return true
	}
	s.eachStatic(k, f)
	if !hit && k.gyCont {
		s.eachGyStatic(k, 0, f) // a graveyard card's landwalk grant (static_gycont.go)
	}
	return hit
}

// islandSub is the filter Sub value (subtype index+1) of the matchup's Island
// subtype, 0 when it has none: no land filter then names it.
func islandSub(k *prep) uint8 {
	if i := k.gen.blgSub[1]; i != subOutside {
		return i + 1
	}
	return 0
}

// landwalkNames reports whether an Or-chain link of the land filter f is the
// bare subtype filter of subtype index sub-1 (Landwalk:Island lowers to a
// clause with only Sub set; kgen TestLacksIslandwalkFilterShape).
func landwalkNames(t *kop.Table, f kop.FilterID, sub uint8) bool {
	for f != 0 {
		fl := t.Filters[f]
		next := fl.Or
		fl.Or, fl.Zone = 0, kop.ZoneNone
		if fl.Sub == sub && sub != 0 {
			fl.Sub = 0
			if fl == (kop.Filter{}) {
				return true
			}
		}
		f = next
	}
	return false
}

// hasIslandwalk: the permanent at sl has a Landwalk keyword naming the
// subtype sub-1 among its derived keywords (gorge keywordPredicateMatches
// over the layer-6 keyword list): its printed landwalk unless it lost its
// abilities, or a Continuous static's grant (the rules landwalkGranted reads,
// also from a graveyard card's landwalk grant, static_gycont.go), or a Pump's
// override record (landwalkPumped).
//
//go:noinline
func (s *State) hasIslandwalk(k *prep, sl Slot) bool {
	sub := islandSub(k)
	if f := k.t.Cards[s.cardAt(sl)].Landwalk; f != 0 && !s.genLost(k, sl) && !s.landwalkRemoved(k, sl, f, 0) && landwalkNames(k.t, f, sub) {
		return true
	}
	lts, lost := s.lossAt(k, sl)
	hit := false
	f := func(st statSrc) bool {
		ab := st.a
		if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatContinuous || ab.Static.Landwalk == 0 ||
			!landwalkNames(k.t, ab.Static.Landwalk, sub) || lost && s.grantTs(st) < lts ||
			s.landwalkRemoved(k, sl, ab.Static.Landwalk, s.grantTs(st)) {
			return true
		}
		if s.statCond(k, st) && s.matchPermGen(k, ab.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			hit = true
			return false
		}
		return true
	}
	s.eachStatic(k, f)
	if !hit && k.gyCont {
		s.eachGyStatic(k, 0, f) // a graveyard card's landwalk grant (static_gycont.go)
	}
	if !hit && s.fxM.has(sl) {
		// a Pump's granted landwalk (landwalkPumped)
		s.eachOvRec(sl, func(r ovRec) {
			if f := k.t.Overrides[r.idx-1].Landwalk; f != 0 && (!lost || r.ts >= lts) &&
				!s.landwalkRemoved(k, sl, f, r.ts) && landwalkNames(k.t, f, sub) {
				hit = true
			}
		})
	}
	return hit
}

// landwalkGrantIn reports whether any ability is a Continuous static
// granting landwalk.
func landwalkGrantIn(abs []kop.Ability) bool {
	for j := range abs {
		if a := &abs[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Landwalk != 0 {
			return true
		}
	}
	return false
}

// keyword arms (cold, out of line).

// artifactTy is slot sl's artifact supertype for the fear / intimidate
// blocker checks (gorge bf.IsArtifact over the object's current face): the
// card types its face resolves to plus its layer-1 copy exceptions' added
// types (CR 707.9b — a copy that is an artifact in addition to its other
// types is an artifact creature), as phasedTypes composes them.
func (s *State) artifactTy(k *prep, sl Slot) bool {
	t := s.typesAt(k, sl)
	if s.fxM.has(sl) {
		t |= s.copyExcTypes(k, sl)
	}
	return t&kop.TArtifact != 0
}

// evadesShadowFear reports whether the fear / shadow rules forbid blocker b from
// blocking attacker a with keywords akw (gorge combat.CanBlock):
// CR 702.28b-c a shadow creature blocks and is blocked only by shadow
// creatures; CR 702.36b a fear creature is blocked only by an artifact
// creature (its current face, gorge bf.IsArtifact) or a black one
// (its derived colours). A live StatCanBlockIfShadow static on the pair
// lifts the shadow rule for exactly that pair (gorge shadowBlockLifted;
// dirac/static_canblockifshadow.go).
func (s *State) evadesShadowFear(k *prep, b, a Slot, akw, bkw kop.KwMask) bool {
	if (akw^bkw)&kop.KwShadow != 0 && !s.shadowLifted(k, b, a) {
		return true
	}
	if akw&kop.KwFear != 0 && !s.artifactTy(k, b) && s.colorsAt(k, b)&kop.ColB == 0 {
		return true
	}
	return false
}

// evasionAt is the permanent at sl's printed CardDef.Evasion bits (a pile:
// those of all its cards), none once it has lost its abilities, plus its
// granted ones (gorge HasKW over the derived keywords).
func (s *State) evasionAt(k *prep, sl Slot) uint8 {
	var ev uint8
	s.eachKwCard(k, sl, func(c CardID) bool {
		ev |= k.t.Cards[c].Evasion
		return true
	})
	if ev != 0 && s.genLost(k, sl) {
		ev = 0
	}
	if k.gen != nil && k.gen.anyEvasionGrant {
		ev |= s.evasionGranted(k, sl) // a static's or a mark's grant (kw_kwmark.go)
	}
	return ev
}

// evasionEvades: attacker a's printed horsemanship or intimidate forbids
// blocker b (gorge rules/combat/block.go CanBlock): horsemanship unless b
// has horsemanship (CR 702.31b); intimidate unless b is an artifact
// creature (its current face, gorge bf.IsArtifact, as Fear's) or shares a
// current colour with a (CR 702.13a).
//
//go:noinline
func (s *State) evasionEvades(k *prep, b, a Slot) bool {
	ev := s.evasionAt(k, a)
	if ev&kop.EvHorsemanship != 0 && s.evasionAt(k, b)&kop.EvHorsemanship == 0 {
		if DebugLandwalkStats != nil && k.t.Cards[s.cardAt(a)].Evasion&kop.EvHorsemanship == 0 {
			DebugLandwalkStats.GrantedHorse++ // no printed horsemanship: a mark's grant
		}
		return true
	}
	if ev&kop.EvIntimidate != 0 && !s.artifactTy(k, b) && s.colorsAt(k, a)&s.colorsAt(k, b)&kop.ColAny == 0 {
		return true
	}
	if ev&kop.EvSkulk != 0 && s.skulkEvades(k, b, a) {
		return true
	}
	return false
}

// skulkEvades: attacker a's printed skulk forbids a blocker b with greater
// power (CR 702.118a; gorge rules/combat/block.go CanBlock: derived powers
// at declaration).
//
//go:noinline
func (s *State) skulkEvades(k *prep, b, a Slot) bool {
	bp, _ := s.ptGen(k, b)
	ap, _ := s.ptGen(k, a)
	if bp > ap && DebugLandwalkStats != nil && k.t.Cards[s.cardAt(a)].Evasion&kop.EvSkulk == 0 {
		DebugLandwalkStats.GrantedSkulk++ // no printed skulk: a static's or a mark's grant
	}
	return bp > ap
}
