//go:build dirac_mp

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFourSeatBodyguardLockstep plays four-seat pods of Dauntless Bodyguards
// against gorge: each controller picks among only their own other creatures
// as the Bodyguard enters (no ask when they control none), and a sacrificed
// Bodyguard's pump protects the creature its controller picked, whichever
// seat's removal is aimed at it. No divergence, no stall, and some game poses
// the creature ask and some game sacrifices a Bodyguard.
func TestFourSeatBodyguardLockstep(t *testing.T) {
	reg := synthReg(t)
	guards := deckOf(10, "Dauntless Bodyguard", deckOf(10, "Grizzly Bears", deckOf(4, "Hill Giant", deckOf(8, "Plains", deckOf(8, "Mountain")...)...)...)...)
	removal := deckOf(4, "Dauntless Bodyguard", deckOf(6, "Doom Blade", deckOf(4, "Shock", deckOf(6, "Grizzly Bears", deckOf(10, "Plains", deckOf(10, "Swamp")...)...)...)...)...)
	decks := [4][]string{guards, removal, guards, removal}
	set := cardsettest.Set(t, "bodyguard")
	var lists [][]cardset.Entry
	for _, d := range decks {
		es := make([]cardset.Entry, len(d))
		for i, n := range d {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	askKind := ActChoose + CanonKind(mustChooseCode("card"))
	asks, acts := 0, 0
	for seed := uint64(1); seed <= 8; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: false, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 6000, 60
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		for _, a := range res.Actions {
			switch a.Kind() {
			case askKind:
				asks++
			case ActActivate:
				acts++
			}
		}
		t.Logf("seed %d: %d decisions, over %v %s", seed, res.Decisions, res.Over, res.Stall)
	}
	if asks == 0 {
		t.Fatal("no game posed the Bodyguard's creature ask")
	}
	t.Logf("%d creature asks, %d activations", asks, acts)
}
