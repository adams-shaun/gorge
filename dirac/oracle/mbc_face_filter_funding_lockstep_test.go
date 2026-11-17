package oracle

import "testing"

// CR 601.2g: while casting a spell the caster may activate mana abilities,
// so Horn of Valhalla's Adventure face (Ysgard's Call, {X}{W}{W}) is castable
// with its second {W} made by activating a Heap Gate's "{1}, {T}: Add one mana
// of any color" (the {1} paid with an Island's mana). Gorge's potential
// planner reaches that cast: the mode-cast play is a prefix-scripted play of
// its PotentialPaymentPlans (engine/rules potential_plan.go, potential_script.go),
// offered as prScript. The kernel priced a face route only from the plain
// census (dirac face_castface.go faceCastable's castAffordsR over availFor;
// its filter route, faceReachable, had no caller on the face path), so it
// offered nothing where gorge offered the scripted castface -- pair 54 of the
// tail-21 pool, decision 412 of the recording (dirac-adventure-cast-through-
// filter-source).
//
// The filter-aware face route landed upstream after the recording (the priced
// routes read the same mana census an ordinary cast uses, filter scripts
// included), so the game replays clean now: gorge still offers the
// filter-funded castface, and the kernel offers it too. The pin keeps it so --
// a face census that loses its filter route diverges here again.
func TestMBCFaceFilterFundingLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "horn_of_valhalla_adventure_cast_through_filter")
}
