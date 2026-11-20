package dirac

import "testing"

// synthMoleManScen builds p0's board from names and puts Mole Man, Moloid
// Master ({2}{G}) in p0's hand. None of the lands listed makes green on its
// own, so the cast is offered only through the combination filter (Baxter
// Building's {4}, {T}: four mana in any combination of colours).
func synthMoleManScen(t *testing.T, names ...string) (*scen, Obj) {
	t.Helper()
	all := append([]string{"Mole Man, Moloid Master"}, names...)
	sc, id := synthScen(t, "std-msh", all...)
	for _, n := range names {
		sc.perm(id[n], 0)
	}
	return sc, sc.hand(id["Mole Man, Moloid Master"], 0)
}

// TestBaxterFilterPaysGreenBesideRestrictedSource is the std-msh lockstep
// divergence: a board holding Baxter Building's combination filter beside
// Castle Doom's artifact-only any-colour ability and Hell's Kitchen's {B}/{R}.
// Gorge's prefix search offers Mole Man's {2}{G} through the filter (its phased
// fee order floats the funding before the paid answer); the kernel must too
// (CR 601.2g-h, 605.3a).
func TestBaxterFilterPaysGreenBesideRestrictedSource(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lands []string
	}{
		{"filterAlone", []string{"Swamp", "Swamp", "Swamp", "Swamp", "Baxter Building"}},
		{"besideRestricted", []string{"Swamp", "Swamp", "Swamp", "Swamp", "Baxter Building", "Castle Doom", "Hell's Kitchen"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, mole := synthMoleManScen(t, tc.lands...)
			if acts := sc.castOptions(mole); len(acts) == 0 {
				t.Fatal("Mole Man is not offered through Baxter Building's combination filter")
			}
		})
	}
}
