package rules

import (
	"regexp"
	"testing"

	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// entryReturnOracle is the rules-text spelling of the shape entryLandReturn
// reads from a script: an enters trigger that returns a land, directly or as
// the price of not sacrificing the land.
var entryReturnOracle = regexp.MustCompile(`(?i)when [^.]* enters, (sacrifice it unless you )?return [^.]*you control to its owner's hand`)

// TestEntryLandReturnCensus holds the script reader and the printed text to the
// same set of lands across the corpus, both ways.
func TestEntryLandReturnCensus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	found := 0
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			if !f.IsLand() {
				continue
			}
			read, printed := costvocab.EntryLandReturn(f) != "", entryReturnOracle.MatchString(f.Oracle)
			if read != printed {
				t.Errorf("%s: script reader %v, rules text %v: %q", f.Name, read, printed, f.Oracle)
			}
			if read {
				found++
			}
		}
	}
	t.Logf("%d lands return a land on entering", found)
	if found == 0 {
		t.Fatal("no land in the corpus has the shape")
	}
}
