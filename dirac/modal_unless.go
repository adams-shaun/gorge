package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A triggered api:GenericChoice is lowered as a one-mode charm marked
// kop.AfCharmTrigger, each mode block opening with its UnlessCost$ (kgen
// lower_genericchoice.go). Forge's ChooseGenericEffect drops a choice whose
// cost the chooser cannot pay, and gorge's placement and resolution asks do
// the same (effects.GenericChoiceAvailable); the mode offer must not present
// one. A resolution-time OpChooseBranch gets the same gate from branchPayable.

// A marked trigger gorge placed unasked (every choice unpayable at placement:
// trigger_queue.go askTriggerModes) re-runs the gate at resolution, as effCharm
// does: once a choice has become payable gorge poses the mode ask then, and the
// kernel's resolveTop raises modalActAsk's offer, filtered the same way.

// unlessInstrPayable reports whether p can pay the unless / if-you-do cost an
// OpUnlessPaid / OpIfPaid / OpUnlessSwitched instruction names.
func (s *State) unlessInstrPayable(k *prep, in *kop.Instr, p uint8) bool {
	return s.canPayGenCapped(k, p, unlessPayKind(int(in.C&0xF), in), s.value(k, in.A), in.B.N, in.F)
}

// modeUnlessPayable reports whether the controller can pay the UnlessCost$ a
// marked GenericChoice mode block opens with. A mode whose block opens with
// no such instruction is always payable. A repeat charm's OpCharm C packs the
// instance as well as the mode, so the mark is only ever set on a single-pick
// charm; the guard keeps a future misuse from reading the wrong block.
func (s *State) modeUnlessPayable(k *prep, a *kop.Ability, m int, p uint8) bool {
	if a.Repeats() {
		return true
	}
	for i := 0; i+1 < len(a.Prog); i++ {
		in := &a.Prog[i]
		if in.Op != kop.OpCharm || int(in.C) != m {
			continue
		}
		n := &a.Prog[i+1]
		if n.Op != kop.OpUnlessPaid && n.Op != kop.OpIfPaid && n.Op != kop.OpUnlessSwitched {
			return true
		}
		return s.unlessInstrPayable(k, n, p)
	}
	return true
}

// anyModeUnlessPayable reports whether some mode of the marked charm a
// passes modeUnlessPayable for p (gorge's GenericChoiceAvailable non-empty).
func (s *State) anyModeUnlessPayable(k *prep, a *kop.Ability, p uint8) bool {
	for m := 0; m < int(a.NModes); m++ {
		if s.modeUnlessPayable(k, a, m, p) {
			return true
		}
	}
	return false
}
