package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// a reflexive trigger's TriggerRemembered$ value.
// OpReflexiveTrigger's B is the parent-context value of the remembered
// set's Amount / CardManaCost / GreatestCardManaCost; it rides the pending
// entry's trigAmt, which the body reads as the trigger amount. The three
// are fixed once the set is, so capturing as the trigger is put on the
// stack equals gorge's read at the instance's resolution.

// DebugReflexAmtZero (test-only) captures 0 instead of B.
var DebugReflexAmtZero bool

// reflexAmt is the trigAmt a reflexive trigger is put on the stack with
// (B = the zero Const for every reflexive trigger without a captured value).
//
//go:noinline
func (s *State) reflexAmt(k *prep, b kop.ValueRef) int16 {
	if DebugReflexAmtZero {
		return 0
	}
	return int16(s.value(k, b))
}

// unlessRemembers reports an unless-cost discard that remembers what it
// discards (kop.PayBRemember; RememberDiscarded$ on the gated
// ImmediateTrigger).
func unlessRemembers(in *kop.Instr) bool {
	return (in.Op == kop.OpUnlessPaid || in.Op == kop.OpUnlessSwitched) &&
		in.C&15 == kop.PayDiscard && in.B.N == kop.PayBRemember
}

// tableRemembersUnlessDisc reports whether some program of t has an
// unless-cost discard that remembers (genPrep.anyRemDisc).
func tableRemembersUnlessDisc(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if unlessRemembers(&in) {
					return true
				}
			}
		}
	}
	return false
}
