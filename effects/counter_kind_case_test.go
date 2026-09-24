package effects

import "testing"

// TestCanonicalCounterKindFoldsCaseVariants pins the fold PutCounter applies
// to CounterType$: the corpus's minority spellings (Stun, Quest, Verse) become
// the kind every reader uses, and case-significant kinds (keyword counters,
// P1P1, the engine's own Shield marker) pass through untouched.
func TestCanonicalCounterKindFoldsCaseVariants(t *testing.T) {
	for in, want := range map[string]string{
		"Stun":            "STUN",
		" Stun ":          "STUN",
		"STUN":            "STUN",
		"Quest":           "QUEST",
		"Verse":           "VERSE",
		"P1P1":            "P1P1",
		"Flying":          "Flying",
		"Shield":          "Shield",
		"":                "",
		"Stun,Flying":     "STUN,Flying",
		"P1P1, Verse":     "P1P1,VERSE",
		"Deathtouch,M1M1": "Deathtouch,M1M1",
	} {
		if got := canonicalCounterKind(in); got != want {
			t.Errorf("canonicalCounterKind(%q) = %q, want %q", in, got, want)
		}
	}
}
