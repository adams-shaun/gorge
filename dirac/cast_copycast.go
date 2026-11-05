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
// spell counts as cast. Leaving the stack the copy rests in exile and ceases
// to exist there (CR 707.10a), as every ofCopy spell does (spellRestExiles,
// then ceaseTokensGen).
//
// The kernel mints the same ofCopy object (newToken, owner = the caster) at
// the freeQ drain, in zone ZoneNone; pushBuild reads "ofCopy in ZoneNone" as
// that holding zone (copyCastFrom). A permanent card's copy resolves into a token
// (CR 707.10f: permCopyEnters, as OpCopySpell's). A paid cast whose cost the
// window cannot meet reverses (CR 733.1): its window taps stay and the copy
// ceases (CR 704.5d), as gorge's does. Not modelled, stalling loud
// (StallCopyCast): a multi-face card other than an Adventure card (whose
// front face is cast: gorge's Play never offers the Adventure, CR 715.3).

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
	if cd.FaceMode != kop.FaceNone && !(cd.FaceMode == kop.FaceAdventure && cd.Face == 0) {
		s.capFull(StallCopyCast)
		return false
	}
	if s.nearCapacity(k) {
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

// copyCastTargetAbort reports whether the paid copy cast being built can no
// longer be completed when its target menu would open: the cost is unpayable
// from the pool and the CR 601.2g window. Gorge's affordableTargetCandidates
// probes that window and withholds every candidate, so a required target
// reverses the cast (cast_begin.go's "no legal target", CR 601.2c, 733.1)
// before the menu is asked. The offer of a copy cast (a Play ability's
// resolution) is not priced, so the kernel reaches the menu with a cost it
// cannot meet; a permission's cast is lowered (the window pays it), and the
// census reads the same either way. Convoke's census is convokeTargetAbort's.
func (s *State) copyCastTargetAbort(k *prep) bool {
	if k.gen == nil || !k.gen.anyCopyCast {
		return false
	}
	e := &s.bld().e
	if e.kind != seSpell || s.objs[e.obj].of&(ofCopy|ofToken) != ofCopy || e.flags&sfFree != 0 {
		return false
	}
	cost := s.buildCost(k)
	return s.targetWindowShort(k, e, &cost, false)
}
