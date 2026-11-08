package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestPinnedFarmWarpSpeed pins the std-trk lockstep game that parts from
// gorge when a shock land enters under Warp Speed's "Lands you control enter
// untapped" (CR 616.1): the order ask puts the land's own enters-tapped body
// last, so its pay-life election is still pending when the body applies. The
// kernel used to apply that body's tap in entryLateSet before the election
// was answered, showing a tapped land at the election ask and tapping it even
// when the player paid; the recorded game reaches Warp Speed, a Hallowed
// Fountain and the order ask. The pin carries the decks, seed and canonical
// actions, so it replays the same game rather than dealing from the pool.
func TestPinnedFarmWarpSpeed(t *testing.T) {
	t.Parallel()
	const path = "testdata/pins/TestPinnedFarmWarpSpeed.0.json"
	b, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("read the hand-recorded Warp Speed pin: %v", err)
	}
	var games []*Repro
	if err := json.Unmarshal(b, &games); err != nil || len(games) == 0 {
		t.Fatalf("%s: %v", path, err)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-trk")
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}}
	for i, r := range games {
		res, hit, err := playPinned(set, reg, r, rc, []func(*GorgeSide) bool{onBattlefield("Warp Speed")})
		if err != nil {
			t.Fatalf("%s game %d: %v", path, i, err)
		}
		reportPinned(t, fmt.Sprintf("%s game %d", path, i), res, hit)
	}
}
