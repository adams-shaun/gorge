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

// TestPinnedFarmAggressiveMammoth pins the std-fdn farm game FDN-s416
// xe:RAND:3:6, the lockstep game that diverged at turn 15 on
// projection Cards[0].Power with Aggressive Mammoth implicated. The WG
// seat's Wilt-Leaf Liege was discarded by the opposing UB seat's Painful
// Quandary; the discard replacement (CR 614.1) puts the Liege onto the
// battlefield in gorge, and its green-and-white anthem is what makes the
// Mammoth 9/9 rather than 8/8. dirac used to send that unless-cost discard
// to the graveyard (211265c4 makes the resolving ability the discard's
// cause, CR 118.12), so the Liege, and the Mammoth's +1/+1 with it, were
// missing from the projection. The pin carries the farm's decks (the
// tape's opening dealt from the pair's decklists sorted by name), its seed
// gameSeed(416, 3, 6) and the whole game's explore-policy actions, and it
// reaches both the Mammoth on the battlefield and the Liege discarded into
// play; it is written from the farm's tape rather than dealt by the
// synthetic pool, so PINS_RECORD is deliberately not used.
func TestPinnedFarmAggressiveMammoth(t *testing.T) {
	t.Parallel()
	const path = "testdata/pins/TestPinnedFarmAggressiveMammoth.0.json"
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
	set := cardset.AllowUnverified(cardsettest.Synth(t, "std-fdn"))
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}}
	reached := []func(*GorgeSide) bool{
		onBattlefield("Aggressive Mammoth"),
		discardedIntoPlay("Wilt-Leaf Liege"),
	}
	for i, r := range games {
		if len(r.Actions) <= 284 {
			t.Fatalf("%s game %d: %d actions, the pin must pass decision 284 (turn 15, where the farm game parted)", path, i, len(r.Actions))
		}
		res, hit, err := playPinned(set, reg, r, rc, reached)
		if err != nil {
			t.Fatalf("%s game %d: %v", path, i, err)
		}
		reportPinned(t, fmt.Sprintf("%s game %d", path, i), res, hit)
		if res.Turns < 15 {
			t.Errorf("%s game %d: played %d turns, short of turn 15 where the farm game parted", path, i, res.Turns)
		}
	}
}
