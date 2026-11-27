package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestAltCostXGrantLockstep drives Kentaro, the Smiling Cat's granted
// alternative cost ("You may pay {X} rather than pay the mana cost for
// Samurai spells you cast, where X is that spell's mana value"; ValidSA$
// Spell.Samurai | XAlternative$ Count$CardManaCost) on a scripted board. The
// policy floats exactly the Samurai's mana value with Rite of Flame, spends
// it down to one less with a one-mana creature, checks the alternative is not
// offered, then builds exactly the mana value again and takes the alternative
// cast. Finally it floats more than the value and checks a non-Samurai spell
// is not offered. Both engines list the same options at every decision; the
// pre-fix refusal (kgen refuses the card, so its table cannot hold it) is the
// test's failing control.
func TestAltCostXGrantLockstep(t *testing.T) {
	t.Parallel()
	sc, err := ParseScenario([]byte(`{"format":"gorge-start-1","focus":"Kentaro, the Smiling Cat","players":2,"opponent":"passive","active":0,"max_turns":2,
 "setup":{"p0":{"battlefield":["Kentaro, the Smiling Cat","Mountain","Mountain","Mountain","Mountain"],"hand":["Rite of Flame","Rite of Flame","Rite of Flame","Raging Goblin","Raging Goblin","Hand of Honor","Konda's Hatamoto","Grizzly Bears"]},
          "p1":{"battlefield":["Forest"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c, err := OpenCorpus(corpusDirOrSkip(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m, err := c.ScenarioMatchup(sc, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	for p := range names {
		for _, card := range m.SeatDeck(p) {
			names[p] = append(names[p], cardName(card))
		}
	}
	set := cardset.Raw(printedDelveSet(t, names))
	entries := func(d []*cards.Card) []cardset.Entry {
		var out []cardset.Entry
		for _, card := range d {
			n := cardName(card)
			i := slices.IndexFunc(out, func(e cardset.Entry) bool { return e.Name == n })
			if i < 0 {
				out = append(out, cardset.Entry{Name: n})
				i = len(out) - 1
			}
			out[i].Count++
		}
		return out
	}
	tbl, _, _, err := set.Matchup(entries(m.SeatDeck(0)), entries(m.SeatDeck(1)))
	if err != nil {
		t.Fatal(err)
	}
	rc := RunConfig{Config: Config{}, MaxDecisions: 600, MaxTurns: 2}
	const seeds = 2
	next := uint64(1)
	var st xPickStats
	for i := 0; i < seeds; i++ {
		seed := sc.GameSeed(next)
		next = seed + 1
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		pol := xPickPolicy(&st)
		res := Lockstep(m, seed, k, pol, rc)
		CloseSide(k)
		if !res.Clean() {
			if res.Div == nil {
				t.Fatalf("seed %d: not clean without a divergence: stall %q", seed, res.Stall)
			}
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v (%s)\nref %v\nker %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
		}
	}
	if !st.exactChecked || st.exactOffers == 0 {
		t.Fatalf("exact floating: checked %v, offers %d; want the alternative offered", st.exactChecked, st.exactOffers)
	}
	if !st.lessChecked || st.lessOffers != 0 {
		t.Fatalf("one less floating: checked %v, offers %d; want no alternative", st.lessChecked, st.lessOffers)
	}
	if !st.nonSamuraiChecked || st.nonSamuraiOffers != 0 {
		t.Fatalf("non-Samurai: checked %v, offers %d; want no alternative", st.nonSamuraiChecked, st.nonSamuraiOffers)
	}
	if st.altCasts == 0 {
		t.Fatal("the alternative cast was never taken")
	}
	t.Logf("%d games clean; exact offers %d, one-less offers %d, non-Samurai offers %d, alt casts %d", seeds, st.exactOffers, st.lessOffers, st.nonSamuraiOffers, st.altCasts)
}

// xPickStats are the positions xPickPolicy reached.
type xPickStats struct {
	exactChecked      bool
	exactOffers       int
	lessChecked       bool
	lessOffers        int
	nonSamuraiChecked bool
	nonSamuraiOffers  int
	altCasts          int
}

// xPickPolicy builds the floating pool with Rite of Flame and Raging Goblin
// and checks the three positions: one less than the Samurai's mana value,
// exactly it, and at least it for a non-Samurai. The first alternative cast
// is taken; the other offers are only counted, so the game stays on the
// checked surface.
func xPickPolicy(st *xPickStats) Policy {
	stage := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || c.D.Kind != DPriority || c.D.Player != 0 {
			return passOrFirst(c)
		}
		g := c.G.E.G
		if g.Active != 0 || g.Step != state.StepMain1 || len(g.Stack) != 0 {
			return passOrFirst(c)
		}
		pool := int(g.Players[0].Pool.Total())
		switch stage {
		case 0: // Rite of Flame adds two: exact
			if pool == 0 {
				if a, ok := plainCastFor(c, "Rite of Flame"); ok {
					stage = 1
					return a
				}
			}
		case 1: // spend one on a 1-mana creature: one less
			if pool == 2 {
				if a, ok := plainCastFor(c, "Raging Goblin"); ok {
					stage = 2
					return a
				}
			}
		case 2: // one less: the alternative must not be offered
			if pool == 1 {
				if _, ok := altCastFor(c, "Konda's Hatamoto"); ok {
					st.lessOffers++
				} else {
					st.lessChecked = true
				}
				if a, ok := plainCastFor(c, "Rite of Flame"); ok {
					stage = 3
					return a
				}
			}
		case 3: // the second Rite adds three (one in a graveyard); spend one
			if pool == 3 {
				if a, ok := plainCastFor(c, "Raging Goblin"); ok {
					stage = 4
					return a
				}
			}
		case 4: // exact: the alternative is offered; take it
			if pool == 2 {
				st.exactChecked = true
				if a, ok := altCastFor(c, "Hand of Honor"); ok {
					st.exactOffers++
					st.altCasts++
					stage = 5
					return a
				}
				stage = 9
			}
		case 5: // the third Rite: more than the Samurai's mana value
			if pool == 0 {
				if a, ok := plainCastFor(c, "Rite of Flame"); ok {
					stage = 6
					return a
				}
			}
		case 6: // a non-Samurai must not be offered the alternative
			if pool >= 2 {
				if _, ok := altCastFor(c, "Grizzly Bears"); ok {
					st.nonSamuraiOffers++
				} else {
					st.nonSamuraiChecked = true
				}
				stage = 9
			}
		}
		return passOrFirst(c)
	})
}

// altCastFor finds the caster's alternative-cost cast of card name (the
// canonical mode's high nibble is the AltCostIndex).
func altCastFor(c *PickCtx, name string) (Canon, bool) {
	for _, a := range c.D.Options {
		if a.Kind() == ActCast && a.Mode()>>4 >= 1 && refCardName(c.G, a.Src()) == name {
			return a, true
		}
	}
	return 0, false
}

// plainCastFor finds the caster's printed-cost cast of card name.
func plainCastFor(c *PickCtx, name string) (Canon, bool) {
	for _, a := range c.D.Options {
		if a.Kind() == ActCast && a.Mode()>>4 == 0 && refCardName(c.G, a.Src()) == name {
			return a, true
		}
	}
	return 0, false
}

// passOrFirst passes a priority decision, or takes the first option elsewhere.
func passOrFirst(c *PickCtx) Canon {
	for _, a := range c.D.Options {
		if a.Kind() == ActPass {
			return a
		}
	}
	if len(c.D.Options) == 0 {
		return 0
	}
	return c.D.Options[0]
}
