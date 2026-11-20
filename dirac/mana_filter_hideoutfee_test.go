package dirac

import "testing"

// CR 605.3b, 106.6: Villainous Hideout's {C} mode is unrestricted, so it pays
// Baxter Building's {4} beside three Swamps; the four mana of any colours then
// pay Death to Our Enemies' {2}{R}. Only its any-colour mode is Villain-only.
func TestHideoutColourlessPaysBaxterFee(t *testing.T) {
	for _, swamps := range []int{2, 3} {
		sc, id := synthScen(t, "std-msh", "Death to Our Enemies", "Baxter Building", "Villainous Hideout", "Swamp")
		sc.perm(id["Baxter Building"], 0)
		sc.perm(id["Villainous Hideout"], 0)
		for i := 0; i < swamps; i++ {
			sc.perm(id["Swamp"], 0)
		}
		death := sc.hand(id["Death to Our Enemies"], 0)
		d := sc.s.Step()
		if got, want := castOffered(&d, death), swamps == 3; got != want {
			t.Errorf("%d Swamps: cast offered=%v, want %v", swamps, got, want)
		}
	}
}
