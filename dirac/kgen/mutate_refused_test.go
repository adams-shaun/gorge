package kgen

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// mutateDeferredUntil is the date mutate (CR 702.140) work is deferred to: until
// then the kernel refuses every Mutate card by name and gorge keeps playing them.
// The ticket that brings mutate back deletes this test with the refused entries.
// No test reads a clock.
const mutateDeferredUntil = "2027-01-15"

// TestMutateCardsAreRefusedUntil2027: every corpus card with a Mutate keyword on
// any face has a dirac/support/refused entry.
func TestMutateCardsAreRefusedUntil2027(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	var names []string
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			if slices.ContainsFunc(f.Keywords, func(k string) bool {
				name, _, _ := strings.Cut(k, ":")
				return name == "Mutate"
			}) {
				// A refused entry names a card by its first face.
				names = append(names, c.Faces[0].Name)
				break
			}
		}
	}
	slices.Sort(names)
	if len(names) == 0 {
		t.Fatal("the corpus registry has no Mutate card: the check below would read nothing")
	}
	s, err := readSupport(os.DirFS(testRoot))
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for _, n := range names {
		if s.refusedFor(n) == nil {
			missing = append(missing, n)
		}
	}
	t.Logf("mutate cards=%d refused=%d", len(names), len(names)-len(missing))
	if len(missing) > 0 {
		t.Errorf("%d Mutate cards have no refused entry (mutate is deferred until %s; refuse each with kgen -refuse):\n%s",
			len(missing), mutateDeferredUntil, strings.Join(missing, "\n"))
	}
}
