package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// manaRiderCards is the mana-rider class's 23 cards (the three shapes the
// kernel did not model: a TapsForMana extra, ManaReflected, AddsNoCounter
// mana) with where each stands. An empty reason is a card that lowers with no
// refusal; any other is the measured reason a card stays refused, which the
// lowering must still name (a substring), so a card that starts to lower or
// changes its reason fails here until this table is updated.
var manaRiderCards = []struct {
	card   string
	reason string
}{
	// lowered: ReflectProperty$ Produce over the controller's own permanents
	// (ColorOrType$ Type adds {C}; mana_reflected.go produceColours)
	{"Reflecting Pool", ""},
	{"Star Compass", ""},
	{"Gond Gate", ""},
	// ManaReflected lowered by dirac-manareflected-leftovers; Incubation
	// Druid's Amount$ is the reflected Amount$ gap class (mana_reflamt.go)
	{"Horizon of Progress", ""},
	{"Incubation Druid", ""},
	{"Cactus Preserve", "AB$ Animate"},
	{"The Grey Havens", ""},
	// AddsNoCounter$ True beside a spell restriction: the restricted batch is
	// its own kind and the payment stamps the spell (dirac mana_nocounter.go)
	{"Cavern of Souls", ""},
	{"Delighted Halfling", ""},
	// AddsNoCounter$ !Permanent: a legendary deferred {C} source that pays 2
	// life (dirac mana_deferred.go, mana_nocounter.go)
	{"Boseiju, Who Shelters All", ""},
	// TapsForMana: a colour chosen at tap time lowers as a StTapAnyCol
	// static beside a class-only Taps line (dirac/mana_tapextra.go); Regal
	// Behemoth's monarch gate on the trigger is outside the closed world
	{"Fertile Ground", ""},
	{"Regal Behemoth", "CheckDefinedPlayer"},
	{"Trace of Abundance", ""},
	{"Dawn's Reflection", ""},
	{"Verdant Haven", ""},
	{"Market Festival", ""},
	// Produced$ Chosen: the source's as-enters colour, gorge defers the
	// enchanted source (a StTapChosen static beside a class-only Taps line)
	{"Utopia Sprawl", ""},
	// Produced$ G | Amount$ X (Count$Valid Elf): a StTapDynamic static beside
	// a class-only Taps line, the extra read at the tap
	{"Elvish Guidance", ""},
	// TapsForMana triggers beyond a mana extra: a token for an opponent, a
	// doubled land, a conditional extra
	{"Forbidden Orchard", "trigger mode TapsForMana"},
	{"Gauntlet of Power", "trigger mode TapsForMana"},
	{"Zhur-Taa Druid", "trigger mode TapsForMana"},
}

// TestLowerManaRiders lowers every card of the mana-rider class from the
// corpus and checks it against manaRiderCards: no refusal for a lowered card,
// the named reason for a refused one.
func TestLowerManaRiders(t *testing.T) {
	cardsDir := ratchetCards(t)
	var names []string
	for _, c := range manaRiderCards {
		names = append(names, c.card)
	}
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		byName[c.Name] = c
	}
	for _, want := range manaRiderCards {
		c := byName[want.card]
		if c == nil {
			t.Errorf("%s: not lowered (missing from the corpus?)", want.card)
			continue
		}
		var got []string
		for _, r := range c.Refusals {
			got = append(got, r.Node+": "+r.Reason)
		}
		all := strings.Join(got, "; ")
		switch {
		case want.reason == "" && len(got) != 0:
			t.Errorf("%s: want no refusal, got %s", want.card, all)
		case want.reason != "" && !strings.Contains(all, want.reason):
			t.Errorf("%s: want refusal containing %q, got %q", want.card, want.reason, all)
		}
	}
}
