//go:build dirac_mp

package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFourSeatOpponentExtremesLockstep plays four-seat pods whose decks lean
// on a figure over the opponents (PlayerCountOpponents$: Bloodghast's lowest
// opponent life, Ticket Tortoise's, Claim Jumper's and Sunstar
// Expansionist's most lands an opponent controls, Joined Researchers' most
// cards in an opponent's hand, Emeritus of Truce's most creatures, Vault of
// Champions' opponent count) dirac against gorge: no divergence, no stall,
// and the cards are cast, so the extreme is read over three opponents.
func TestFourSeatOpponentExtremesLockstep(t *testing.T) {
	reg := synthReg(t)
	cases := []struct {
		pool  string
		decks [4][]string
	}{
		{"std-dft", [4][]string{
			deckOf(16, "Bloodghast", deckOf(24, "Swamp")...),
			deckOf(16, "Ticket Tortoise", deckOf(24, "Swamp")...),
			deckOf(16, "Bloodghast", deckOf(24, "Swamp")...),
			deckOf(16, "Ticket Tortoise", deckOf(24, "Swamp")...),
		}},
		{"std-otj", [4][]string{
			deckOf(16, "Claim Jumper", deckOf(24, "Plains")...),
			deckOf(40, "Plains"),
			deckOf(16, "Claim Jumper", deckOf(24, "Plains")...),
			deckOf(40, "Plains"),
		}},
		{"std-sos", [4][]string{
			deckOf(16, "Joined Researchers", deckOf(24, "Plains")...),
			deckOf(16, "Emeritus of Truce", deckOf(24, "Plains")...),
			deckOf(40, "Plains"),
			deckOf(16, "Joined Researchers", deckOf(24, "Plains")...),
		}},
		{"std-eoe", [4][]string{
			deckOf(16, "Sunstar Expansionist", deckOf(24, "Plains")...),
			deckOf(40, "Plains"),
			deckOf(16, "Sunstar Expansionist", deckOf(24, "Plains")...),
			deckOf(40, "Plains"),
		}},
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	for _, tc := range cases {
		set := cardsettest.Synth(t, tc.pool)
		var lists [][]cardset.Entry
		for _, d := range tc.decks {
			lists = append(lists, entries(d))
		}
		tbl, _, err := set.MatchupN(lists)
		if err != nil {
			t.Fatalf("%s: %v", tc.pool, err)
		}
		casts := 0
		for seed := uint64(1); seed <= 6; seed++ {
			r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{tc.decks[0], tc.decks[1]},
				MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{tc.decks[2], tc.decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			rc := r.RunConfig()
			rc.MaxDecisions, rc.MaxTurns = 3000, 30
			k, err := SetSide(set, tbl, rc.Config)(m, seed)
			if err != nil {
				t.Fatalf("%s seed %d: %v", tc.pool, seed, err)
			}
			res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
			CloseSide(k)
			if res.Div != nil {
				t.Fatalf("%s seed %d: divergence at %d: %s %s %v", tc.pool, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("%s seed %d: kernel stopped: %s", tc.pool, seed, res.Stall)
			}
			for _, a := range res.Actions {
				if a.Kind() == ActCast {
					casts++
				}
			}
			t.Logf("%s seed %d: %d decisions, over %v %s", tc.pool, seed, res.Decisions, res.Over, res.Stall)
		}
		if casts == 0 {
			t.Fatalf("%s: no spell was cast in any game", tc.pool)
		}
		t.Log(fmt.Sprintf("%s: %d casts", tc.pool, casts))
	}
}
