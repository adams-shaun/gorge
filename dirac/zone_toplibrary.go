package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// gorge's ChangeZone Defined$ TopOfLibrary Origin$
// Library Optional$ True (moveDefinedLibraryObjects). The selector reads the
// resolving controller's library: an empty one is consumed with no ask and no
// move (unlike a mill's mill_optional), otherwise the one yes / no ask is
// posed (OpMayAsk MayGuardLibraryTop) and a yes moves the top card to the
// graveyard (kop.ExtToGraveyard) or the bottom of the library (OpToLibrary
// C=1). A position read, not a search: no shuffle.

// DebugTopLibraryAlwaysMove is a test hook: the move runs even when the
// controller declined the ask. The lockstep must fail with it set.
var DebugTopLibraryAlwaysMove bool

// libTopRef is RefLibTop: the top card of the resolving controller's library,
// or NoObj (known) when it is empty.
func (s *State) libTopRef() (Obj, bool) {
	pl := &s.pl[s.res.e.ctrl]
	if pl.libLen() == 0 {
		return NoObj, true
	}
	return pl.lib[int(pl.libLo)], true
}

// libTopExists is MayGuardLibraryTop: the controller's library holds a card.
func (s *State) libTopExists() bool {
	return s.pl[s.res.e.ctrl].libLen() > 0
}

// topLibForceMove is the DebugTopLibraryAlwaysMove hook at the answer: true
// when the declined ask is the library-top one (the move then runs anyway).
func (s *State) topLibForceMove(k *prep) bool {
	return DebugTopLibraryAlwaysMove && s.prog(k)[s.res.pc].C == kop.MayGuardLibraryTop
}

// libTopMV is ExMVOf RefLibTop (gorge count_cov3.go
// evalCov3HeadTopOfLibraryCMC, Count$TopOfLibraryCMC): the mana value of the
// top card of the evaluating controller's library, 0 when it is empty.
func (s *State) libTopMV(k *prep, ctx evalCtx) int {
	if int(ctx.ctrl) >= len(s.pl) {
		return 0
	}
	pl := &s.pl[ctx.ctrl]
	if pl.libLen() == 0 {
		return 0
	}
	o := pl.lib[int(pl.libLo)]
	return s.objMV(k, s.objs[o].card, o)
}
