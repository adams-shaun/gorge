package oracle

import "testing"

// A convoke pick commits the creature it taps to the payment, so CR 601.2g's
// window cannot tap it for mana too. When the committed creature is a Combo
// Any census gap (Sage of the Maze's "{T}: Add two mana in any combination of
// colors"), its units reach the cast's X ask only through availXWindow's gap
// census, which excludePickedSources' removeSrc never takes out (the planner
// prices no unit for the gap source, so canTapForMana is false and removeSrc
// is skipped). p0 casts Worldsoul Colossus {X}{G}{W} convoking an untapped
// Sage of the Maze beside a Plains and two Forests: the announced X must be
// bounded at 2 (one Forest plus the pick's generic over the {G}{W} pips), not
// 4 (the Sage's two any-colour units still counted). pickCommittedSrc of
// dirac/cast_xpicked.go drops the pick-committed source from the window
// census's gap arms (availXWindow's, availCA's and availLifeGap's).
func TestMBCSageOfTheMazeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "convoke_gap_x_ask")
}
