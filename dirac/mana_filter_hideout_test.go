package dirac

import "testing"

// TestFilterBesideVillainHideoutPaysBlueSpell: Baxter Building's {4},{T}
// four-of-any filter funded by Forest, Forest, Asgardian Citadel and a
// Villainous Hideout tapped for its unrestricted {C} makes the {U} a
// Futurist Forge needs (CR 605.3a); the Hideout's Villain-only mana beside
// it must not keep the filter search off the board.
func TestFilterBesideVillainHideoutPaysBlueSpell(t *testing.T) {
	sc, id := synthScen(t, "std-msh", "Futurist Forge", "Baxter Building", "Forest", "Villainous Hideout", "Asgardian Citadel")
	sc.perm(id["Baxter Building"], 0)
	sc.perm(id["Forest"], 0)
	sc.perm(id["Forest"], 0)
	sc.perm(id["Villainous Hideout"], 0)
	cit := sc.perm(id["Asgardian Citadel"], 0)
	sc.s.perms[sc.slot(cit)].flags &^= pfTapped // (it enters tapped)
	c := sc.hand(id["Futurist Forge"], 0)
	if acts := sc.castOptions(c); len(acts) == 0 {
		t.Fatalf("Futurist Forge is not offered: Baxter's filter, funded by four other lands, makes its {U}")
	}
}
