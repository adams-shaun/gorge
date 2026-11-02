package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// MinMaxBlocker Max$ 1 (CR 509.1b): an attacker matching a
// kop.StatMaxBlockers static can't be blocked by more than one creature.
// Gorge validates the finished declaration (ValidateMinMaxBlockers); the
// oracle's multi-pick emulation of KBlockers offers a second blocker on such
// an attacker at no pick (combat.go blockerOptions, the lure path). A floor
// above the cap (menace, Min$) leaves the attacker never offered (gorge
// blockPairScopeFor's max < min), which menaceImpossible decides.

// DebugMaxBlkOff is the lockstep mutation: no attacker is capped, so the
// kernel offers the second blocker gorge does not.
var DebugMaxBlkOff bool

// maxOneBlocker: attacker a is under a live Max$ 1 static.
func (s *State) maxOneBlocker(k *prep, a Slot) bool {
	return !DebugMaxBlkOff && s.staticFlagGen(k, kop.StatMaxBlockers, a)
}

// blockCapped: attacker a already has its one blocker and may take no more.
func (s *State) blockCapped(k *prep, a Slot) bool {
	return s.perms[a].nblock >= 1 && s.maxOneBlocker(k, a)
}
