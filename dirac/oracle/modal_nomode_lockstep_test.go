package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestModalCastNoModeReversal: a spell that must choose a mode but has no
// legal one is not a legal announcement (CR 733.1). Gorge reverses the cast
// (rules/cast_asks.go: "no legal modal choice"); the kernel used to continue
// with no mode and resolve the spell, so the card left the hand and the game
// diverged. Cloud's Limit Break with twelve copies and only Plains is the
// reproducer: Blade Beam's ModeCost$ is unaffordable early and its
// Cross-Slash mode has no tapped creature.
func TestModalCastNoModeReversal(t *testing.T) {
	reg := synthReg(t)
	pool, set := synthPoolWith(t, "Cloud's Limit Break", "Plains")
	t.Logf("carrier pool %s", pool)
	deck := deckOf(12, "Cloud's Limit Break", deckOf(28, "Plains")...)
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
	// cast counts the policy's Cloud's Limit Break casts: it proves the
	// reproducer reached the card, not that any cast reversed. The divergence
	// assertion is what proves the two engines agree about the reversal.
	cast := 0
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
			if a.Kind() == ActCast && c.G != nil && refCardName(c.G, a.Src()) == "Cloud's Limit Break" {
				cast++
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
	if cast == 0 {
		t.Fatalf("the policy never chose a Cloud's Limit Break cast over %d seeds; the reproducer never reached the card", seeds)
	}
}

// synthPoolWith finds a synthetic pool that holds every named card, rather
// than naming one: buildSynthList splits a tail-* bucket when a newly lowered
// card pushes its table over a size limit, and a pinned bucket name would
// then fail an unrelated landing. Plains is in every pool.
func synthPoolWith(t *testing.T, cards ...string) (string, cardset.Set) {
	t.Helper()
	for _, name := range cardsettest.SynthNames(t) {
		set := cardsettest.Synth(t, name)
		ok := true
		for _, card := range cards {
			i, has := set.Lookup(card)
			if !has || set.Status(i) == cardset.StatusRefused {
				ok = false
				break
			}
		}
		if ok {
			return name, set
		}
	}
	t.Fatalf("no synthetic pool holds %v", cards)
	return "", nil
}
