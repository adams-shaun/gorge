package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// pod4Deck is a deck of one card name per entry, as deckOf builds them.
func pod4Deck(parts ...[]string) []string {
	var d []string
	for _, p := range parts {
		d = append(d, p...)
	}
	return d
}

// pod4FixtureLockstep plays seeds four-seat games of decks over the fixture
// pool, dirac against gorge under the random policy, and returns how many
// diverged. report fails the test on each divergence and on a kernel stop
// (an unmodelled or kernel stall); mutate, when set, edits the kernel's table
// before every game (a mutation test's proof that the pool exercises the
// mechanism).
func pod4FixtureLockstep(t *testing.T, pool string, decks [4][]string, seeds uint64, report bool, mutate func(*kop.Table)) int {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Set(t, pool)
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
	if mutate != nil {
		mutate(tbl)
	}
	div := 0
	for seed := uint64(1); seed <= seeds; seed++ {
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
			div++
			if report {
				t.Errorf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			continue
		}
		if report && (res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix)) {
			t.Errorf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
	return div
}
