package main

import (
	"math/rand/v2"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/internal/searchseat"
)

func writeOracleCheckpoint(t *testing.T) string {
	t.Helper()
	m := policynet.NewModel(policynet.TableRows, 8, 4, rand.New(rand.NewPCG(2, 3)))
	m.InitValue(3, rand.New(rand.NewPCG(4, 5)))
	m.Features = policynet.FeaturesMZOppHand
	path := filepath.Join(t.TempDir(), "oracle.bin")
	if err := m.SaveOracleCheckpoint(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// -search-oracle-checkpoint's front door: off leaves the knobs untouched (the
// bench output byte for byte), a good checkpoint lands in OracleValue, and
// every misuse is refused before any game starts.
func TestSearchOracleCheckpointFrontDoor(t *testing.T) {
	base := searchseat.Defaults()
	base.HorizonTurns = 2
	got, err := withSearchOracle("", "search", "bot", base)
	if err != nil || !reflect.DeepEqual(got, base) {
		t.Fatalf("flag off changed the knobs: %v", err)
	}

	oracle := writeOracleCheckpoint(t)
	got, err = withSearchOracle(oracle, "bot", "search", base)
	if err != nil || got.OracleValue == nil || got.OracleValue.Features != policynet.FeaturesMZOppHand || got.Value != nil {
		t.Fatalf("oracle checkpoint not loaded: %v", err)
	}

	plain := policynet.NewModel(policynet.TableRows, 8, 4, rand.New(rand.NewPCG(2, 3)))
	plain.InitValue(3, rand.New(rand.NewPCG(4, 5)))
	plainPath := filepath.Join(t.TempDir(), "plain.bin")
	if err := plain.SaveCheckpoint(plainPath); err != nil {
		t.Fatal(err)
	}
	clair := base
	clair.Clairvoyant = true
	noHorizon := base
	noHorizon.HorizonTurns = 0
	for _, tc := range []struct {
		name, path, a, b string
		knobs            searchseat.Options
		want             string
	}{
		{"no search side", oracle, "bot", "bot", base, "neither side is search"},
		{"game-end rollouts", oracle, "search", "bot", noHorizon, "-search-horizon > 0"},
		{"redacted checkpoint", plainPath, "search", "bot", base, "not an oracle"},
		{"clairvoyant ceiling", oracle, "search", "bot", clair, "Clairvoyant"},
	} {
		if _, err := withSearchOracle(tc.path, tc.a, tc.b, tc.knobs); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}

	// And through mainExit: refused before any game starts.
	defer func(p string, k searchseat.Options) { searchOracleCheckpoint, searchKnobs = p, k }(searchOracleCheckpoint, searchKnobs)
	searchOracleCheckpoint = oracle
	if code := mainExit("bot", "bot", 1, 0, 2, 0, "mono-red-goblins:mono-blue-tempo", "constructed", "text", 0,
		200, 20000, ".cards", "", false, false, "", 0, 0, "", "", "", "", ""); code == 0 {
		t.Error("-search-oracle-checkpoint without a search side must exit non-zero")
	}
}
