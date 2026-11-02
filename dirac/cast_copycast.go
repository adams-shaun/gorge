package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Play with CopyCard$ True (CR 707.12, 707.10a): "cast a copy of that
// card", the original staying where it is. Reached only behind
// genPrep.anyCopyCast (some OpCastFree has C bit 6).
//
// Gorge (rules/cast_begin.go beginPlay) mints the copy at the moment its cast
// begins, as a StackCopy "copy card for play" event: a new object of the
// original's face owned by the caster, IsCopy, in the temporary library
// holding zone that becomes the cast's From (so the copy is cast "from the
// library", prov and the cast ledger read it). Nothing else about the cast
// differs from an ordinary free cast: cast triggers fire (CR 707.12), the
// spell counts as cast. Leaving the stack the copy is put where its card
// would go and ceases to exist (CR 704.5d), as every ofCopy spell does
// (ceaseTokensGen).
//
// The kernel mints the same ofCopy object (newToken, owner = the caster) at
// the freeQ drain, in zone ZoneNone; pushBuild reads "ofCopy in ZoneNone" as
// that holding zone (copyCastFrom). A permanent card's copy resolves into a token
// (CR 707.10f: permCopyEnters, as OpCopySpell's). Not modelled, stalling loud
// (StallCopyCast): a multi-face card and
// a copy cast reversed after the push (CR 733.1: gorge leaves the copy in the
// holding zone).

// DebugCopyCastNoCopy casts the original itself instead of a copy (mutation
// test only).
var DebugCopyCastNoCopy bool

// prepCopyCast sets genPrep.anyCopyCast.
func prepCopyCast(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpCastFree && in.C&64 != 0 {
					g.anyCopyCast = true
					g.anyPermCopy = true // a permanent card's copy resolves into a token (CR 707.10f; copy_permanentspell.go)
				}
			}
		}
	}
}

// copyCastFrom reports whether o is a minted copy whose cast has not been
// pushed yet: it sits in no zone and stands for gorge's library holding zone.
func (s *State) copyCastFrom(o Obj) bool {
	ob := &s.objs[o]
	return ob.zone == kop.ZoneNone && ob.of&(ofCopy|ofToken) == ofCopy
}

// beginCopyCast mints the copy of original o for caster ctrl and begins its
// cast (the freeQ drain's copy arm). It reports whether a cast began; false
// (a loud stall, or the original is gone) lets the drain take the next card.
//
//go:noinline
func (s *State) beginCopyCast(k *prep, o Obj, ctrl uint8) bool {
	c := s.objs[o].card
	cd := &k.t.Cards[c]
	if cd.FaceMode != kop.FaceNone {
		s.capFull(StallCopyCast)
		return false
	}
	if s.nearCapacity() {
		s.capFull(StallBoard)
		return false
	}
	cp := s.newToken(c, ctrl)
	s.objs[cp].of = ofCopy
	s.objs[cp].zone = kop.ZoneNone
	if DebugCopyCastNoCopy {
		cp = o
	}
	holder := s.prio
	s.prio = ctrl // the caster (beginCast reads it)
	s.beginCast(k, cp, 0)
	s.prio = holder
	return true
}
