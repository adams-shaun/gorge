package rules

import "testing"

// These two real-card pins cover ninjutsu carriers from the original gap
// report in addition to the end-to-end Walker of Secret Ways test.
func TestNinjutsuOrochiSoulReaverAndFallenShinobiFromHand(t *testing.T) {
	reg := searchTestRegistry(t)
	for _, name := range []string{"Orochi Soul-Reaver", "Fallen Shinobi"} {
		t.Run(name, func(t *testing.T) {
			card := searchCorpusCard(t, reg, name)
			if d := card.Link(); len(d) != 0 {
				t.Fatalf("link %s: %v", name, d)
			}
			if !card.Faces[0].HasKeyword("Ninjutsu") {
				t.Fatalf("precondition: %s does not print Ninjutsu in the corpus", name)
			}
			found := false
			for _, ab := range card.Faces[0].Abilities {
				if ab.Params["Keyword"] == "Ninjutsu" && ab.Params["ActivationZone"] == "Hand" {
					found = true
					if ab.Params["Destination"] != "Battlefield" || ab.Params["Tapped"] != "True" || ab.Params["Attacking"] != "True" {
						t.Errorf("%s hand ninjutsu has incorrect entry effects: %+v", name, ab.Params)
					}
				}
			}
			if !found {
				t.Fatalf("%s has no expanded hand-zone Ninjutsu ability", name)
			}
		})
	}
}
