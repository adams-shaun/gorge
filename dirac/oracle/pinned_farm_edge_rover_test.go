package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestPinnedFarmEdgeRover pins the std-eoe farm game EOE-s645 xe:RAND:2:9, the
// lockstep game that diverged at decision 179 on the projection's NStack: a
// Kavaron Harrier's attack trigger created two Robot tokens in one resolution,
// and gorge's one end-of-combat sacrifice names both (CR 603.7), while the
// kernel registered a delayed rider per token. The two pending riders then
// made the kernel pose an order ask before its stack held what gorge's did, so
// the projection parted at the end of combat. The pin carries the farm's decks
// (the tape's lower row has swap=true, so the game's seat 0 is the lowered
// item's p1 deck, the WR build, and seat 1 its p0 deck, WG), its seed and the
// whole game's explore-policy actions up to the divergence; it is written from the farm's tape rather than dealt by the
// synthetic pool, so PINS_RECORD is deliberately not used.
func TestPinnedFarmEdgeRover(t *testing.T) {
	t.Parallel()
	const path = "testdata/pins/TestPinnedFarmEdgeRover.0.json"
	b, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("read the farm pin: %v", err)
	}
	var games []*Repro
	if err := json.Unmarshal(b, &games); err != nil || len(games) == 0 {
		t.Fatalf("%s: %v", path, err)
	}
	reg := synthReg(t)
	// The farm's table plays unverified cards too (xengine's dside.Tables).
	set := cardset.AllowUnverified(cardsettest.Synth(t, "std-eoe"))
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}}
	for i, r := range games {
		res, hit, err := playPinned(set, reg, r, rc, []func(*GorgeSide) bool{onBattlefield("Edge Rover")})
		if err != nil {
			t.Fatalf("%s game %d: %v", path, i, err)
		}
		reportPinned(t, fmt.Sprintf("%s game %d", path, i), res, hit)
	}
}
