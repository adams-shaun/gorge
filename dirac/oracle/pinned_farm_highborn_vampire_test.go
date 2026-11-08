package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestPinnedFarmHighbornVampire pins the farm game FDN-s240 xe:RAND:2:1, the
// std-fdn lockstep game that diverged (options) at decision 744 with Highborn
// Vampire named. Joraga Invocation made p0's attackers "must be blocked if
// able" (CR 509.1c); with Marauding Blight-Priest already declared on Prime
// Speaker Zegana, a second blocker there obeys fewer requirements than one on
// another attacker, so gorge's validateBlockers would refuse the declaration
// and the kernel (exact, mbReach) did not offer it. The gorge side's
// completability search had spent its budget by Highborn Vampire's pairs and
// offered Highborn Vampire on Zegana anyway; blockAttackMustReachable now
// bounds that exactly. The pin carries the recorded decks, seed and the whole
// game's actions (it ends at turn 27). It is written from the farm's tape
// (decks) and the farm's own policy (ExplorePolicy at the game seed) rather
// than dealt by the synthetic pool, so PINS_RECORD is deliberately not used:
// re-recording would replace it with a different game.
func TestPinnedFarmHighbornVampire(t *testing.T) {
	t.Parallel()
	const path = "testdata/pins/TestPinnedFarmHighbornVampire.0.json"
	b, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("read the farm pin: %v", err)
	}
	var games []*Repro
	if err := json.Unmarshal(b, &games); err != nil || len(games) == 0 {
		t.Fatalf("%s: %v", path, err)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-fdn")
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}}
	reached := []func(*GorgeSide) bool{onBattlefield("Highborn Vampire"), cardSeen("Joraga Invocation"), onBattlefield("Prime Speaker Zegana")}
	for i, r := range games {
		if len(r.Actions) <= 744 {
			t.Fatalf("%s game %d: %d actions, the pin must pass decision 744 (the blockers decision that diverged)", path, i, len(r.Actions))
		}
		res, hit, err := playPinned(set, reg, r, rc, reached)
		if err != nil {
			t.Fatalf("%s game %d: %v", path, i, err)
		}
		reportPinned(t, fmt.Sprintf("%s game %d", path, i), res, hit)
		if res.Decisions < 745 {
			t.Errorf("%s game %d: played %d decisions, short of the decision-744 blockers ask", path, i, res.Decisions)
		}
	}
}
