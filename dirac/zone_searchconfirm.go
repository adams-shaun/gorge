package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// searchConfirmAsk reports the player gorge asks "search your library?" before
// a pick, and whether the ask happens at all. An Optional$ library search
// (SrConfirm on OpSearchEx) gates on the fetching player before the candidate
// list is consulted, so it asks even when no card qualifies; a declined ask
// skips the fetch, the pick and the shuffle (gorge zone_search.go's
// confirmAction and optional_library_search_confirm_test.go). The hand move
// (OpChooseMove) keeps its reveal-only condition: there is nothing to reveal
// when the hand is empty.
func (s *State) searchConfirmAsk(k *prep, in *kop.Instr, p uint8) (uint8, bool) {
	if s.g.pickF&kop.SrConfirm == 0 {
		return 0, false
	}
	switch in.Op {
	case kop.OpSearchEx:
		return s.chooserOf(k, p), true
	case kop.OpChooseMove:
		if s.g.pickF&kop.SrReveal == 0 || s.anyCand(k) {
			return s.chooserOf(k, p), true
		}
	}
	return 0, false
}
