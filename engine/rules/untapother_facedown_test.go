package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestFaceDownSeedbornMuseUntapsNothing pins CR 708.2a on the foreign untap
// scan: a Seedborn Muse manifested by Scroll of Fate is a 2/2 with no
// abilities, so its "untap all permanents you control during each other
// player's untap step" static untaps nothing. The face-up arm untaps the bear,
// so the scan is live.
func TestFaceDownSeedbornMuseUntapsNothing(t *testing.T) {
	t.Parallel()
	e := roomEngine(t, "Seedborn Muse", "Grizzly Bears")
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	muse := moveByName(t, e, 0, "Seedborn Muse", state.ZBattlefield)
	if !e.untapOtherStaticsMatch(bear) {
		t.Fatal("precondition: a face-up Seedborn Muse does not untap the bear")
	}
	e.G.Obj(muse).FaceDown = true
	if e.untapOtherStaticsMatch(bear) {
		t.Fatal("a face-down Seedborn Muse untaps the bear during another player's untap step")
	}
}
