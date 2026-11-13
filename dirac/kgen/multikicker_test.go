package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMultikickerLowersTheClass: every corpus card whose only blocker at land
// e46fc98fa8 was K:Multikicker / Count$TimesKicked lowers its per-payment cost
// and reaches the kernel's CardDef.HasMultikicker. An {X} base cost stays
// refused by name, because the kernel asks the count before the X announcement
// (kw_multikicker.go's pool-only bound).
func TestMultikickerLowersTheClass(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lowers := []string{
		"Apex Hawks", "Batroc the Leaper", "Bloodhusk Ritualist", "Deathforge Shaman",
		"Enclave Elite", "Everflowing Chalice", "Flametongue Yearling", "Gnarlid Pack",
		"Joraga Warcaller", "Lightkeeper of Emeria", "Marshal's Anthem", "Quag Vampires",
		"Skitter of Lizards", "Spell Contortion", "Voyager Drake", "Wolfbriar Elemental",
	}
	for _, name := range lowers {
		t.Run(name, func(t *testing.T) {
			lc := lowerMultikickerCard(t, reg, name)
			if len(lc.Refusals) != 0 {
				t.Fatalf("%s refusals = %v, want none", name, lc.Refusals)
			}
			if lc.Multikicker == nil {
				t.Fatalf("%s lowered without a Multikicker cost", name)
			}
		})
	}
	// Comet Storm and Strength of the Tajuru have an {X} base cost: the count
	// bound would be read before X, so they stay refused by name with a true
	// reason rather than silently lowering a shape the kernel cannot play.
	for _, name := range []string{"Comet Storm", "Strength of the Tajuru"} {
		t.Run(name, func(t *testing.T) {
			lc := lowerMultikickerCard(t, reg, name)
			refused := false
			for _, r := range lc.Refusals {
				if strings.Contains(r.Reason, "multikicker on an {X} spell") {
					refused = true
				}
			}
			if !refused {
				t.Fatalf("%s refusals = %v, want the {X} refusal", name, lc.Refusals)
			}
		})
	}
}

func lowerMultikickerCard(t *testing.T, reg *cards.Registry, name string) *lCard {
	t.Helper()
	card, ok := reg.Lookup(name)
	if !ok || card == nil {
		t.Fatalf("%s not in the corpus", name)
	}
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: reg}
	return lowerPool(p).cards[0]
}
