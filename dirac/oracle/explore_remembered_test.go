package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// Defossilize returns a creature card it remembers, then that creature
// explores twice (Explore Defined$ Remembered). TestStdSetReprosLockstep
// replays the recorded game; the cases reuse the mass writers' recorder.
func exploreRememberedCases() []rememberMassCase {
	return []rememberMassCase{
		{rememberChoiceCase: rememberChoiceCase{
			// the opponent's Disfigure kills a Memnite; Defossilize returns it
			// and it explores twice
			charmRepeatCase: charmRepeatCase{
				name: "return_remembers_the_explorer_defossilize", card: "Defossilize", prefer: []string{"Memnite", "Disfigure"},
				decks: [2][]string{
					append(deckOf(6, "Defossilize"), append(deckOf(6, "Memnite"), deckOf(12, "Swamp")...)...),
					append(deckOf(8, "Disfigure"), deckOf(16, "Swamp")...),
				},
				kind: events.Explore, n: 2,
			},
			cardName: "Defossilize",
		}},
	}
}

// TestExploreRememberedReprosResolve replays the recorded game: no
// divergence, every scripted action offered, and the returned card explored
// twice during Defossilize's resolution.
func TestExploreRememberedReprosResolve(t *testing.T) {
	reg := synthReg(t)
	for _, c := range exploreRememberedCases() {
		t.Run(c.cardName, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, hits := rememberMassPlay(t, reg, c, r, charmRepeatSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if hits < c.n {
				t.Fatalf("%d %v events from the %s resolution, want at least %d", hits, c.kind, c.cardName, c.n)
			}
		})
	}
}

// TestExploreRememberedRecord plays the cases' scripted policies over seeds
// until one resolves the card with its effect, and writes the repro. It runs
// only with EXPLORE_REMEMBERED_RECORD=1; the games it writes are the
// committed repros.
func TestExploreRememberedRecord(t *testing.T) {
	if os.Getenv("EXPLORE_REMEMBERED_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range exploreRememberedCases() {
		t.Run(c.cardName, func(t *testing.T) {
			set := charmRepeatSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, hits := rememberMassPlay(t, reg, c, r, set, charmRepeatPolicy(m, c.charmRepeatCase))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if hits < c.n {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions, %d events", seed, len(res.Actions), hits)
				return
			}
			t.Fatalf("no seed resolved %s with its effect", c.cardName)
		})
	}
}
