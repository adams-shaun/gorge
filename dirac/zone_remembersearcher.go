package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// remSearcherState is the remembered-searcher bookkeeping of one resolution:
// mask holds the seats that searched (bit p for seat p), pend the searcher a
// kop.ExtRememberSearcher just armed (seat + 1, 0 none). The optional search's
// confirmation decides pend: an accepted (or unasked) search commits it to
// mask, a declined one drops it. A cold region, so the fixed State layout does
// not grow (TestNoNewFixedStateFields); the mask is not hashed and survives
// ClearRemembered$, which clears the object set alone.
type remSearcherState struct {
	mask uint8
	pend uint8
}

var coldRemSearchers = newCold[remSearcherState]("remSearchers", func(g *genPrep) int {
	if g.anyRemSearcher {
		return 1
	}
	return 0
})

// rememberSearcher is kop.ExtRememberSearcher (gorge effSearchLibrary:
// RememberSearcher$ remembers the player who searches, found card or not). The
// kernel cannot hold a player in the remembered set, so the searcher joins the
// remembered count (genState.remCnt) and, when a following Shuffle reads the
// seats, the cold mask. The count is added here, as before, so a marker with no
// search behind it still counts; the optional search's confirmation drops both
// again when it is declined (rememberSearcherDrop). Kgen emits the marker only
// after the search's confirm-or-not point, so a successful search commits it
// (rememberSearcherCommit).
//
//go:noinline
func (s *State) rememberSearcher(k *prep, in *kop.Instr) {
	s.g.remCnt++
	c := coldRemSearchers.of(s)
	if len(c) == 0 {
		return
	}
	p := s.playerRef(k, in.Obj)
	if in.Obj == kop.RefNone {
		p = s.res.e.ctrl // the marker with no player ref names the controller (the old spelling)
	}
	if p >= MaxSeats {
		c[0].pend = 0
		return
	}
	c[0].pend = p + 1
}

// rememberSearcherCommit marks the armed searcher's seat once its search
// actually starts (after an accepted Optional$ confirmation, or immediately
// when none is asked).
func (s *State) rememberSearcherCommit() {
	c := coldRemSearchers.of(s)
	if len(c) == 0 || c[0].pend == 0 {
		return
	}
	c[0].mask |= 1 << (c[0].pend - 1)
	c[0].pend = 0
}

// rememberSearcherDrop forgets an armed searcher whose search did not happen
// (the Optional$ confirmation was declined, or the search's player is gone),
// taking back the remembered count the marker added.
func (s *State) rememberSearcherDrop() {
	c := coldRemSearchers.of(s)
	if len(c) == 0 || c[0].pend == 0 {
		return
	}
	c[0].pend = 0
	if s.g.remCnt > 0 {
		s.g.remCnt--
	}
}

// shuffleRememberedSearchers is OpShuffle with C 2 (gorge effShuffle over
// Defined$ Player.IsRemembered): each distinct seat that searched shuffles
// once, in the order the search loop visited them (the remembered set's
// sacrifice order, seat 0's permanents then each later seat's). The mask is
// cleared behind it, so a second Shuffle of the set shuffles nobody.
//
//go:noinline
func (s *State) shuffleRememberedSearchers() {
	c := coldRemSearchers.of(s)
	if len(c) == 0 || c[0].mask == 0 {
		return
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if c[0].mask&(1<<p) == 0 {
			continue
		}
		s.shuffleLib(p)
	}
	c[0].mask = 0
}
