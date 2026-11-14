//go:build dirac_mp

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestFourSeatMBCAzraOddsmakerLowers plays four-seat pods of Azra Oddsmaker
// against bears, alternating around the table: the chosen creature's combat
// damage to a player (any of three opponents) draws its controller two cards,
// another creature's does not. No divergence, no kernel stop, and some game
// holds the Effect's registration at a pick (the choice was made).
func TestFourSeatMBCAzraOddsmakerLowers(t *testing.T) {
	reg := synthReg(t)
	azra := deckOf(10, "Azra Oddsmaker", deckOf(6, "Grizzly Bears", deckOf(10, "Swamp", deckOf(10, "Mountain", deckOf(4, "Hill Giant")...)...)...)...)
	bears := deckOf(16, "Grizzly Bears", deckOf(24, "Forest")...)
	decks := [4][]string{azra, bears, azra, bears}
	set := printedDelveSet(t, [2][]string{{"Azra Oddsmaker", "Grizzly Bears", "Swamp", "Mountain", "Hill Giant"}, {"Grizzly Bears", "Forest"}})
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
	reached := 0
	for seed := uint64(1); seed <= 12; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 30
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		armed := false
		rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
			for _, dt := range ref.E.G.Delayed {
				if o := ref.E.G.Obj(dt.Source); dt.EventMode == "DamageDone" && o != nil && o.Face() != nil && o.Face().Name == "Azra Oddsmaker" {
					armed = true
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if armed {
			reached++
		}
		t.Logf("seed %d: %d decisions, over %v %s, Effect armed %v", seed, res.Decisions, res.Over, res.Stall, armed)
	}
	if reached == 0 {
		t.Fatal("no game held an Azra Oddsmaker Effect registration")
	}
}
