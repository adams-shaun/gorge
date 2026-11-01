package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// corpusDirOrSkip is the corpus directory, or a skip when this checkout has none.
func corpusDirOrSkip(t testing.TB) string {
	t.Helper()
	return testutil.CorpusDirOrSkip(t)
}
