package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// emptyFetchShuffle is a Defined$ Remembered library fetch (OpBounce with
// BncFromLibrary, OpReturnEx with RetFromLibrary) whose remembered list is
// empty while the script owes a shuffle (gorge zone_search.go
// emptyDefinedFetch): a preceding library search deferred its "Then shuffle"
// to this leg, and the search happened, so the library is shuffled once
// whatever it found (CR 701.24a). Nissa's Pilgrimage finding one Forest is
// the carrier: the battlefield leg takes it and DBHand's Remembered is empty.
// The searched library is the resolving controller's (every lowered carrier
// searches "your library"). A fetch with a remembered object that is no
// longer in a library still shuffles nothing, as in gorge.
func (s *State) emptyFetchShuffle(in *kop.Instr) {
	if in.Obj != kop.RefRemembered || s.g.nrem != 0 {
		return
	}
	owed := false
	switch in.Op {
	case kop.OpBounce:
		owed = in.C&kop.BncFromLibrary != 0 && in.C&kop.BncShuffleOwed != 0
	case kop.OpReturnEx:
		owed = in.B.N&kop.RetFromLibrary != 0 && in.B.N&kop.RetShuffleOwed != 0
	}
	if owed {
		s.shuffleLib(s.res.e.ctrl)
	}
}
