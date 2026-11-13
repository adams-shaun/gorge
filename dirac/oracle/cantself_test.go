package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The self CantBeCast lockstep: Jace Reawakened ("can't
// be cast during your first three turns", Caster$ Player.Active) and Proft
// ("can't be cast unless seven cards are in your graveyard") on a deck with
// self-mill and card selection. The kernel must withhold the cast offer
// exactly while gorge's castRestricted does, from the card's own static in
// the hand.

var kcantselfDecks = [][2][]cardset.Entry{{
	{
		{Name: "Jace Reawakened", Count: 4}, {Name: "Proft, Sinister Mastermind", Count: 4}, {Name: "Stitcher's Supplier", Count: 4},
		{Name: "Thought Scour", Count: 4}, {Name: "Opt", Count: 4}, {Name: "Island", Count: 10}, {Name: "Swamp", Count: 10},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Gravedigger", Count: 4}, {Name: "Opt", Count: 4},
		{Name: "Island", Count: 12}, {Name: "Swamp", Count: 12},
	},
}}

type kcantselfStats struct {
	games, divs, stalls int
	withheld            int // priority decisions with a Jace / Proft in hand, its mana untapped, at sorcery timing, and no cast offered
	cast                int // casts of Jace / Proft
}

// kcantselfPolicy casts Jace / Proft whenever offered, else random; it counts
// from the gorge side the decisions that withheld them.
func kcantselfPolicy(seed uint64, st *kcantselfStats) Policy {
	rnd := RandomPolicy(seed)
	isTarget := func(n string) bool { return n == "Jace Reawakened" || n == "Proft, Sinister Mastermind" }
	return PolicyFunc(func(c *PickCtx) Canon {
		g := c.G
		if g == nil || g.fr == nil || g.fr.d == nil || c.D.Kind != DPriority {
			return rnd.Pick(c)
		}
		gm := g.E.G
		for _, a := range c.D.Options {
			if a.Kind() != ActCast || a.Mode() != 0 {
				continue
			}
			if o, ok := g.GorgeOption(a); ok {
				if ob := gm.Obj(o.Obj); ob != nil && ob.Face() != nil && isTarget(ob.Face().Name) {
					st.cast++
					return a
				}
			}
		}
		p := state.PlayerID(c.D.Player)
		if gm.Active == p && gm.Step.IsMain() && len(gm.Stack) == 0 {
			islands, lands := 0, 0
			for _, id := range gm.Zone(state.ZBattlefield, p) {
				if o := gm.Obj(id); o != nil && !o.Tapped && objectHasType(o, "Land") {
					lands++
					if o.Face().Name == "Island" {
						islands++
					}
				}
			}
			for _, id := range gm.Zone(state.ZHand, p) {
				switch gm.Obj(id).Face().Name {
				case "Jace Reawakened":
					if islands >= 2 {
						st.withheld++
					}
				case "Proft, Sinister Mastermind":
					if lands >= 3 && islands < lands {
						st.withheld++
					}
				}
			}
		}
		return rnd.Pick(c)
	})
}

// runCantself plays the matchup over seeds 1..seeds; off sets the
// DebugCantSelfOff mutation.
func runCantself(t *testing.T, seeds int, off bool) kcantselfStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cantself")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugCantSelfOff
	dirac.DebugCantSelfOff = off
	defer func() { dirac.DebugCantSelfOff = old }()
	var st kcantselfStats
	for mi, md := range kcantselfDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kcantself-a", "kcantself-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, g, k, kcantselfPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			st.games++
			if d := res.Div; d != nil {
				st.divs++
				if !off {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				st.stalls++
				if !off {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				}
			}
		}
	}
	return st
}

func TestCantselfLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runCantself(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d withheld, %d casts", st.games, st.divs, st.stalls, st.withheld, st.cast)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.withheld < 30 {
		t.Errorf("only %d withheld decisions, want >= 30", st.withheld)
	}
	if st.cast < 30 {
		t.Errorf("only %d casts, want >= 30", st.cast)
	}
}

// TestCantselfMutation ignores the self statics: the kernel then offers
// Jace / Proft where gorge withholds them, so the games must diverge.
func TestCantselfMutation(t *testing.T) {
	st := runCantself(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("mutation left the statics off in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
