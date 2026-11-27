package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoRiotShapes: the demo decks' granted-Riot cards lower with no
// refusal (CR 702.136, 702.136a). Spider-Punk prints Riot, grants it to other
// Spiders and carries the "spells and abilities can't be countered" Counter
// replacement; Uncivil Unrest and Rhythm of the Wild grant Riot from a
// Continuous static. A grant is kop.Static.Riot on the Continuous static, the
// record the kernel reads beside CardDef.Riot (dirac/kw_riot.go).
func TestDemoRiotShapes(t *testing.T) {
	names := []string{"Spider-Punk", "Uncivil Unrest", "Rhythm of the Wild"}
	lw := lowerNamedPool(t, names)
	if len(lw.cards) != len(names) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(names))
	}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r)
		}
	}
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	grants := map[string]int{}
	printed := map[string]bool{}
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
		printed[b.def.Name] = b.def.Riot
		for _, ab := range b.def.Abilities {
			if ab.Kind == kop.AbStatic && ab.Static.Kind == kop.StatContinuous && ab.Static.Riot {
				grants[b.def.Name]++
			}
		}
	}
	for _, n := range names {
		if grants[n] != 1 {
			t.Errorf("%s: %d Static.Riot grants, want 1", n, grants[n])
		}
	}
	if !printed["Spider-Punk"] {
		t.Error("Spider-Punk: CardDef.Riot is not set")
	}
}
