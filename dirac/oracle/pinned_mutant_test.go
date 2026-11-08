package oracle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestMutantRecordingNeverWritesCleanPins: PINS_MUTANT=1 promises a game the
// mutated build diverges in. The bounded scan below finds none on the fixed
// build, so recording must fail and write nothing; before the fix it recorded
// three ordinary clean games that pass on the mutated tree.
func TestMutantRecordingNeverWritesCleanPins(t *testing.T) {
	t.Setenv("PINS_MUTANT", "1")
	old := pinScan
	pinScan = 19 // pairs 0, 12 and 18 record clean games here; none diverges
	t.Cleanup(func() { pinScan = old })

	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-ecl")
	rc := RunConfig{Config: Config{}}
	reached := []func(*GorgeSide) bool{onBattlefield("Shore Lurker")}
	path := filepath.Join(t.TempDir(), "pin.json")

	err := recordPins(t, path, "std-ecl", set, rc, 1, reached)
	if err != nil {
		if !strings.Contains(err.Error(), "PINS_MUTANT") {
			t.Fatalf("unexpected recording failure: %v", err)
		}
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("failed PINS_MUTANT recording left %s", path)
		}
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var games []*Repro
	if err := json.Unmarshal(b, &games); err != nil {
		t.Fatal(err)
	}
	if len(games) == 0 {
		t.Fatal("recording succeeded but wrote no games")
	}
	for i, r := range games {
		got, h, err := playPinned(set, reg, r, rc, reached)
		if err != nil {
			t.Fatalf("game %d: %v", i, err)
		}
		if got.Div == nil {
			t.Errorf("game %d (seed %d) does not reproduce a divergence", i, r.Seed)
		}
		if contains(h, false) {
			t.Errorf("game %d (seed %d) no longer reaches every condition", i, r.Seed)
		}
	}
}
