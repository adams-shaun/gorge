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

// TestPinnedFarmGoblinFirebomb pins the std-fdn farm game FDN-s616
// xe:RAND:0:9, the lockstep game that diverged (options) on Goblin Firebomb at
// turn 25. The Goggles' deferred {R} (TriggersWhenSpent$) was a census gap the
// kernel's grant search did not list, so dirac dropped the {7} activation whose
// payment needs the deferred source beside the five untapped lands, while gorge
// offered it (the seventh mana is the land still in hand, CR 601.2g-h, 605.1a).
// ee1b488f08 fixed the kernel; the pin carries the farm's decks, seed and the
// recorded action prefix up to and past that decision. It is written from the
// farm's tape (decks) and the farm's own policy at the game seed 616*1000003+9,
// so PINS_RECORD is deliberately not used: re-recording would replace it.
func TestPinnedFarmGoblinFirebomb(t *testing.T) {
	t.Parallel()
	const path = "testdata/pins/TestPinnedFarmGoblinFirebomb.0.json"
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
	for i, r := range games {
		if len(r.Actions) < 526 {
			t.Fatalf("%s game %d: %d actions, the pin must pass decision 526 (the Firebomb activation the farm offered)", path, i, len(r.Actions))
		}
		res, hit, err := playPinned(set, reg, r, rc, []func(*GorgeSide) bool{
			onBattlefield("Goblin Firebomb"), firebombActivationIsAffordable,
		})
		if err != nil {
			t.Fatalf("%s game %d: %v", path, i, err)
		}
		reportPinned(t, fmt.Sprintf("%s game %d", path, i), res, hit)
		if res.Decisions < 526 {
			t.Errorf("%s game %d: played %d decisions, short of the decision-526 Firebomb activation", path, i, res.Decisions)
		}
	}
}

// firebombActivationIsAffordable is the state the farm's decision 526 needed:
// an untapped Goblin Firebomb on its controller's side, the controller still
// holding a land drop (so the seventh mana is a land in hand), five other
// untapped mana sources beside the deferred Pyromancer's Goggles, and an
// opponent permanent to target. A game that no longer reaches it cannot test
// the offer.
func firebombActivationIsAffordable(ref *GorgeSide) bool {
	g := ref.E.G
	for p := range g.Players {
		seat := state.PlayerID(p)
		firebomb := false
		for _, id := range g.Zone(state.ZBattlefield, seat) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil || o.Face().Name != "Goblin Firebomb" {
				continue
			}
			if !o.Tapped {
				firebomb = true
			}
		}
		if !firebomb || g.Players[seat].LandsPlayed != 0 {
			continue
		}
		lands, goggles := 0, false
		for _, id := range g.Zone(state.ZBattlefield, seat) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil || o.Tapped {
				continue
			}
			switch name := o.Face().Name; {
			case name == "Pyromancer's Goggles":
				goggles = true
			case ref.E.IsLand(id):
				lands++
			}
		}
		if goggles && lands >= 5 && hasLandInHand(g, seat) && opponentHasPermanent(g, 1-seat) {
			return true
		}
	}
	return false
}

// hasLandInHand reports whether the seat holds a land card to play.
func hasLandInHand(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZHand, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().IsLand() {
			return true
		}
	}
	return false
}

// opponentHasPermanent reports whether the other seat controls a permanent, a
// legal target of Goblin Firebomb's "destroy target permanent" (the game's
// pending decision is the priority offer).
func opponentHasPermanent(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && o.Controller == p {
			return true
		}
	}
	return false
}
