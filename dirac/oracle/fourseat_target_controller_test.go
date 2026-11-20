package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// fourSeatTargetPlay returns a player of four-seat pods of decks on pool,
// dirac against gorge, one seed per call, under pol.
func fourSeatTargetPlay(t *testing.T, pool string, decks [4][]string, pol func(uint64) Policy) func(uint64) Result {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, pool)
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
	return func(seed uint64) Result {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 10000, 60
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		defer CloseSide(k)
		return Lockstep(m, seed, k, pol(seed), rc)
	}
}

// castWatchPolicy plays a heuristic game but takes a watched card's cast
// whenever it is offered, so a four-seat pod actually resolves it; casts
// counts those picks.
func castWatchPolicy(card string, casts *int) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := HeuristicPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DPriority {
				for _, a := range c.D.Options {
					if a.Kind() == ActCast && refCardName(c.G, a.Src()) == card {
						*casts++
						return a
					}
				}
			}
			return rnd.Pick(c)
		})
	}
}

// TestFourSeatTargetControllerLockstep plays four-seat pods of the cards
// whose target-derived player was read as the one opponent: Astarion and
// Thought-Stalker Warlock read the targeted player's own life-lost tally
// (CR 119.3), Rules of Acquisition draws for the remembered target's
// controller, and Legions to Ashes sweeps only the target's same-name
// tokens. Each card is played in its own synthetic pool, dirac against gorge
// at four seats, with its cast taken whenever offered: no divergence and no
// stall, and each card is actually cast (the precondition the comparison
// rests on).
func TestFourSeatTargetControllerLockstep(t *testing.T) {
	astarion := append(deckOf(10, "Astarion, the Decadent"),
		append(deckOf(10, "Ebony Charm"),
			append(deckOf(10, "Plains"), deckOf(10, "Swamp")...)...)...)
	warlock := append(deckOf(10, "Thought-Stalker Warlock"), deckOf(30, "Swamp")...)
	rules := append(deckOf(10, "Rules of Acquisition"),
		append(deckOf(10, "Romulan Bird-of-Prey"), deckOf(20, "Island")...)...)
	legions := append(deckOf(10, "Legions to Ashes"),
		append(deckOf(10, "Accorder Paladin"), append(deckOf(10, "Plains"), deckOf(10, "Swamp")...)...)...)
	cases := []struct {
		card string
		pool string
		deck []string
	}{
		{"Astarion, the Decadent", "tail-06", astarion},
		{"Thought-Stalker Warlock", "std-blb", warlock},
		{"Rules of Acquisition", "std-trk", rules},
		{"Legions to Ashes", "tail-64", legions},
	}
	for _, c := range cases {
		c := c
		t.Run(c.card, func(t *testing.T) {
			casts := 0
			play := fourSeatTargetPlay(t, c.pool, [4][]string{c.deck, c.deck, c.deck, c.deck}, castWatchPolicy(c.card, &casts))
			const seeds = 4
			for seed := uint64(1); seed <= seeds; seed++ {
				res := play(seed)
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %q\nker %q\n%s last %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff,
						res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced)
				}
				if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
					t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
				}
				t.Logf("seed %d: %d decisions, over %v %s", seed, res.Decisions, res.Over, res.Stall)
			}
			if casts == 0 {
				t.Fatalf("%s was never cast in %d four-seat games", c.card, seeds)
			}
			t.Logf("%s cast %d times", c.card, casts)
		})
	}
}
