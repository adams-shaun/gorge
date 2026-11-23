package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
)

// roomNamePodEntries: every seat holds Gruff Triplets (a name read,
// "each creature you control named Gruff Triplets") beside the Rooms and the
// effects that unlock a door, so a Gruff Triplets dies with a Room on the
// battlefield.
func roomNamePodEntries() [4][]cardset.Entry {
	deck := func(land1, land2 string) []cardset.Entry {
		return []cardset.Entry{
			{Name: "Gruff Triplets", Count: 4}, {Name: "Dazzling Theater", Count: 3},
			{Name: "Bottomless Pool", Count: 3}, {Name: "Ghostly Keybearer", Count: 2},
			{Name: "Keys to the House", Count: 2}, {Name: "Grizzly Bears", Count: 4},
			{Name: land1, Count: 11}, {Name: land2, Count: 11},
		}
	}
	return [4][]cardset.Entry{deck("Forest", "Plains"), deck("Forest", "Island"), deck("Forest", "Swamp"), deck("Forest", "Plains")}
}

// TestRoomNameFourSeatLockstep: a pod holding Gruff Triplets and a two-door
// Room builds (the matchup guard refuses only a read of a name a Room's doors
// carry, CR 709.5) and plays dirac against gorge with no divergence, doors
// unlocked.
func TestRoomNameFourSeatLockstep(t *testing.T) {
	decks := roomNamePodEntries()
	set := cardsettest.Set(t, "roomname")
	tbl, _, err := set.MatchupN(decks[:])
	if err != nil {
		t.Fatalf("matchup refused: %v", err)
	}
	reg := synthReg(t)
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	unlocks := 0
	for seed := uint64(1); seed <= 6; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
			Cards: [2][]string{expand(decks[0]), expand(decks[1])}, MoreDecks: []string{"x2", "x3"},
			MoreCards: [][]string{expand(decks[2]), expand(decks[3])}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 8000, 30
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		g, err := NewGorgeSide(m, seed, rc.Config, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, castFirstPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			ventureFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		n := 0
		for i := range g.E.L.Events {
			if g.E.L.Events[i].Kind == events.DoorUnlock {
				n++
			}
		}
		unlocks += n
		t.Logf("seed %d: %d decisions, over %v %s, %d door unlocks", seed, res.Decisions, res.Over, res.Stall, n)
	}
	if unlocks == 0 {
		t.Fatal("no door was unlocked: the fixture never reached a Room on the battlefield")
	}
}
