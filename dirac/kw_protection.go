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
// col): from one of its colours (protColFrom), or a printed K:Protection:
// <spec> quality it matches (CardDef.ProtFrom; gorge sourceHasQuality).
func (s *State) protFrom(k *prep, sl, srcSl Slot, srcCard CardID, col kop.ColorMask) bool {
	return (s.protColFrom(k, sl, col) || k.gen.anyProtF && s.protQuality(k, sl, srcSl, srcCard)) && !s.noPrev(k) // CR 615.12
}

// protQuality: sl's printed K:Protection:<spec> filter matches the source
// (the permanent at srcSl, else card srcCard as a spell or a departed
// source); none once sl lost its abilities.
//
//go:noinline
func (s *State) protQuality(k *prep, sl, srcSl Slot, srcCard CardID) bool {
	f := k.t.Cards[s.cardAt(sl)].ProtFrom
	if f == 0 || s.genLost(k, sl) {
		return false
	}
	p := s.perms[sl].ctrl
	if srcSl != NoSlot && s.occ.has(srcSl) {
		return s.matchPermGen(k, f, srcSl, p, NoSlot, layerFull)
	}
	return s.matchCardGen(k, f, srcCard, p, p, NoObj)
}
