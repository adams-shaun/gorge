package cards

import "testing"

// A granted "MustBeBlockedBy <filter>" line is a CR 613.11 game-rule effect
// like every sibling spelling: a later "loses all abilities" cannot end it.
func TestMustBeBlockedByFilterIsHiddenRuleEffectKeyword(t *testing.T) {
	for _, k := range []string{
		"MustBeBlockedBy Creature", "MustBeBlockedBy Creature.Dalek",
		"MustBeBlockedBy Creature.Eldrazi", "MustBeBlockedByAll:Creature.Wall",
	} {
		if HiddenKeywordOf(k) == HiddenKeywordNone {
			t.Fatalf("%q must be recognised as a hidden keyword", k)
		}
		if !IsHiddenRuleEffectKeyword(k) {
			t.Errorf("%q must survive an ability removal (CR 613.11)", k)
		}
	}
}
