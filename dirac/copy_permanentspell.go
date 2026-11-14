package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A copy of a permanent spell becomes a token as it resolves (CR 111.13,
// 608.3f, 707.10f). The copy is the ofCopy stack object mintSpellCopy made;
// as it moves to the battlefield its object.of flips from ofCopy to ofToken,
// exactly as gorge's Move to the battlefield flips IsCopy to IsToken
// (apply_zone.go). No token is created: no TrigTokenCreated fires. Cold;
// reached behind genPrep.anyPermCopy (some OpCopySpell has C&2).

// DebugPermCopyNoToken skips the flip: the copy stays ofCopy on the
// battlefield (mutation test only).
var DebugPermCopyNoToken bool

// DebugPermCopySkip makes copySpell mint nothing for a permanent spell
// (mutation test only).
var DebugPermCopySkip bool

// prepPermCopy sets genPrep.anyPermCopy: some OpCopySpell may copy a
// permanent spell (kgen sets C&2 when the filter admits one). It also sets
// anyCopyElect: some copy may choose new targets (copyElect's writers).
func prepPermCopy(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpCopySpell && in.C&2 != 0 {
					g.anyPermCopy = true
				}
				if (in.Op == kop.OpCopySpell || in.Ext() == kop.ExtCopyAbility) && in.C&1 != 0 || in.Op == kop.OpDemonstrate { // seat-ok: an instruction flag bit
					g.anyCopyElect = true
				}
			}
		}
	}
}

// permCopyEnters flips the spell copy o to a token as it enters the
// battlefield. A permanent copy of a spell is always ofCopy here; a real
// permanent card (of == 0) and a token are left alone.
//
//go:noinline
func (s *State) permCopyEnters(o Obj) {
	if DebugPermCopyNoToken {
		return
	}
	ob := &s.objs[o]
	if ob.of&(ofCopy|ofToken) == ofCopy {
		ob.of = ob.of&^ofCopy | ofToken
	}
}

// permCopyBlocked reports whether the copy of the stack entry ce cannot be
// modelled: a permanent spell of a two-faced card whose back face the copy
// could carry (CR 707.10g's double-faced copy). It stalls the game
// (StallCopyFace) and returns true. An omen card is not two-faced (CR
// 720.2c): off the stack, and on the stack not as an Omen, it has only its
// normal characteristics (CR 720.4), so the copy of its permanent spell is
// an ordinary copy whose token is the front face (CR 707.10f).
//
//go:noinline
func (s *State) permCopyBlocked(k *prep, ce *stackEntry) bool {
	if DebugPermCopySkip && k.cards[ce.card].types&(kop.TInstant|kop.TSorcery) == 0 {
		return true
	}
	if k.cards[ce.card].types&(kop.TInstant|kop.TSorcery) != 0 {
		return false
	}
	switch k.t.Cards[ce.card].FaceMode {
	case kop.FaceNone, kop.FaceOmen, kop.FacePrototype:
		return false
	}
	s.stall(StallCopyFace)
	return true
}

// copyRetargeted: the copy's new-target answer records its picks as gorge's
// recordChosenTargets does (one TargetsChosen per pick), so each pick that is
// a permanent fires its becomes-target triggers and its ward (CR 702.21a),
// kept inherited picks included. The queued triggers reach the stack above the
// copy once the resolution that made it ends. Cold: reached only from a spell
// copy's new-target election.
//
//go:noinline
func (s *State) copyRetargeted(k *prep, picks []Obj) {
	e := &s.stack[s.sp-1]
	for _, o := range picks {
		s.targetedOne(k, e, TCode(o))
	}
}
