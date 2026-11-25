package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A generated scenario's setup places the battlefield one card at a time, so a
// Mode$ Always state trigger (CR 603.8) can match mid-placement even though its
// condition is false once the whole listed battlefield is in play. XMage's
// addCard places the starting position without a sequence of real game states;
// the trigger must not survive setup and be resolved against the finished
// board. Marjhan's "When you control no Islands, sacrifice this creature" is
// the canonical case: placed before its Islands, it used to sacrifice itself
// with three Islands in play.
func TestOracleSetupStateTriggerSeesWholeSetup(t *testing.T) {
	testutil.CorpusRegistry(t)
	res, _ := runFixtureScenario(t,
		`{"name":"marjhan-islands","setup":{"p0":{"battlefield":["Marjhan","Island","Island","Island"]},"p1":{}},"steps":[]}`)
	if len(res.Snapshots) == 0 {
		t.Fatal("scenario produced no setup snapshot")
	}
	snap := res.Snapshots[0]
	// Precondition: the drive reached turn 1's first main phase, the point the
	// setup checkpoint is taken at, and the whole listed battlefield is in play,
	// so the trigger's condition is false at the checkpoint rather than the
	// scenario having placed neither Marjhan nor its Islands.
	if snap.Checkpoint != "setup" || snap.Turn != 1 || snap.Step != "main1" {
		t.Fatalf("setup checkpoint = %q turn %d step %q, want setup turn 1 main1", snap.Checkpoint, snap.Turn, snap.Step)
	}
	if setupPermCount(snap, "Marjhan") != 1 || setupPermCount(snap, "Island") != 3 {
		t.Fatalf("setup is not the whole listed battlefield: %+v", snap.Permanents)
	}
	if gy := snap.Players[0].Graveyard; len(gy) != 0 {
		t.Errorf("p0 graveyard = %v at the setup checkpoint, want empty: Marjhan's state trigger resolved from a mid-placement state with its Islands in play", gy)
	}

	// Control: with no Island the state trigger's condition holds, so Marjhan
	// is still sacrificed during the setup drive. The drop above must not make
	// a still-true Mode$ Always trigger disappear (CR 603.8 re-triggers it).
	res, _ = runFixtureScenario(t,
		`{"name":"marjhan-alone","setup":{"p0":{"battlefield":["Marjhan"]},"p1":{}},"steps":[]}`)
	if len(res.Snapshots) == 0 {
		t.Fatal("control scenario produced no setup snapshot")
	}
	snap = res.Snapshots[0]
	if snap.Checkpoint != "setup" || snap.Turn != 1 || snap.Step != "main1" {
		t.Fatalf("control checkpoint = %q turn %d step %q, want setup turn 1 main1", snap.Checkpoint, snap.Turn, snap.Step)
	}
	if setupPermCount(snap, "Marjhan") != 0 {
		t.Fatalf("control: Marjhan is still on the battlefield without an Island: %+v", snap.Permanents)
	}
	if gy := snap.Players[0].Graveyard; !slices.Equal(gy, []string{"Marjhan"}) {
		t.Errorf("control: p0 graveyard = %v, want [Marjhan]", gy)
	}
}
