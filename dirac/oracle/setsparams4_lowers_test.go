package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// The FlippedCoin lockstep: "whenever you win a coin flip" (Tavern
// Scoundrel's two Treasures, Chance Encounter's luck counter, Setzer's two
// tapped Treasures; gorge trigmatch flippedCoinMatches on effFlipCoin's
// per-coin result Note, kernel trig_flippedcoin.go from OpRollDie's flip).
// Tavern Scoundrel and Goblin Archaeologist flip coins of their own.
var sp4FlipDecks = [2][]cardset.Entry{
	{{Name: "Tavern Scoundrel", Count: 10}, {Name: "Chance Encounter", Count: 4}, {Name: "Setzer, Wandering Gambler", Count: 2}, {Name: "Goblin Archaeologist", Count: 4}, {Name: "Mountain", Count: 14}, {Name: "Swamp", Count: 6}},
	{{Name: "Ornithopter", Count: 6}, {Name: "Grizzly Bears", Count: 16}, {Name: "Forest", Count: 18}},
}

// sp4FlipObserver counts, from gorge's events, FlippedCoin triggers put on
// the stack (Tavern Scoundrel's and Chance Encounter's only triggers; Setzer's
// pushes are counted apart, they include its enters and damage lines).
func sp4FlipObserver(fired, setzer *int) func(*GorgeSide) bool {
	seen := 0
	var last *GorgeSide
	return func(ref *GorgeSide) bool {
		if ref != last {
			last, seen = ref, 0
		}
		g := ref.E.G
		evs := ref.E.L.Events
		hit := false
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			if ev.Kind != events.TriggerPush {
				continue
			}
			o := g.Obj(ev.Obj)
			if o == nil || o.Face() == nil {
				continue
			}
			switch o.Face().Name {
			case "Tavern Scoundrel", "Chance Encounter":
				*fired++
				hit = true
			case "Setzer, Wandering Gambler":
				*setzer++
			}
		}
		return hit
	}
}

func sp4FlipLowers(t *testing.T, pol func(uint64) Policy, name string) {
	if isolate(t) {
		return
	}
	fired, setzer := 0, 0
	_, games := sp3Run(t, "flippedcoin", sp4FlipDecks, pol, 24, false, nil, sp4FlipObserver(&fired, &setzer))
	t.Logf("%s: FlippedCoin triggers %d in %d games, Setzer pushes %d", name, fired, games, setzer)
	if fired < 4 {
		t.Errorf("FlippedCoin fired %d times, want at least 4", fired)
	}
}

// TestFlippedCoinLowersRandom / TestFlippedCoinLowersExplore: no divergence
// and no unmodelled stall, and won flips fired their triggers on gorge's
// side (one policy per test: each fits the 60 s budget).
func TestFlippedCoinLowersRandom(t *testing.T)  { sp4FlipLowers(t, pkPolicy, "random") }
func TestFlippedCoinLowersExplore(t *testing.T) { sp4FlipLowers(t, ExplorePolicy, "explore") }

// TestFlippedCoinMutation: with a "win" line firing on a lost flip too
// (dirac.DebugFlippedCoinAnySide) the kernel queues triggers gorge does not,
// so the lockstep diverges and the Lowers tests do not pass vacuously.
func TestFlippedCoinMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugFlippedCoinAnySide = true
	defer func() { dirac.DebugFlippedCoinAnySide = false }()
	fired, setzer := 0, 0
	if d, _ := sp3Run(t, "flippedcoin", sp4FlipDecks, ExplorePolicy, 24, true, nil, sp4FlipObserver(&fired, &setzer)); d == nil {
		t.Error("no divergence with FlippedCoin firing on either side")
	}
}
