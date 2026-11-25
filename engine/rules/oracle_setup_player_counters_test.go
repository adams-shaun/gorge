package rules

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestOracleSetupPlayerCounters restates a seat's complete player-counter
// count at the setup checkpoint (turn 1 main 1). Aethersquall Ancient's
// mandatory upkeep trigger makes 3 energy during the drive, so seeding 8 must
// add the 5-count delta, not 8 more.
func TestOracleSetupPlayerCounters(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	// Seed 42 tosses seat 0 to start, so p0's Aethersquall Ancient triggers
	// on turn 1's upkeep (precondition below).
	seat := func(counters []oraclePlayerCounter) oracleSeat {
		return oracleSeat{
			Battlefield:    []string{"Aethersquall Ancient"},
			Library:        []string{"Island", "Island", "Island"},
			PlayerCounters: counters,
		}
	}
	run := func(t *testing.T, counters []oraclePlayerCounter) *state.Game {
		t.Helper()
		sc := oracleScenario{
			Name:  "setup-player-counters",
			Setup: map[string]oracleSeat{"p0": seat(counters), "p1": {Library: []string{"Island", "Island", "Island"}}},
		}
		fails, transcript, r := runOracleScenario(reg, sc)
		if len(fails) > 0 {
			t.Fatalf("scenario failed: %v\ntranscript:\n%s", fails, strings.Join(transcript, "\n"))
		}
		return r.e.G
	}

	// Precondition: the upkeep trigger really made 3 energy during the drive,
	// so the seeded run's assertion below is about restating, not a dead card.
	plain := run(t, nil)
	if got := plain.Players[0].Counter("ENERGY"); got != 3 {
		t.Fatalf("precondition: Aethersquall Ancient's upkeep trigger made %d energy, want 3", got)
	}
	if got := plain.Players[0].Counter("POISON"); got != 0 {
		t.Fatalf("precondition: unseeded POISON = %d, want 0", got)
	}

	seeded := run(t, []oraclePlayerCounter{{Kind: "ENERGY", N: 8}})
	if got := seeded.Players[0].Counter("ENERGY"); got != 8 {
		t.Errorf("ENERGY = %d, want the complete 8 (not 8 + the 3 the upkeep trigger made)", got)
	}
	if got := seeded.Players[1].Counter("ENERGY"); got != 0 {
		t.Errorf("seat 1 ENERGY = %d, want 0", got)
	}

	// A scenario that does not set player_counters carries no key, and a
	// decode/encode round trip of one is byte-identical.
	plainSC := oracleScenario{
		Name:  "setup-player-counters",
		Setup: map[string]oracleSeat{"p0": {Battlefield: []string{"Aethersquall Ancient"}}},
	}
	first, err := json.Marshal(plainSC)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(first, []byte("player_counters")) {
		t.Errorf("a zero-value player-count seed added the key: %s", first)
	}
	decoded, err := decodeOracleScenario(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("scenario without player_counters is not byte-identical:\nfirst:  %s\nsecond: %s", first, second)
	}

	// The field is strict: an array of objects, never a map.
	bad := []byte(`{"setup":{"p0":{"player_counters":{"ENERGY":8}}}}`)
	if _, err := decodeOracleScenario(bad); err == nil {
		t.Error("a player_counters map decoded without error")
	}
}
