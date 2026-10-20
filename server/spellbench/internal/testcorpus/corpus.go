// Package testcorpus shares one corpus registry across a test binary. Tests
//
// Ported from github.com/jackmaiorino/spellbench/engines/gorge/internal/testcorpus
// at revision 1beea4c104911bba77a5672c644e570ed9afe28a (MIT).
// skip when the corpus is absent, matching the rest of gorge's suites.
package testcorpus

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func Dir(t testing.TB) string {
	t.Helper()
	return testutil.CorpusDirOrSkip(t)
}

func Registry(t testing.TB) *cards.Registry {
	t.Helper()
	return testutil.CorpusRegistry(t)
}
