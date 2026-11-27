package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// RepeatCztStats counts the batched RepeatEach Token/Copy shape as it runs,
// so a lockstep test can prove the new lowering fired rather than passing on
// games where it never resolved (the granted-toxic pattern).
type RepeatCztStats struct {
	Ordered int // a ChooseOrder$ order was asked over more than one subject
	Tokens  int // a body instruction created a token
	Copies  int // a body instruction copied a permanent
	Fights  int // a body instruction fought the repeated card with its token
	Exiles  int // a DpMany delayed program exiled its whole set
}

// DebugRepeatCztStats is the test-only counter sink; nil in production.
var DebugRepeatCztStats *RepeatCztStats

// DebugRepeatEachCztOff is the mutation switch: the batched body runs no
// passes, so the new lowering's tokens, copies and fights never happen.
var DebugRepeatEachCztOff bool

// DebugRepeatEachCztExileOff is the narrow mutation switch: the DpMany
// end-of-combat program exiles nothing, so a batched CopyPermanent's copies
// stay on the battlefield while gorge exiles them (Mirror-Style Master).
var DebugRepeatEachCztExileOff bool

// repeatCztPass counts one body instruction of a batched RepeatEach pass.
func (s *State) repeatCztPass(in *kop.Instr) {
	st := DebugRepeatCztStats
	if st == nil {
		return
	}
	switch in.Op {
	case kop.OpCreateTokenEx:
		st.Tokens++
	case kop.OpCopyAsTokenEx:
		st.Copies++
	case kop.OpFight:
		if in.Obj == kop.RefEach && in.C == int16(kop.RefRemembered) {
			st.Fights++
		}
	}
}
