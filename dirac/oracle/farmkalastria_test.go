package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPinnedFarmKalastriaHighborn pins the farm game FDN-s233 xe:RAND:4:0, the
// std-fdn lockstep game that diverged at an options decision with Kalastria
// Highborn named. Its controller held only Forests and Vizier of the Menagerie
// when the Highborn died: gorge offered the triggered {B} payment because it read
// Vizier's "spend mana as though it were mana of any type to cast creature
// spells" as covering a triggered cost, while that conversion covers a cast only
// (CR 609.4b). The pin is the farm's decks played on a seed that reaches the same
// death; PINS_RECORD is not used, since re-recording would replace it.
func TestPinnedFarmKalastriaHighborn(t *testing.T) {
	t.Parallel()
	const path = "testdata/pins/TestPinnedFarmKalastriaHighborn.0.json"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the hand-recorded farm pin: %v", err)
	}
	var games []*Repro
	if err := json.Unmarshal(b, &games); err != nil || len(games) == 0 {
		t.Fatalf("%s: %v", path, err)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-fdn")
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}}
	for i, r := range games {
		res, hit, err := playPinned(set, reg, r, rc, []func(*GorgeSide) bool{func(ref *GorgeSide) bool {
			return inZoneNamed(ref.E.G, state.ZGraveyard, 1, "Kalastria Highborn")
		}})
		if err != nil {
			t.Fatalf("%s game %d: %v", path, i, err)
		}
		reportPinned(t, fmt.Sprintf("%s game %d", path, i), res, hit)
	}
}
