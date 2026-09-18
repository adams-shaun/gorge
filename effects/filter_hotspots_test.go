package effects

import "testing"

var benchmarkMatch bool

// Simple filter matching is read-only and needs no owned result storage.
// Reintroducing per-alternative or per-predicate slices breaks this budget,
// including on failed alternatives and fail-closed unknown predicates.
func TestSimpleFilterMatchingDoesNotAllocate(t *testing.T) {
	g, ids := board(t)
	for _, tc := range []struct {
		spec string
		want bool
	}{
		{"Creature", true},
		{"Creature.YouCtrl+nonLegendary", true},
		{"Land,Creature.YouCtrl", true},
		{"Land,Artifact", false},
		{"Creature.UnknownPredicate", false},
		{"Creature.UnknownPredicate,Creature.YouCtrl", true},
		{"Creature.YouCtrl+", true},
		{"", false},
	} {
		t.Run(tc.spec, func(t *testing.T) {
			allocs := testing.AllocsPerRun(100, func() {
				if got := MatchesSpec(g, tc.spec, ids["myBear"], 0); got != tc.want {
					t.Fatalf("MatchesSpec(%q) = %v; want %v", tc.spec, got, tc.want)
				}
			})
			if allocs != 0 {
				t.Fatalf("MatchesSpec(%q) allocated %.0f objects; want zero", tc.spec, allocs)
			}
		})
	}
}

func BenchmarkFilterTextualMatch(b *testing.B) {
	g, ids := board(b)
	sc := SpecContext{You: 0, Source: ids["myBear"]}
	b.ReportAllocs()
	for range b.N {
		benchmarkMatch = MatchesSpecCtx(g, "Creature.YouCtrl+tapped", ids["myBear"], sc)
	}
}

func BenchmarkFilterTextualReject(b *testing.B) {
	g, ids := board(b)
	sc := SpecContext{You: 0, Source: ids["myBear"]}
	b.ReportAllocs()
	for range b.N {
		benchmarkMatch = MatchesSpecCtx(g, "Land,Artifact", ids["myBear"], sc)
	}
}

func BenchmarkFilterTextualUnknown(b *testing.B) {
	g, ids := board(b)
	sc := SpecContext{You: 0, Source: ids["myBear"]}
	b.ReportAllocs()
	for range b.N {
		benchmarkMatch = MatchesSpecCtx(g, "Creature.UnknownPredicate", ids["myBear"], sc)
	}
}
