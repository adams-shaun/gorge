package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestMBCSpectralHuntCallerReprosLockstep pins the ev-tail-41-c001 game class
// that parted at an options decision over Spectral Hunt-Caller (a lockstep
// divergence where only one engine offered an option in the same priority
// decision window). The divergence itself no longer reproduces on main: the
// kernel-side modal-offer fixes it landed beside (reverse a modal cast with no
// legal mode, CR 733.1; the per-mode offer gates) took the class with them, so
// there is no shrunk repro to record. This is the standing guard for the
// class instead of the repro replay: six lockstep games whose decks are
// stuffed with Spectral Hunt-Caller, whose policy activates {5}{G} whenever it
// can, assert that the two engines agree on every option the card poses --
// the {5}{G} activation offer, its affordability, its target ask and the
// pumped state.
func TestMBCSpectralHuntCallerReprosLockstep(t *testing.T) {
	reg := synthReg(t)
	pool, set := synthPoolWith(t, "Spectral Hunt-Caller", "Forest")
	t.Logf("carrier pool %s", pool)
	deck := deckOf(6, "Spectral Hunt-Caller", deckOf(24, "Forest")...)
	lists := make([][]cardset.Entry, 2)
	for i := range lists {
		es := make([]cardset.Entry, len(deck))
		for j, n := range deck {
			es[j] = cardset.Entry{Name: n, Count: 1}
		}
		lists[i] = es
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	const seeds = 6
	// act counts the policy's Spectral Hunt-Caller activations: it proves the
	// reproducer reached the card, not that the engines agree. The divergence
	// assertion is what proves the two engines agree about every option the
	// card's {5}{G} activation poses.
	act := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{deck, deck}, Seed: seed, PassElision: true, Surface: "full"}
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
		base := castActivatePlayedPolicy(seed, map[string]int{})
		pol := PolicyFunc(func(c *PickCtx) Canon {
			a := base.Pick(c)
			if a.Kind() == ActActivate && c.G != nil && refCardName(c.G, a.Src()) == "Spectral Hunt-Caller" {
				act++
			}
			return a
		})
		res := Lockstep(m, seed, k, pol, rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.StallUnmodelled {
			t.Fatalf("seed %d: kernel stopped unmodelled: %s", seed, res.Stall)
		}
		t.Logf("seed %d: %d decisions, over %v, %s", seed, res.Decisions, res.Over, res.Stall)
	}
	if act == 0 {
		t.Fatalf("the policy never activated Spectral Hunt-Caller's {5}{G} over %d seeds; the reproducer never reached the card", seeds)
	}
}
