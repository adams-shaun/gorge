package kgen

import (
	"strings"
	"testing"
)

// TestFilterWordWithInfect: withInfect lowers to PredWord7 / W7WithInfect
// (gorge's printed-keyword read); the other spellings of the word stay
// unknown properties.
func TestFilterWordWithInfect(t *testing.T) {
	const script = `Name:Infect Word Probe
ManaCost:1 B
Types:Sorcery
A:SP$ ChangeZone | Origin$ Graveyard | Destination$ Hand | ValidTgts$ Creature.WORD+YouOwn | SpellDescription$ probe
Oracle:probe
`
	probe := func(word string) []string {
		return lowerScript(t, "Infect Word Probe", strings.Replace(script, "WORD", word, 1))
	}
	if rf := probe("withInfect"); len(rf) != 0 {
		t.Fatalf("withInfect refused: %v", rf)
	}
	for _, w := range []string{"withoutInfect", "hasKeywordInfect"} {
		rf := probe(w)
		if len(rf) == 0 || !strings.Contains(strings.Join(rf, "|"), "unknown property") {
			t.Errorf("%s: refusals %v, want an unknown property", w, rf)
		}
	}
}
