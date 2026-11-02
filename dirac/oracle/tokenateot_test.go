package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The created-token AtEOT$ rider lockstep: Kavaron Harrier's
// Robot (AtEOT$ SacrificeCombat: an untracked sacrifice at this turn's end of
// combat), Harried Dronesmith's Thopter (YourSacrifice: gorge's plain
// Sacrifice at the end step) and Old Hob's Mutant (AtEOT$ Destroy: a tracked
// destroy at the end step, which indestructible stops; Old Hob's own pump
// grants it). The blockers kill Robots before end of combat (the trigger then
// goes on the stack over a token that left) and Shock kills Mutants before the
// end step (the tracked Destroy promise is dropped).

var ktokenateotDecks = [][2][]cardset.Entry{{
	// A: the carriers
	{
		{Name: "Kavaron Harrier", Count: 4}, {Name: "Harried Dronesmith", Count: 3}, {Name: "Old Hob, Alleycat Blues", Count: 4},
		{Name: "Goblin Piker", Count: 2}, {Name: "Raging Goblin", Count: 2}, {Name: "Hill Giant", Count: 2},
		{Name: "Savannah Lions", Count: 2}, {Name: "Suntail Hawk", Count: 2}, {Name: "Serra Angel", Count: 1}, {Name: "Shock", Count: 3},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 7},
	},
	// B: blockers and burn, with a few carriers
	{
		{Name: "Kavaron Harrier", Count: 2}, {Name: "Old Hob, Alleycat Blues", Count: 1},
		{Name: "Goblin Piker", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Suntail Hawk", Count: 4}, {Name: "Serra Angel", Count: 2}, {Name: "Raging Goblin", Count: 3}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 4},
	},
}, {
	// the mirror of A
	{
		{Name: "Kavaron Harrier", Count: 4}, {Name: "Harried Dronesmith", Count: 3}, {Name: "Old Hob, Alleycat Blues", Count: 4},
		{Name: "Goblin Piker", Count: 2}, {Name: "Raging Goblin", Count: 2}, {Name: "Hill Giant", Count: 2},
		{Name: "Savannah Lions", Count: 2}, {Name: "Suntail Hawk", Count: 2}, {Name: "Serra Angel", Count: 1}, {Name: "Shock", Count: 3},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 7},
	},
	{
		{Name: "Kavaron Harrier", Count: 4}, {Name: "Harried Dronesmith", Count: 3}, {Name: "Old Hob, Alleycat Blues", Count: 4},
		{Name: "Goblin Piker", Count: 2}, {Name: "Raging Goblin", Count: 2}, {Name: "Hill Giant", Count: 2},
		{Name: "Savannah Lions", Count: 2}, {Name: "Suntail Hawk", Count: 2}, {Name: "Serra Angel", Count: 1}, {Name: "Shock", Count: 3},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 7},
	},
}}

// ktokenateotTok is one tracked rider token (gorge's object id).
type ktokenateotTok struct {
	kind     string // Robot, Thopter or Mutant
	mint     int32  // the turn it was first seen
	survived bool
}

// ktokenateotTally counts, from gorge's state at each decision, how each
// rider token left the battlefield.
type ktokenateotTally struct {
	robotEOC, robotEarly     int // Robots gone at the end of combat step / before it
	thopterEnd, thopterEarly int // Thopters gone at the end step / before it
	mutantEnd, mutantEarly   int // Mutants destroyed at the end step / gone before it (the Destroy promise dropped)
	mutantSurvived           int // Mutants still there after their turn's end step
	srcDivs                  int // divergences on a projected Stack[i].Source
	toks                     map[state.ObjID]*ktokenateotTok
}

func ktokenateotKind(o *state.Object) string {
	if o == nil || !o.IsToken || o.Face() == nil {
		return ""
	}
	switch n := strings.TrimSuffix(o.Face().Name, " Token"); n {
	case "Robot", "Thopter", "Mutant":
		return n
	}
	return ""
}

// observe reads gorge's battlefield at one decision.
func (tl *ktokenateotTally) observe(g *state.Game) {
	if tl.toks == nil {
		tl.toks = map[state.ObjID]*ktokenateotTok{}
	}
	seen := map[state.ObjID]bool{}
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			k := ktokenateotKind(g.Obj(id))
			if k == "" {
				continue
			}
			seen[id] = true
			t := tl.toks[id]
			if t == nil {
				t = &ktokenateotTok{kind: k, mint: g.Turn}
				tl.toks[id] = t
			}
			if g.Turn > t.mint && !t.survived {
				t.survived = true
				if k == "Mutant" {
					tl.mutantSurvived++
				}
			}
		}
	}
	for id, t := range tl.toks {
		if seen[id] {
			continue
		}
		delete(tl.toks, id)
		if g.Turn != t.mint {
			continue // died on a later turn: not a rider's doing
		}
		switch t.kind {
		case "Robot":
			if g.Step == state.StepEndCombat {
				tl.robotEOC++
			} else if g.Step < state.StepEndCombat {
				tl.robotEarly++
			}
		case "Thopter":
			if g.Step == state.StepEnd {
				tl.thopterEnd++
			} else if g.Step < state.StepEnd {
				tl.thopterEarly++
			}
		case "Mutant":
			if g.Step == state.StepEnd {
				tl.mutantEnd++
			} else if g.Step < state.StepEnd {
				tl.mutantEarly++
			}
		}
	}
}

