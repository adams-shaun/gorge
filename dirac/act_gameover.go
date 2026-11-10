package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// winGame is OpLoseGame with C 1 (DB$ WinsGame, CR 104.2b; gorge effWinsGame
// -> playerWins). p wins unless the game is over, p already lost, or an
// opponent of p controls an "opponents can't win" replacement (gorge
// gameWinPrevented). The win ends the game by marking the other player lost
// directly: gorge's win does not pass through the loser's "can't lose"
// (playerLoses), so s.lose is not used.
func (s *State) winGame(k *prep, p uint8) {
	if s.over || s.pl[p].lost {
		return
	}
	if k.gen != nil {
		blocked := false
		s.eachStatic(k, func(st statSrc) bool {
			if st.a.Kind == kop.AbReplacement && st.a.Repl.Kind == kop.ReplOppsCantWinGame && st.ctrl != p {
				blocked = true
			}
			return !blocked
		})
		if blocked {
			return
		}
	}
	if MaxSeats > 2 {
		for q := uint8(0); q < s.nSeats(); q++ {
			if q != p {
				s.pl[q].lost = true // each other player loses (CR 104.2b)
			}
		}
	} else {
		s.pl[1-p].lost = true
	}
	s.checkOver()
}
