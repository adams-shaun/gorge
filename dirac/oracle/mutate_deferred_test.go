package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// mutateDeferredWhy is the why of the mutate deferral's refused entries (mutate,
// CR 702.140, is deferred in the kernel until 2027-01-15). The ticket that brings
// mutate back deletes this file with those entries.
const mutateDeferredWhy = "Mutate (CR 702.140) is deferred in the kernel until 2027-01-15; refused by name, gorge unchanged"

// deferredRefusal reports a refusal by policy: it claims no divergence, so
// TestRefusedEntriesStillDiverge has nothing to replay (its repro names the
// focus list fx-mutate, not a table) and scripts/regen-main.sh must not unrefuse
// it as stale. Only this deferral's exact class and why count.
func deferredRefusal(e cardset.RefusedEntry) bool {
	return e.Class == "deferred" && e.Why == mutateDeferredWhy
}

// TestDeferredRefusalIsOnlyTheMutateDeferral: the exemption from the divergence
// check reads exactly the mutate deferral, so every other entry is still replayed.
func TestDeferredRefusalIsOnlyTheMutateDeferral(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	deferred, replayed := 0, 0
	for _, e := range entries {
		if !deferredRefusal(e) {
			replayed++
			if e.Class == "deferred" {
				t.Errorf("%s: class deferred with another why %q: it is replayed as a divergence claim", e.Card, e.Why)
			}
			continue
		}
		deferred++
		if e.Repro.Pool != "fx-mutate" {
			t.Errorf("%s: a deferred entry with the repro pool %q: a playable game belongs to a divergence refusal", e.Card, e.Repro.Pool)
		}
	}
	if deferred == 0 || replayed == 0 {
		t.Fatalf("deferred=%d replayed=%d: the check needs both kinds on disk", deferred, replayed)
	}
	for _, e := range []cardset.RefusedEntry{
		{Card: "a", Class: "lockstep", Why: mutateDeferredWhy},
		{Card: "b", Class: "deferred", Why: "another deferral"},
	} {
		if deferredRefusal(e) {
			t.Errorf("%s (class %q, why %q) is exempt from the divergence check", e.Card, e.Class, e.Why)
		}
	}
}
