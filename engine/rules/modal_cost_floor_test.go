package rules

import "testing"

// TestFaceModalFloorZero is the gate on the modal-cost potential proof: only a
// Spree or Tiered face with a mode that adds nothing may run the census and
// prefix search for it (potential_plan.go potentialPlayVerdictShared), because
// only then is the printed cost the cheapest legal total (CR 702.172a/702.183a).
func TestFaceModalFloorZero(t *testing.T) {
	t.Parallel()
	for i, tc := range []struct {
		name, src string
		want      bool
	}{
		{
			"free mode",
			"K:Tiered\nA:SP$ Charm | Choices$ A1 | MinCharmNum$ 1\nSVar:A1:DB$ Draw | NumCards$ 1 | ModeCost$ 0",
			true,
		},
		{
			"free mode beside a paid one",
			"K:Spree\nA:SP$ Charm | Choices$ A1,A2 | MinCharmNum$ 1\nSVar:A1:DB$ Draw | NumCards$ 1 | ModeCost$ 2\nSVar:A2:DB$ Draw | NumCards$ 2 | ModeCost$ 0",
			true,
		},
		{
			"every mode costs mana",
			"K:Spree\nA:SP$ Charm | Choices$ A1,A2 | MinCharmNum$ 1\nSVar:A1:DB$ Draw | NumCards$ 1 | ModeCost$ 2\nSVar:A2:DB$ Draw | NumCards$ 2 | ModeCost$ 1",
			false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "Name:Modal Floor Spell\nManaCost:R\nTypes:Instant\n" + tc.src + "\nOracle:test\n"
			e, _, spell := newFixtureDeck(t, uint64(9700+i), src)
			o := e.G.Obj(spell)
			if o == nil || o.Face() == nil {
				t.Fatal("fixture spell has no face")
			}
			if got := faceModalFloorZero(o.Face()); got != tc.want {
				t.Fatalf("faceModalFloorZero = %v, want %v", got, tc.want)
			}
			if got, want := faceHasModeCost(o.Face()), true; got != want {
				t.Fatalf("fixture is not a modal-cost face: faceHasModeCost = %v, want %v", got, want)
			}
		})
	}
}
