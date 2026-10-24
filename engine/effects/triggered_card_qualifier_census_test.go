package effects

import (
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// triggeredCardQualifiedCarriers is the exact corpus set ("card/SVar") whose
// Defined$ is a dotted Triggered-card-family selector
// (`TriggeredCardLKICopy.Spider`), resolved by triggeredCardQualified. Each
// member carries a behavioural test: Aunt May ->
// rules TestAuntMayNonSpiderEntrantGetsNoCounter /
// TestAuntMaySpiderEntrantGetsTheCounter. A new carrier fails here until it
// is added with a test of its own.
var triggeredCardQualifiedCarriers = []string{
	"Aunt May/DBPutCounter",
}

// TestTriggeredCardQualifiedDefinedCensus walks every corpus SVar and pins the
// dotted Triggered-card Defined$ carriers, asserting each base is a
// Triggered-card family spelling the qualified resolver accepts.
func TestTriggeredCardQualifiedDefinedCensus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	var got []string
	for _, card := range reg.AllCards() {
		for _, face := range card.Faces {
			for name := range face.SVars {
				sa := cards.ResolveSVar(face.SVars, name)
				if sa == nil {
					continue
				}
				spec := strings.TrimSpace(sa.Params["Defined"])
				base, _, dotted := strings.Cut(spec, ".")
				if !dotted || !strings.HasPrefix(base, "Triggered") {
					continue
				}
				code := definedSpecCodes.Code(base)
				if code != definedSpecTriggeredCard && code != definedSpecTriggeredAttacker {
					continue
				}
				got = append(got, face.Name+"/"+name)
			}
		}
	}
	sort.Strings(got)
	got = slices.Compact(got)
	want := slices.Clone(triggeredCardQualifiedCarriers)
	sort.Strings(want)
	if len(got) == 0 {
		t.Fatal("census found no carrier: corpus missing or Aunt May's selector no longer parses")
	}
	if !slices.Equal(got, want) {
		t.Fatalf("dotted Triggered-card Defined$ carriers = %v, want %v", got, want)
	}
}