// ktokenateotPolicy attacks and plays whenever it can (so the games reach the
// carriers, their attacks and Old Hob's pump), taking every "may pay {2}" yes,
// else random.
func ktokenateotPolicy(seed uint64, tl *ktokenateotTally) Policy {
	rnd := rand.New(rand.NewPCG(seed, 0x746f6b656e))
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.observe(c.G.E.G)
		}
		buf = buf[:0]
		for _, kinds := range [...][]CanonKind{{ActYes}, {ActAttack}, {ActBlock, ActCast, ActPlayLand, ActActivate, ActTarget}} {
			for _, a := range c.D.Options {
				for _, k := range kinds {
					if a.Kind() == k {
						buf = append(buf, a)
					}
				}
			}
			if len(buf) != 0 && rnd.IntN(8) != 0 {
				return buf[rnd.IntN(len(buf))]
			}
			buf = buf[:0]
		}
		return c.D.Options[rnd.IntN(len(c.D.Options))]
	})
}

// runTokenateot plays the matchups for seeds 1..seeds and returns the games,
// the divergences and the tally. mutate makes a delayed destroy a plain
// move to the graveyard (the mutation must diverge).
func runTokenateot(t *testing.T, seeds int, mutate bool) (games, divs int, tl ktokenateotTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tokenateot")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugTokAtEOTDestroyIsLeave
	dirac.DebugTokAtEOTDestroyIsLeave = mutate
	defer func() { dirac.DebugTokAtEOTDestroyIsLeave = old }()
	quiet := mutate || dirac.DebugAteotSourceAffected // a mutation run expects divergences
	for mi, md := range ktokenateotDecks {
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
		// the stop rule: no other end-of-combat delayed program in the pool
		// (fireDlyEndCombat enqueues after the s.g.dp programs)
		for _, c := range tbl.Cards {
			for _, a := range c.Abilities {
				for _, in := range a.Prog {
					if in.Op == kop.OpDelayedProgram && in.A.N != 0 {
						t.Fatalf("%s: a timed delayed program (A %d) in the pool", c.Name, in.A.N)
					}
				}
			}
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"ktokenateot-a", "ktokenateot-b"}, decks, 1)
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
			tl.toks = nil
			res := LockstepLed(m, seed, g, k, ktokenateotPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			games++
			if d := res.Div; d != nil {
				divs++
				if strings.HasPrefix(d.Field, "Stack[") && strings.HasSuffix(d.Field, "].Source") {
					tl.srcDivs++
				}
				if !quiet {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
			CloseSide(k)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) && !quiet {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
		}
	}
	return games, divs, tl
}

func TestTokenateotLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, tl := runTokenateot(t, 256, false)
	t.Logf("%d games, %d divergences; Robots sacrificed at end of combat %d, gone before it %d; Thopters sacrificed at the end step %d (gone before %d); Mutants destroyed at the end step %d, survived %d, gone before it (Destroy dropped) %d",
		games, divs, tl.robotEOC, tl.robotEarly, tl.thopterEnd, tl.thopterEarly, tl.mutantEnd, tl.mutantSurvived, tl.mutantEarly)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if tl.robotEOC < 20 || tl.robotEarly < 3 || tl.thopterEnd < 20 || tl.mutantEnd < 10 || tl.mutantSurvived < 3 || tl.mutantEarly < 3 {
		t.Errorf("thin coverage: Robot EOC %d (want 20), Robot gone early %d (want 3), Thopter end %d (want 20), Mutant destroyed %d (want 10), Mutant survived %d (want 3), Mutant dropped %d (want 3)",
			tl.robotEOC, tl.robotEarly, tl.thopterEnd, tl.mutantEnd, tl.mutantSurvived, tl.mutantEarly)
	}
}

// TestTokenateotMutation makes a delayed destroy a plain move to the
// graveyard: the indestructible Mutant dies in the kernel only.
func TestTokenateotMutation(t *testing.T) {
	games, divs, _ := runTokenateot(t, 128, true)
	if divs == 0 {
		t.Fatalf("mutation (delayed destroy ignores indestructible) ran %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
