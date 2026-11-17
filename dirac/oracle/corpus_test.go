package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// corpusDirOrSkip is the corpus directory, or a skip when this checkout has
// none. A tree with no git repository (a bare export, as pr-precheck's fresh
// copy) has no corpus either: it skips rather than fails.
func corpusDirOrSkip(t testing.TB) string {
	t.Helper()
	if _, ok := testutil.SharedCorpusDir("."); !ok {
		t.Skip("no card corpus: no git repository here to find .cards")
	}
	return testutil.CorpusDirOrSkip(t)
}
