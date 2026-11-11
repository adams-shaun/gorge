package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Trigger-event and keyword classes 19-62: cold,
// out-of-line arms.

// protPrinted is the permanent at sl's printed protection-from-colour
// colours (CardDef.ProtColors; CR 702.16), none once it has lost its
// abilities (gorge protectedFrom reads the derived keywords).
//
//go:noinline
func (s *State) protPrinted(k *prep, sl Slot) kop.ColorMask {
	c := k.t.Cards[s.cardAt(sl)].ProtColors
	if c == 0 || s.genLost(k, sl) {
		return 0
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
	if f := k.t.Cards[s.cardAt(sl)].ProtFrom; f != 0 && !s.genLost(k, sl) && s.protMatch(k, f, sl, srcSl, srcCard) {
		return true
	}
	return k.gen.anyProtGrant && s.protGranted(k, sl, srcSl, srcCard)
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
func (s *State) protGranted(k *prep, sl, srcSl Slot, srcCard CardID) bool {
	lts, lost := s.lossAt(k, sl)
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.ProtFrom == 0 {
			return true
		}
		if lost && s.grantTs(st) < lts {
			return true
		}
		if s.statCond(k, st) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) &&
			s.protMatch(k, a.Static.ProtFrom, sl, srcSl, srcCard) {
			hit = true
			return false
		}
		return true
	})
	return hit
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
		if a := &abs[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.ProtFrom != 0 {
			return true
		}
	}
	return false
}
