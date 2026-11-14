package dirac

import "testing"

// grantCapTapXOffers lists the activations of o the priority decision offers.
func (sc *scen) grantCapTapXOffers(o Obj) (n int) {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(o)) {
			n++
		}
	}
	return
}

// TestGrantCapKeepsTapXActivation: under Mm'menon's spell-only grant on every
// artifact, an artifact's "{N}, {T}, Sacrifice" activation is paid by lands plus
// Gene Pollinator's tap-another-permanent mana (CR 605.1a, 601.2g-h). The grant
// cap's tapXScript fallback priced it with removeSrc taking the granted-only
// artifact's unit out of an availability that never held it, so the lands fell
// one short and the activation was dropped (EOE-s144, EOE-s151).
func TestGrantCapKeepsTapXActivation(t *testing.T) {
	for _, c := range []struct {
		name  string
		art   string
		deck  string // the card the table is built from (the token's maker)
		lands int
	}{
		{"torpedo six", "Thaumaton Torpedo", "Thaumaton Torpedo", 5},          // {6}: five lands and Pollinator
		{"lander token two", "c_a_lander_sac_search", "Seedship Agrarian", 1}, // {2}: one land and Pollinator
	} {
		t.Run(c.name, func(t *testing.T) {
			sc, id := synthScen(t, "std-eoe", c.deck, "Gene Pollinator", "Mm'menon, the Right Hand", "Edge Rover", "Island")
			if _, ok := id[c.art]; !ok { // a token: the card the table holds for it
				for i := range sc.k.t.Cards {
					if sc.k.t.Cards[i].Name == c.art {
						id[c.art] = CardID(i)
					}
				}
				if _, ok := id[c.art]; !ok {
					t.Fatalf("%s is not in the table", c.art)
				}
			}
			sc.lands(0, id["Island"], c.lands)
			art := sc.perm(id[c.art], 0)
			sc.perm(id["Gene Pollinator"], 0)
			sc.perm(id["Mm'menon, the Right Hand"], 0)
			sc.perm(id["Edge Rover"], 0) // the permanent Pollinator taps
			if sc.grantCapTapXOffers(art) != 1 {
				t.Fatalf("%s: want its one activation offered", c.art)
			}
		})
	}
}
