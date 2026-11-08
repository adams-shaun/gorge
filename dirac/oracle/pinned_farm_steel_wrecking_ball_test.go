package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPinnedFarmSteelWreckingBall pins the std-spm farm game SPM-s145
// xe:RAND:4:19, whose lockstep diverged on the options at turn 8 with Steel
// Wrecking Ball named. The farm kept no tape, so the game is the farm's two
// decklists (read off the game's turn windows) played at a seed where the same
// board arises: Steel Wrecking Ball in hand beside an untapped Daily Bugle
// Building and Interdimensional Web Watch. gorge's census misses both mana
// sources (a filter and a wide restricted one), so its prefix search prices
// the play and spends its 160 nodes before the Swamp-funded filter's {R}
// (CR 601.2g, 605.3a); dirac's mirror of the Web Watch's colour-pair ask
// dropped its W pairs, found the script inside the budget gorge spends, and
// offered the play gorge withholds: Steel Wrecking Ball's discard ability, on
// the autopay surface.
// Written by hand, so PINS_RECORD is not used: re-recording would deal a
// different game.
func TestPinnedFarmSteelWreckingBall(t *testing.T) {
	t.Parallel()
	const path = "testdata/pins/TestPinnedFarmSteelWreckingBall.0.json"
	b, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("read the hand-recorded farm pin: %v", err)
	}
	var games []*Repro
	if err := json.Unmarshal(b, &games); err != nil || len(games) == 0 {
		t.Fatalf("%s: %v", path, err)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-spm")
	reached := []func(*GorgeSide) bool{wreckingBallBoard}
	for i, r := range games {
		rc := RunConfig{Config: Config{GorgeAutopaySurface: r.Surface == "autopay"}}
		res, hit, err := playPinned(set, reg, r, rc, reached)
		if err != nil {
			t.Fatalf("%s game %d: %v", path, i, err)
		}
		reportPinned(t, fmt.Sprintf("%s game %d", path, i), res, hit)
	}
}

// wreckingBallBoard is the condition that a player holds Steel Wrecking Ball
// while controlling an untapped Daily Bugle Building and Interdimensional Web
// Watch: the two census gaps the prefix search branches on.
func wreckingBallBoard(ref *GorgeSide) bool {
	g := ref.E.G
	for p := range g.Players {
		pid := state.PlayerID(p)
		if !inZoneNamed(g, state.ZHand, pid, "Steel Wrecking Ball") {
			continue
		}
		untapped := func(name string) bool {
			for _, id := range g.Zone(state.ZBattlefield, pid) {
				if o := g.Obj(id); o != nil && !o.Tapped && o.Face() != nil && o.Face().Name == name {
					return true
				}
			}
			return false
		}
		if untapped("Daily Bugle Building") && untapped("Interdimensional Web Watch") {
			return true
		}
	}
	return false
}
