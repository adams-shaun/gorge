package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestOracleSnapshotOmitsPhasedOutPermanents pins CR 702.26b in the checkpoint
// writer: a phased-out permanent keeps its battlefield zone membership but is
// treated as though it does not exist, so the snapshot must not name it. The
// phase-out and phase-in come from a real card (Divine Smite), not a hand-set
// flag.
func TestOracleSnapshotOmitsPhasedOutPermanents(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	runScenario := func(raw string) *oracleRun {
		t.Helper()
		sc, err := decodeOracleScenario([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		fails, transcript, run := runOracleScenario(reg, sc)
		if len(fails) != 0 {
			t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
		}
		return run
	}
	// The two permanent slots are Grizzly Bears and Hill Giant; Divine Smite
	// phases the Giant out. The second scenario drives on to p1's next untap
	// step, where the Giant phases back in.
	head := `{
  "name": "snapshot-phased-out", "cr": ["702.26b"], "why": "snapshot omits a phased-out permanent",
  "setup": {
    "p0": {"hand": ["Divine Smite"]},
    "p1": {"battlefield": ["Grizzly Bears", "Hill Giant"]}
  },
  "steps": [
    {"op": "cast", "seat": 0, "card": "p0:Divine Smite", "mana": "WC", "targets": ["p1:Hill Giant"]},
    {"op": "resolve", "seat": 0}`
	phaseIn := `,
    {"op": "pass_to", "seat": 0, "step": "upkeep", "active": "p1"}]}`

	// Precondition: the resolve leaves Hill Giant a battlefield permanent that
	// carries the phased-out status; a phase-out is not a zone change.
	stuck := runScenario(head + "]}")
	if got := len(stuck.snaps); got != 3 {
		t.Fatalf("%d snapshots, want 3 (setup + cast + resolve)", got)
	}
	giant := stuck.refs["p1:Hill Giant"]
	o := stuck.e.G.Obj(giant)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Hill Giant is not a battlefield permanent: %+v", o)
	}
	if !o.PhasedOut {
		t.Fatal("precondition: Hill Giant was not phased out by Divine Smite")
	}

	after := stuck.snaps[2]
	if _, ok := snapPerm(after, "p1:Grizzly Bears"); !ok {
		t.Fatal("phased-in Grizzly Bears missing from the snapshot")
	}
	if _, ok := snapPerm(after, "p1:Hill Giant"); ok {
		t.Fatal("snapshot listed the phased-out Hill Giant")
	}

	// It phases in at its controller's next untap step, and both permanents
	// are then on the board the snapshot reports.
	back := runScenario(head + phaseIn)
	if got := len(back.snaps); got != 4 {
		t.Fatalf("%d snapshots, want 4 (setup + 3 steps)", got)
	}
	o = back.e.G.Obj(back.refs["p1:Hill Giant"])
	if o == nil || o.PhasedOut {
		t.Fatal("Hill Giant did not phase back in at p1's untap step")
	}
	final := back.snaps[3]
	if _, ok := snapPerm(final, "p1:Grizzly Bears"); !ok {
		t.Fatal("Grizzly Bears missing after Hill Giant phased back in")
	}
	if _, ok := snapPerm(final, "p1:Hill Giant"); !ok {
		t.Fatal("Hill Giant missing after phasing back in")
	}
}
