//go:build dirac_mp

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestFourSeatMBCTriggerReferents2Lowers plays four-seat pods of the
// TriggeredTarget$Valid carriers against bears, alternating around the
// table: past two seats the damaged player is the seat the attacker hit,
// not the controller's next opponent, so Rakdos the Defiler's "half the
// non-Demon permanents they control" and Tomb Blade's "the number of
// creatures they control" count that seat's battlefield. No divergence, no
// kernel stop, and the trigger is seen in some game (a bears seat losing a
// Forest, or losing life not straight after a Damage event: nothing else
// makes either). Battle Angels of Tyr is not here: its Myriad lowering is
// two-seat only (lower_myriad.go).
func TestFourSeatMBCTriggerReferents2Lowers(t *testing.T) {
	reg := synthReg(t)
	bears := deckOf(16, "Grizzly Bears", deckOf(24, "Forest")...)
	for _, tc := range []struct {
		name  string
		deck  []string
		names []string
	}{
		{"Rakdos_the_Defiler", deckOf(14, "Rakdos the Defiler", deckOf(13, "Swamp", deckOf(13, "Mountain")...)...), []string{"Rakdos the Defiler", "Swamp", "Mountain"}},
		{"Tomb_Blade", deckOf(16, "Tomb Blade", deckOf(24, "Swamp")...), []string{"Tomb Blade", "Swamp"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decks := [4][]string{tc.deck, bears, tc.deck, bears}
			set := printedDelveSet(t, [2][]string{tc.names, {"Grizzly Bears", "Forest"}})
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
				hit, next, forests := false, 1, [2]int{-1, -1}
				rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
					evs := ref.E.L.Events
					for i := next; i < len(evs); i++ {
						e := evs[i]
						hit = hit || (e.Kind == events.LifeChange && e.Player%2 == 1 && e.Amount < 0 && evs[i-1].Kind != events.Damage)
					}
					next = max(len(evs), 1)
					for i, p := range [2]state.PlayerID{1, 3} {
						n := trigRefsNamedOn(ref, p, "Forest")
						// a seat that lost takes its Forests with it (CR 800.4a)
						hit = hit || (forests[i] >= 0 && n < forests[i] && !ref.E.G.Players[p].Lost)
						forests[i] = n
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
				if hit {
					reached++
				}
				t.Logf("seed %d: %d decisions, over %v %s, trigger seen %v", seed, res.Decisions, res.Over, res.Stall, hit)
			}
			if reached == 0 {
				t.Fatal("no game saw the damage trigger resolve")
			}
		})
	}
}
