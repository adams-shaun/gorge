package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestPinnedFarmStolenStarkTech pins the farm game MSH-s119 xe:RAND:3:1, the
// std-msh lockstep game that panicked in dirac's convoke payment ("kernel:
// payment failed (generic)") at decision 452 with Stolen Stark Tech named. The
// damage is a convoke cast whose picks leave a fixed pip the floated restricted
// mana must pay (CR 106.6, 601.2h); the pin carries the recorded decks, seed and
// the actions up to that cast. It is written by hand from the farm's tape rather
// than dealt by the synthetic pool, so PINS_RECORD is deliberately not used:
// re-recording would replace it with a different game.
func TestPinnedFarmStolenStarkTech(t *testing.T) {
	t.Parallel()
	const path = "testdata/pins/TestPinnedFarmStolenStarkTech.0.json"
	b, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("read the hand-recorded farm pin: %v", err)
	}
	var games []*Repro
	if err := json.Unmarshal(b, &games); err != nil || len(games) == 0 {
		t.Fatalf("%s: %v", path, err)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-msh")
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}}
	for i, r := range games {
		res, hit, err := playPinned(set, reg, r, rc, []func(*GorgeSide) bool{cardSeen("Stolen Stark Tech")})
		if err != nil {
			t.Fatalf("%s game %d: %v", path, i, err)
		}
		reportPinned(t, fmt.Sprintf("%s game %d", path, i), res, hit)
	}
}
