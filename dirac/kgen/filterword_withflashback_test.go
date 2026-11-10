package kgen

import (
	"strings"
	"testing"
)

// TestFilterWordWithFlashback: withFlashback lowers to PredWord7 /
// W7WithFlashback (gorge's printed-keyword read); the other spellings of the
// word stay unknown properties.
func TestFilterWordWithFlashback(t *testing.T) {
	const script = `Name:Flashback Word Probe
ManaCost:1 U
Types:Sorcery
A:SP$ ChangeZone | Origin$ Exile | Destination$ Hand | ValidTgts$ Card.WORD+YouOwn | SpellDescription$ probe
Oracle:probe
`
	probe := func(word string) []string {
		return lowerScript(t, "Flashback Word Probe", strings.Replace(script, "WORD", word, 1))
	}
	if rf := probe("withFlashback"); len(rf) != 0 {
		t.Fatalf("withFlashback refused: %v", rf)
	}
	for _, w := range []string{"withoutFlashback", "hasKeywordFlashback"} {
		rf := probe(w)
		if len(rf) == 0 || !strings.Contains(strings.Join(rf, "|"), "unknown property") {
			t.Errorf("%s: refusals %v, want an unknown property", w, rf)
		}
	}
}
