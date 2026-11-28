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

// SiegeModeByIndex counts the answers by mode index (Rick's three flag
// branches; written beside SiegeModeAnswered).
var SiegeModeByIndex [8]atomic.Uint64

// siegeModeStore records mode answer v on the permanent at sl.
func (s *State) siegeModeStore(sl Slot, v uint8) {
	if DebugSiegeModeFlip {
		v = 1 - v&1 // seat-ok: flips a mode bit
	}
	s.perms[sl].chosen = objWord(v)
	s.touchSl(sl)
	SiegeModeAnswered.Add(1)
	SiegeModeByIndex[v&7].Add(1)
}

// siegeModeRepl is runReplProgAs' OpChooseBranch C 2: the ask is queued and
// the rest of the program waits for the answer. gorge poses it once the
// permanent is on the battlefield (the queued as-enters ask), not while it is
// still a spell on the stack as it poses a colour (s.g.pre).
func (s *State) siegeModeRepl(sl Slot, c CardID, ab uint8, pc int) {
	s.queueAskAt(sl, akMode, c, ab, uint8(pc+1))
}

// siegeModeOptions poses the mode: one choice per mode, index = mode.
func siegeModeOptions(d *Decision, n int) {
	d.Sub = ChGenBranch
	for m := 0; m < n; m++ {
		d.add(MakeAction(AkChoose, 0, 0, uint8(m)))
	}
}

// siegeModeCount is the number of modes the queued ask offers: the A of the
// OpChooseBranch that posed it (the op before the ask's resume point; two for
// a Siege). Rick's three flag branches are the same ask with A 3.
func (s *State) siegeModeCount(k *prep) int {
	if s.preAskPosed() && s.g.pre.kind == akMode {
		return siegeModeProgCount(k, s.objs[s.g.pre.obj].card) // the return's pre-entry ask: no queued ask yet
	}
	a := &s.g.ask[0]
	if a.card == NoCard || a.pc == 0 {
		return 2
	}
	prog := k.t.Cards[a.card].Abilities[a.ab].Prog
	if int(a.pc) > len(prog) || prog[a.pc-1].Op != kop.OpChooseBranch {
		return 2
	}
	return int(prog[a.pc-1].A.N)
}

// replBranchSkip is OpBranch inside an as-enters program: the block runs only
// when it is the mode the entering permanent's answer chose (the chosen byte);
// otherwise it returns the block's length to skip.
func (s *State) replBranchSkip(sl Slot, in *kop.Instr) int {
	if uint8(s.perms[sl].chosen) == uint8(in.C) {
		return 0
	}
	return int(in.A.N)
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

// siegeModeProgCount is the mode count of card c's as-enters OpChooseBranch
// C 2 (0 when it has none): the A of that op.
func siegeModeProgCount(k *prep, c CardID) int {
	if int(c) >= len(k.t.Cards) {
		return 2
	}
	for i := range k.t.Cards[c].Abilities {
		a := &k.t.Cards[c].Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplEntersProgram {
			continue
		}
		for j := range a.Prog {
			if in := &a.Prog[j]; in.Op == kop.OpChooseBranch && in.C == 2 {
				return int(in.A.N)
			}
		}
	}
	return 0
}

// returnAskKind is the as-enters choice kind of a card an until-leaves exile
// returns outside a resolution that gorge asks with the card still in exile:
// enterChoiceKind's kinds, and a Siege's mode (gorge's etb_modechoice.go poses
// the owner's choice at the entry boundary when no tape run serves the entry;
// during a resolution the body's own ask is served by the run and the kernel
// queues it after the entry).
func (s *State) returnAskKind(k *prep, c CardID) uint8 {
	if kind := s.enterChoiceKind(k, c); kind != 0 {
		return kind
	}
	if !DebugReturnModeNoPark && k.gen != nil && k.gen.has(c, gfEnterRepl) && siegeModeProgCount(k, c) != 0 {
		return akMode
	}
	return 0
}

// DebugReturnModeNoPark is the lockstep mutation: a returning Siege enters
// before its mode is asked (the queued ask after the entry, the old order).
var DebugReturnModeNoPark bool
