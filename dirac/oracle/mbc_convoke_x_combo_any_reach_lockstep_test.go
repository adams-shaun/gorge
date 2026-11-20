package oracle

import "testing"

// CR 601.2b with CR 702.51a and 601.2g: casting a convoke spell with an {X}
// in its cost (Worldsoul Colossus, X{G}{W}) asks X after the convoke picks
// are announced, and each announced generic pick taps rather than paying one
// generic mana of the total cost, so an X that leaves a pick no generic mana
// to replace is not offered (gorge xAsk's convokeAbsorbs, the kernel's
// convokeXCost fold). The X the picks do leave payable is bounded by the cast
// window's proven reach (CR 601.2g): a Combo Any Amount 2 source (Sage of the
// Maze) selects one colour per mana unit as it resolves, so one activation
// makes {G}{W} and X=1 is payable with no untapped land in sight. The gorge
// side once priced that source's window alternatives as single-colour
// productions only, found no X payable, and posed its fallback X=0 (an
// announcement the announced pick cannot settle) while the kernel offered
// X=1 — tail-36-c001 pair 2, decision 260; the same shape with four picks
// parts at X=0 vs X=4 (pair 52, Descendant of Kiyomaro).
func TestMBCChampionOfLambholtReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "convoke_x_combo_any_reach")
}

// TestMBCConvokeXComboAnyReachTail36Lockstep replays every tail-36 recording
// by glob (TestReproGlobMatchesTable requires each table pool to be globed by
// some test); today that is the one recording above.
func TestMBCConvokeXComboAnyReachTail36Lockstep(t *testing.T) {
	t.Parallel()
	stdReprosGlobPools(t, "tail-36")
}
