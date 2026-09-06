package host

// Defect A regression: a seated client crashed before turn 1 because the
// wire's turn_starts field marshalled as null. protocol.Snapshot.TurnStarts
// is tagged json:"turn_starts" with no omitempty, so a nil slice produces
// "turn_starts": null — and web/src/lib/dvr.ts spreads [...a.turnStarts],
// which throws "turn_starts is not iterable" and kills the SPA before it
// ever renders a board. Any table with a London mulligan round is the exact
// shape: the mulligan decisions run between the deal and turn 1, so the
// match start snapshot (fanout.go's snapshotFrame) carries an empty
// TurnStarts.
//
// This is a marshalling defect, so it is tested by marshalling: asserting on
// the Go slice length would not discriminate (both nil and the empty slice
// have len 0 for the same reason), but the JSON bytes do. The test drives
// the production path — snapshotFrame — and asserts the field re-marshals
// to [], not null.

import (
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/protocol"
)

func TestSnapshotTurnStartsNeverMarshalsNull(t *testing.T) {
	t.Parallel()
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(mulliganTable("t1", 1)); err != nil {
		t.Fatal(err)
	}
	m, err := r.newMatch(r.tables["t1"], 1)
	if err != nil {
		t.Fatal(err)
	}
	// Precondition that keeps this test sharp: a mulligan table's match start
	// snapshot is genuinely served before any TurnChange, so turnStarts is
	// empty here. If that ever stops being true, this test would silently
	// stop exercising the null path — so fail loudly instead.
	if len(m.turnStarts) != 0 {
		t.Fatalf("precondition: expected no turn start before turn 1, got %v", m.turnStarts)
	}

	f := r.snapshotFrame(r.tables["t1"], m)
	var snap protocol.Snapshot
	if err := f.Decode(&snap); err != nil {
		t.Fatalf("decode snapshot frame: %v", err)
	}
	raw, err := json.Marshal(snap.TurnStarts)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); got != "[]" {
		t.Fatalf("turn_starts marshalled as %s, want [] — a null breaks web/src/lib/dvr.ts", got)
	}
}
