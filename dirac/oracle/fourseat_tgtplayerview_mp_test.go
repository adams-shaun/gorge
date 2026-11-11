//go:build dirac_mp

package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestFourSeatTargetedPlayerCountLockstep plays four-seat HOB pods of Sting,
// Bilbo's Sword: its enters trigger puts a hone counter for each creature
// the targeted opponent controls (ThisTargetedPlayer$Valid, one player slot,
// the targeted player's own battlefield at any seat count). No divergence and
// no stall; counting every seat's creatures (dirac.DebugTgtPlayerCtrlAny) the
// counters differ from gorge's on some seed.
func TestFourSeatTargetedPlayerCountLockstep(t *testing.T) {
	sting := deckOf(14, "Sting, Bilbo's Sword", deckOf(8, "Little Bear", deckOf(6, "Guardian of the Halls", deckOf(12, "Forest")...)...)...)
	bears := deckOf(10, "Little Bear", deckOf(10, "Attercop", deckOf(8, "Bejeweled Warg", deckOf(12, "Forest")...)...)...)
	play := fourSeatAtkPlay(t, "std-hob", [4][]string{sting, bears, sting, bears})
	const seeds = 8
	fourSeatAtkClean(t, play, seeds)
	if !fourSeatAtkMutated(t, play, seeds, &dirac.DebugTgtPlayerCtrlAny) {
		t.Fatal("counting every seat's creatures changed no seed")
	}
}
