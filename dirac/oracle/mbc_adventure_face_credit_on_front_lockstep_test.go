package oracle

import "testing"

// CR 715.4, 601.2g: Chief Engineer gives Horn of Valhalla (an artifact) convoke,
// but its Adventure (Ysgard's Call, {X}{W}{W}) has none. Gorge's potential
// planner reads the card's live (front) face for cast credit and declines the
// face route "unsupported", "cost:credit" (potential_plan.go
// potentialModeCastPlan), so only the floating pool pays it; the kernel's face
// census paid it through Heap Gate's filter script and over-offered the
// castface (decision 596 of the seed-38 cast-choosing tail).
func TestMBCAdventureFaceCreditOnFrontLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "adventure_face_credit_on_front_decline")
}
