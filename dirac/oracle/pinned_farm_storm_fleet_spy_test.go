package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPinnedFarmStormFleetSpy pins the farm game FDN-s462 xe:RAND:2:15, the
// std-fdn lockstep game that diverged at decision 423 on Cards[74].Flags (Storm
// Fleet Spy) and Pool[1][5]. The UR seat's Storm Fleet Spy, under the UB seat's
// Imprisoned in the Moon, is a colourless land with a granted "{T}: Add {C}";
// beside it sits Pyromancer's Goggles, a deferred {R} source. Both are gaps of
// gorge's planner census, and gorge's one prefix search for Shivan Dragon's {R}
// pump activates the gaps in battlefield order: the Spy (entered first), then
// the Goggles, the {C} floating unused. dirac ran its scripted-gap prefix
// first, which taps the Goggles alone, so the Spy stayed untapped; it now runs
// the grant search (which lists the deferred source) first, as gorge does. The
// pin carries the farm's decks (seat 0 is pair deck 0, the UB deck the tape
// names p1), seed and the whole game's actions under the explore policy; it is
// written from the farm's tape rather than dealt by the synthetic pool, so
// PINS_RECORD is deliberately not used.
func TestPinnedFarmStormFleetSpy(t *testing.T) {
	t.Parallel()
	const path = "testdata/pins/TestPinnedFarmStormFleetSpy.0.json"
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
	reached := []func(*GorgeSide) bool{cardSeen("Storm Fleet Spy"), imprisonedSpyTappedBeside("Pyromancer's Goggles")}
	for i, r := range games {
		res, hit, err := playPinned(set, reg, r, rc, reached)
		if err != nil {
			t.Fatalf("%s game %d: %v", path, i, err)
		}
		reportPinned(t, fmt.Sprintf("%s game %d", path, i), res, hit)
	}
}

// imprisonedSpyTappedBeside holds when Storm Fleet Spy is on the battlefield
// enchanted by Imprisoned in the Moon (so a mana source) while the named source
// of its controller is tapped: the board the divergent payment was made on.
func imprisonedSpyTappedBeside(source string) func(*GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		g := ref.E.G
		for p := range g.Players {
			var spy state.ObjID
			tapped := false
			for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
				o := g.Obj(id)
				if o == nil || o.Face() == nil {
					continue
				}
				switch o.Face().Name {
				case "Storm Fleet Spy":
					spy = id
				case source:
					tapped = tapped || o.Tapped
				}
			}
			if spy == 0 || !tapped {
				continue
			}
			for q := range g.Players {
				for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(q)) {
					if o := g.Obj(id); o != nil && o.AttachedTo == spy && o.Face() != nil && o.Face().Name == "Imprisoned in the Moon" {
						return true
					}
				}
			}
		}
		return false
	}
}
