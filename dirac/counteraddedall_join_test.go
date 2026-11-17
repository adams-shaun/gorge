package dirac

import "testing"

// TestCounterAddedAllJoin: the watcher's batch merge folds a second matching
// recipient of one counter action into the instance already queued, adds its
// amount, and never reaches across the action's start marker.
func TestCounterAddedAllJoin(t *testing.T) {
	s := &State{}
	s.g.on = true
	s.objs[7].ver = 1
	s.npend = 1
	s.pend[0] = pendingTrig{obj: 7, card: 3, ab: 2, srcVer: 1, trigAmt: 1}
	l := trigListener{obj: 7, sl: 0, card: 3, ab: 2}

	s.g.ctrCellHit = 1 << 1 // this action began at pending index 0
	if !s.ctrAllJoin(l, 2) {
		t.Fatal("a matching recipient in the open batch did not join")
	}
	if got := s.pend[0].trigAmt; got != 3 {
		t.Fatalf("batched amount = %d, want 3", got)
	}

	s.g.ctrCellHit = 2 << 1 // a later action: the earlier instance is behind its start
	if s.ctrAllJoin(l, 2) {
		t.Fatal("joined an instance queued by an earlier action")
	}
	if got := s.pend[0].trigAmt; got != 3 {
		t.Fatalf("earlier action's amount changed to %d", got)
	}

	s.g.ctrCellHit = 0 // no open batch: every put is a batch of one
	if s.ctrAllJoin(l, 2) {
		t.Fatal("joined without an open batch")
	}

	// the mutation switch turns the merge off even inside a batch
	s.g.ctrCellHit = 1 << 1
	old := DebugCtrAllPerRecipient
	DebugCtrAllPerRecipient = true
	defer func() { DebugCtrAllPerRecipient = old }()
	if s.ctrAllJoin(l, 2) {
		t.Fatal("joined with the per-recipient mutation on")
	}
}
