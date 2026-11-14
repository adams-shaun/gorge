package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// the Tarkir Dragonstorm Sieges' as-enters mode choice
// ("As this enchantment enters, choose Abzan or Mardu"; gorge api:GenericChoice
// with SetChosenMode$ True as a K:ETBReplacement body).
//
// The program is one OpChooseBranch with C 2 (kgen etbSetChosenMode): the
// owner picks one of two modes, a decision like a mid-resolution GenericChoice
// (ChGenBranch, mode = the choice index; gorge poses KModes), asked the way
// the colour election is. The answer is the permanent's chosen byte, which a
// Siege never uses for a colour or a type (the byte is 255 until answered).
// gorge folds it into the permanent's ChosenModes (events foldChoose "mode");
// Card.ChosenMode<X> reads it (ExChosenMode, Gate{CtrMode}).

// DebugSiegeModeFlip is the mutation switch: the answer stores the other
// mode (TestSiegeLockstepMutation).
var DebugSiegeModeFlip bool

// SiegeModeAnswered counts the mode answers stored (a test's proof that its
// pool asks the choice; written only when a Siege's mode is answered).
var SiegeModeAnswered atomic.Uint64

// siegeModeStore records mode answer v on the permanent at sl.
func (s *State) siegeModeStore(sl Slot, v uint8) {
	if DebugSiegeModeFlip {
		v = 1 - v&1 // seat-ok: flips a mode bit
	}
	s.perms[sl].chosen = objWord(v)
	s.touchSl(sl)
	SiegeModeAnswered.Add(1)
}

// siegeModeRepl is runReplProgAs' OpChooseBranch C 2: the ask is queued and
// the rest of the program waits for the answer. gorge poses it once the
// permanent is on the battlefield (the queued as-enters ask), not while it is
// still a spell on the stack as it poses a colour (s.g.pre).
func (s *State) siegeModeRepl(sl Slot, c CardID, ab uint8, pc int) {
	s.queueAskAt(sl, akMode, c, ab, uint8(pc+1))
}

// siegeModeOptions poses the mode: one choice per mode, index = mode.
func siegeModeOptions(d *Decision) {
	d.Sub = ChGenBranch
	d.add(MakeAction(AkChoose, 0, 0, 0))
	d.add(MakeAction(AkChoose, 0, 0, 1))
}

// chosenModeOf is ExChosenMode: the source permanent's mode, 255 off the
// battlefield or unanswered (the gate then fails, as gorge's filter does).
func (s *State) chosenModeOf(sl Slot) int {
	if sl == NoSlot || !s.occ.has(sl) {
		return 255
	}
	return int(s.perms[sl].chosen)
}

// modeGateOK is gateOK for a Siege's mode gate.
func (s *State) modeGateOK(sl Slot, a *kop.Ability) bool {
	return s.chosenModeOf(sl) == int(a.Gate.N)
}

// isModeGrant reports whether ability ab of card c is a Siege's mode-gated
// granted trigger: gorge's abilityIndex finds no Face.Triggers entry for it
// (the trigger comes from an AddTrigger$ SVar), so the projection reports 255.
func isModeGrant(k *prep, c CardID, ab uint8) bool {
	if int(c) >= len(k.t.Cards) || int(ab) >= len(k.t.Cards[c].Abilities) {
		return false
	}
	a := &k.t.Cards[c].Abilities[ab]
	return a.Kind == kop.AbTriggered && a.Gate.Ctr == kop.CtrMode
}
