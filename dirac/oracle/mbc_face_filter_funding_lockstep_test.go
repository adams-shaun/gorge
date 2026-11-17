package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const hornOfValhallaFaceFilterRepro = "horn_of_valhalla_face_second_white_through_a_filter"

// CR 601.2g: while casting a spell the caster may activate mana abilities,
// so Ysgard's Call ({X}{W}{W}, Horn of Valhalla's Adventure face) is
// castable with its second {W} made by activating a Heap Gate's
// "{1}, {T}: Add one mana of any color" (the {1} paid with an Island's
// mana). Gorge's potential planner reaches that cast: the mode-cast play is
// a prefix-scripted play of its PotentialPaymentPlans (engine/rules
// potential_plan.go, potential_script.go), offered as prScript. The kernel
// prices a face route only from the plain census (dirac face_castface.go
// faceCastable's castAffordsR over availFor; its filter route,
// faceReachable, has no caller on the face path), so the kernel offers
// nothing where gorge offers the scripted castface. The full mirror -- a
// face route through the kernel's gap-script searches, with the node-ask on
// the face's own census and the witness paid from the floated pool -- is a
// new subsystem, so Horn of Valhalla (a filter-funded Adventure {X} cast)
// is refused by name and the recorded game still shows the divergence with
// it implicated.
func TestMBCFaceFilterFundingLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Horn of Valhalla" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != hornOfValhallaFaceFilterRepro {
			t.Fatalf("Horn of Valhalla's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, hornOfValhallaFaceFilterRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Horn of Valhalla has no dirac/support/refused entry: gorge prices its Adventure face cast through a filter, the kernel's face census does not")
}
