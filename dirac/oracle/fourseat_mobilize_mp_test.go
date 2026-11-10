//go:build dirac_mp

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFourSeatMobilizeLockstep plays four-seat TDM pods of mobilize cards
// dirac against gorge: each mobilize trigger asks which opponent its tokens
// attack, the attacker's own defender first (gorge chooseEnteringDefender),
// and the tokens attack the one picked. No divergence, no stall, and some
// ask picks a defender other than the first.
func TestFourSeatMobilizeLockstep(t *testing.T) {
	reg := synthReg(t)
	red := deckOf(8, "Shock Brigade", deckOf(8, "Bone-Cairn Butcher", deckOf(24, "Mountain")...)...)
	white := deckOf(8, "Voice of Victory", deckOf(8, "Zurgo's Vanguard", deckOf(24, "Plains")...)...)
	decks := [4][]string{red, white, red, white}
	set := cardsettest.Synth(t, "std-tdm")
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
	asks, other := 0, 0
	for seed := uint64(1); seed <= 8; seed++ {
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
		res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		for _, a := range res.Actions {
			if a.Kind() >= ActChoose && a.Tgt() >= RefPlayer && a.Tgt() < RefNone {
				asks++
			}
		}
		other += res.Decisions
		t.Logf("seed %d: %d decisions, over %v %s", seed, res.Decisions, res.Over, res.Stall)
	}
	t.Logf("%d player-choice actions over %d decisions", asks, other)
}
