package effects

import "testing"

// TestCompiledPredicateDefiniteResultsMatchText guards the compiler's safety
// contract: a local predicate it claims definite must agree with the existing
// matcher, while unsupported grammar remains a textual fallback.
func TestCompiledPredicateDefiniteResultsMatchText(t *testing.T) {
	g, ids := board(t)
	ps := CompilePredicatePrograms([]string{
		"Creature.YouCtrl+untapped",
		"Land,Artifact",
		"Creature.UnknownPredicate",
	})
	sc := SpecContext{You: 0, Source: ids["myBear"]}
	for _, tc := range []struct {
		spec string
		want PredicateResult
	}{
		{"Creature.YouCtrl+untapped", PredicateYes},
		{"Land,Artifact", PredicateNo},
		{"Creature.UnknownPredicate", PredicateMaybe},
		{"Creature.notCompiled", PredicateMaybe},
	} {
		got := ps.Evaluate(tc.spec, g, g.Obj(ids["myBear"]), sc)
		if got != tc.want {
			t.Errorf("Evaluate(%q) = %v, want %v", tc.spec, got, tc.want)
		}
		if got != PredicateMaybe && (got == PredicateYes) != MatchesSpecCtx(g, tc.spec, ids["myBear"], sc) {
			t.Errorf("Evaluate(%q) definite result differs from textual matcher", tc.spec)
		}
	}
}

func TestCompiledPredicateInputOrderDoesNotChangePrograms(t *testing.T) {
	a := CompilePredicatePrograms([]string{"Land,Artifact", "Creature.YouCtrl", "Land,Artifact"})
	b := CompilePredicatePrograms([]string{"Creature.YouCtrl", "Land,Artifact"})
	if a.Len() != b.Len() || a.Len() != 2 {
		t.Fatalf("compiled program counts = %d and %d, want 2", a.Len(), b.Len())
	}
}

func TestCompiledPredicateUnknownBaseFallsBack(t *testing.T) {
	g, ids := board(t)
	ps := CompilePredicatePrograms([]string{"UnmodeledBase"})
	if got := ps.Evaluate("UnmodeledBase", g, g.Obj(ids["myBear"]), SpecContext{You: 0}); got != PredicateMaybe {
		t.Fatalf("unknown base result = %v, want PredicateMaybe", got)
	}
}

func TestMatchesSpecCtxUsesPredicateProgramsWithTextFallback(t *testing.T) {
	g, ids := board(t)
	ps := CompilePredicatePrograms([]string{
		"Creature.YouCtrl+untapped",
		"Land,Artifact",
		"Creature.UnknownPredicate",
	})
	sc := SpecContext{You: 0, Source: ids["myBear"], PredicatePrograms: ps}
	for _, tc := range []struct {
		spec string
		want bool
	}{
		{"Creature.YouCtrl+untapped", true},
		{"Land,Artifact", false},
		{"Creature.UnknownPredicate", false},
	} {
		if got := MatchesSpecCtx(g, tc.spec, ids["myBear"], sc); got != tc.want {
			t.Errorf("MatchesSpecCtx(%q) = %v, want %v", tc.spec, got, tc.want)
		}
	}
}
