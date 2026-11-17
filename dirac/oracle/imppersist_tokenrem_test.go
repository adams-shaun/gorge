package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestImpPersistTokenRememberMutation: Saproling Burst's token remembers its
// creator (RememberSource$) and its P/T is the creator's fade counters. With
// the per-token smRem record dropped the kernel's token reads 0/0 and dies,
// so it must diverge from gorge.
func TestImpPersistTokenRememberMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugSrcMemNoRem = true
	defer func() { dirac.DebugSrcMemNoRem = false }()
	run := runFixtureLockstep(t, "imppersist", impPersistDecks, map[string]bool{"Saproling Burst": true}, 120, false, nil)
	run.requireDivergence(t, "the token does not remember its creator")
}
