package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Trigger-event and keyword classes 19-62: cold,
// out-of-line arms.

// protPrinted is the permanent at sl's printed protection-from-colour
// colours (CardDef.ProtColors; CR 702.16), none once it has lost its
// abilities (gorge protectedFrom reads the derived keywords), and the
// colours a chosen-colour grant gave it until end of turn (cgProtOf).
//
//go:noinline
func (s *State) protPrinted(k *prep, sl Slot) kop.ColorMask {
	c := k.t.Cards[s.cardAt(sl)].ProtColors
	if c != 0 && s.genLost(k, sl) {
		c = 0
	}
	if k.gen.anyColGrant {
		c |= s.colProtOf(k, sl) // static_colourprot.go
	}
	return c
}

// protFrom reports whether permanent sl has protection from a damage
// source (slot srcSl, NoSlot off the battlefield; card srcCard; colours
// col): from one of its colours (protColFrom), or a K:Protection:<spec>
// quality it matches (CardDef.ProtFrom or a granted one; gorge
// sourceHasQuality).
func (s *State) protFrom(k *prep, sl, srcSl Slot, srcCard CardID, col kop.ColorMask, combat bool) bool {
	return (s.protColFrom(k, sl, col) || k.gen.anyProtF && s.protQuality(k, sl, srcSl, srcCard)) && !s.noPrev(k, combat) // CR 615.12
}

// protQuality: sl's printed K:Protection:<spec> filter matches the source
// (the permanent at srcSl, else card srcCard as a spell or a departed
// source), or a granted one does (protGranted); the printed one is none
// once sl lost its abilities.
//
//go:noinline
func (s *State) protQuality(k *prep, sl, srcSl Slot, srcCard CardID) bool {
	return s.protQualityAt(k, sl, srcSl, srcCard, false)
}

// protQualityAt is protQuality; sba marks the Aura state-based check
// (turn.go), where a granted protection's exception spares its source (the
// attached Auras and Equipment a Benevolent Blessing leaves, CR 702.16).
//
//go:noinline
func (s *State) protQualityAt(k *prep, sl, srcSl Slot, srcCard CardID, sba bool) bool {
	if f := k.t.Cards[s.cardAt(sl)].ProtFrom; f != 0 && !s.genLost(k, sl) && s.protMatch(k, f, sl, srcSl, srcCard) {
		return true
	}
	return k.gen.anyProtGrant && s.protGranted(k, sl, srcSl, srcCard, sba)
}

// protMatch: the protection quality filter f matches the source, read for
// a permanent controlled as sl is.
func (s *State) protMatch(k *prep, f kop.FilterID, sl, srcSl Slot, srcCard CardID) bool {
	p := s.perms[sl].ctrl
	if srcSl != NoSlot && s.occ.has(srcSl) {
		return s.matchPermGen(k, f, srcSl, p, NoSlot, layerFull)
	}
	return s.matchCardGen(k, f, srcCard, p, p, NoObj)
}

// protGranted: a Continuous static's ProtFrom quality, granted to sl,
// matches the source (CR 702.16; Sword of Wealth and Power's instants and
// sorceries). A grant is a layer-6 effect, so it does not survive a later
// loss of sl's abilities (the rule kwGen uses). Cold: protQuality calls it
// behind genPrep.anyProtGrant.
//
//go:noinline
func (s *State) protGranted(k *prep, sl, srcSl Slot, srcCard CardID, sba bool) bool {
	lts, lost := s.lossAt(k, sl)
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.ProtFrom == 0 && !a.Static.ProtChosen {
			return true
		}
		if lost && s.grantTs(st) < lts {
			return true
		}
		if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			return true
		}
		if !(a.Static.ProtFrom != 0 && s.protMatch(k, a.Static.ProtFrom, sl, srcSl, srcCard) || a.Static.ProtChosen && s.protChosenHit(k, st.sl, srcSl, srcCard)) {
			return true
		}
		if sba && a.Static.ProtExc != 0 && srcSl != NoSlot && s.occ.has(srcSl) &&
			s.matchPermGen(k, a.Static.ProtExc, srcSl, st.ctrl, st.sl, layerFull) {
			return true // the grant's exception spares this attachment (CR 702.16)
		}
		hit = true
		return false
	})
	return hit
}

// protChosenHit: the source (the permanent at srcSl, else card srcCard as a
// spell or a departed source) has the colour the static source host chose
// (a grant of protection from the chosen colour); a host with no choice
// protects from nothing.
func (s *State) protChosenHit(k *prep, host, srcSl Slot, srcCard CardID) bool {
	ch := s.perms[host].chosen
	if ch > 4 {
		return false
	}
	if srcSl != NoSlot && s.occ.has(srcSl) {
		return s.colorsGen(k, srcSl)&(kop.ColW<<ch) != 0
	}
	return k.t.Cards[srcCard].Colors&(kop.ColW<<ch) != 0
}

// prepProtGrant arms the granted-protection reads (genPrep.anyProtGrant
// for a Continuous static with a ProtFrom quality; it also opens the
// targeting, block and attach sites through anyProtF) and the protection
// replacement bit the damage sites gate on.
func prepProtGrant(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		if protGrantIn(t.Cards[i].Abilities) {
			g.anyProtGrant, g.anyProtF = true, true
			g.replBits[i] |= rbProtCol
		}
	}
	for i := range t.Emblems {
		if protGrantIn(t.Emblems[i].Abilities) {
			g.anyProtGrant, g.anyProtF = true, true
		}
	}
}

// protGrantIn reports whether any ability is a Continuous static granting
// protection from a quality.
func protGrantIn(abs []kop.Ability) bool {
	for j := range abs {
		if a := &abs[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && (a.Static.ProtFrom != 0 || a.Static.ProtChosen) {
			return true
		}
	}
	return false
}
