package dirac

import "testing"

// TestFilterSearchOverAThirdRestriction: the pool names every spend
// restriction of a table (genPrep.restrF), so with Villainous Hideout's
// Villain-only {G} beside Avengers Tower's Hero-only one and Castle Doom's
// artifact-only one, the Hideout's restricted mana floats restricted, not
// unrestricted. A filter search that would fund Claim the Kingdom's {1}{G}
// through it finds nothing, as gorge's planner does (the mana cannot be spent
// on it, CR 106.6), and the kernel stalls on nothing.
func TestFilterSearchOverAThirdRestriction(t *testing.T) {
	sc, id := synthScen(t, "std-msh", "Claim the Kingdom", "Villainous Hideout", "Surveillance Room", "Plains", "Avengers Tower", "Castle Doom")
	if got := sc.k.gen.restrF; got[0] == 0 || got[1] == 0 || got[2] == 0 {
		t.Fatalf("the table's restrictions = %v, want three named", got)
	}
	sc.perm(id["Villainous Hideout"], 0)
	sc.perm(id["Surveillance Room"], 0)
	c := sc.hand(id["Claim the Kingdom"], 0)
	if acts := sc.castOptions(c); len(acts) != 0 {
		t.Fatalf("Claim the Kingdom is offered on a Villain-only source: %v", acts)
	}
	if sc.s.Stalled() {
		t.Fatal("a third restriction's mana latched a capacity stall")
	}
}

// TestRestrictedManaOfAThirdKindFloats: each of a table's spend restrictions
// floats its mana restricted beside the pool, whichever of the three it is.
func TestRestrictedManaOfAThirdKindFloats(t *testing.T) {
	sc, _ := synthScen(t, "std-msh", "Villainous Hideout", "Avengers Tower", "Castle Doom")
	for i, f := range sc.k.gen.restrF[:3] {
		var pl player
		var rx uint16
		if !pl.addRestr(sc.k, &rx, f, 4, 1) {
			t.Fatalf("restriction %d does not float", i)
		}
		if got := sc.k.gen.restrFOf(pl.poolR); got != f {
			t.Fatalf("restriction %d reads back as %d, want %d", i, got, f)
		}
	}
}
