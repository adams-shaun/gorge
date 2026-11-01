package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated
// trigcondand pool, whose three carriers each hold an end-step trigger with
// TWO intervening-if clauses (an IsPresent$ beside a CheckSVar$), which
// gorge's triggerConditionHoldsWithSVars ANDs: every clause is its own gate,
// checked as the trigger fires and again at resolution. Creakwood Safewright
// (an Elf card in your graveyard AND a -1/-1 counter on it), Emergent
// Haunting (no spell cast from your hand this turn AND it isn't a creature)
// and Inventive Wingsmith (no spell cast from your hand this turn AND no
// flying counter on it). The decks carry Elves and burn (Elf cards reach a
// graveyard), cheap spells cast from hand (which flip the EQ0 clause) and
// creatures and basics. kgen lowers each trigger's Cond to a CondAnd of the
// two clauses; the test tallies, from gorge's state at the end step's
// start, the end steps where exactly one clause held (the trigger must not
// fire) and where both held.

var ktrigcondandDecks = [2][]cardset.Entry{
	{
		{Name: "Creakwood Safewright", Count: 4}, {Name: "Inventive Wingsmith", Count: 4},
		{Name: "Emergent Haunting", Count: 3}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Elvish Mystic", Count: 3}, {Name: "Opt", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Swamp", Count: 5}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 4}, {Name: "Island", Count: 3},
	},
	{
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Elvish Mystic", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
}

var ktrigcondandCarriers = [...]string{"Creakwood Safewright", "Emergent Haunting", "Inventive Wingsmith"}

// ktrigcondandTally counts, per carrier, the end steps by which clauses held.
type ktrigcondandTally struct {
	one, both [3]int
	snap      [3][2]bool // per carrier: its two clauses at the last decision
	have      [3]bool    // the carrier is on the active player's battlefield
	prevEnd   bool
}

func (y *ktrigcondandTally) reset() { y.snap, y.have, y.prevEnd = [3][2]bool{}, [3]bool{}, false }

// check runs at every decision both sides agree on. The clause values at a
// decision are those of the next end step's start while the game is before
// it, so the first decision seen inside an end step tallies the snapshot of
// the decision before.
func (y *ktrigcondandTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	inEnd := g.Step == state.StepEnd
	if inEnd && !y.prevEnd {
		for i := range y.have {
			if !y.have[i] {
				continue
			}
			switch {
			case y.snap[i][0] && y.snap[i][1]:
				y.both[i]++
			case y.snap[i][0] != y.snap[i][1]:
				y.one[i]++
			}
		}
	}
	y.prevEnd = inEnd
	if inEnd {
		return nil
	}
	y.have = [3]bool{}
	cast0 := g.Players[g.Active].SpellsCastThisTurn == 0 // every pool spell is cast from hand
	for _, id := range g.Zone(state.ZBattlefield, g.Active) {
		o := g.Obj(id)
		if o == nil || o.Face() == nil || o.Controller != g.Active {
			continue
		}
		switch o.Face().Name {
		case ktrigcondandCarriers[0]:
			elf := false
			for _, gid := range g.Zone(state.ZGraveyard, o.Owner) {
				if go1 := g.Obj(gid); go1 != nil && go1.Face() != nil {
					for _, ty := range go1.Face().Types {
						if ty == "Elf" {
							elf = true
						}
					}
				}
			}
			y.have[0], y.snap[0] = true, [2]bool{elf, o.Counter("M1M1") >= 1}
		case ktrigcondandCarriers[1]:
			y.have[1], y.snap[1] = true, [2]bool{cast0, !ref.E.IsCreature(id)}
		case ktrigcondandCarriers[2]:
			y.have[2], y.snap[2] = true, [2]bool{cast0, o.Counter("Flying") == 0}
		}
	}
	return nil
}

func ktrigcondandSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("trigcondand")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// ktrigcondandMutate rewrites every carrier's built Trigger.Cond from the
// conjunction to its first child alone (the second clause dropped).
func ktrigcondandMutate(tbl *kop.Table) int {
	n := 0
	for i := range tbl.Cards {
		for j := range tbl.Cards[i].Abilities {
			ab := &tbl.Cards[i].Abilities[j]
			if ab.Kind != kop.AbTriggered || ab.Trig.Cond == 0 {
				continue
			}
			if c := tbl.Conds[ab.Trig.Cond]; c.Kind == kop.CondAnd {
				ab.Trig.Cond = c.Sub[0]
				n++
			}
		}
	}
	return n
}

// ktrigcondandRun plays seeds 1..seeds; mutate rewrites the kernel table's
// carrier conditions first. It returns the first divergence's seed (0 none).
func ktrigcondandRun(t *testing.T, reg *cards.Registry, set cardset.Set, seeds uint64, mutate bool, y *ktrigcondandTally) (games int, divSeed uint64, stalls int) {
	t.Helper()
	var decks [2][]*cards.Card
	for s, d := range ktrigcondandDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(ktrigcondandDecks[0], ktrigcondandDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate {
		if n := ktrigcondandMutate(tbl); n != 3 {
			t.Fatalf("mutated %d carrier conditions, want 3", n)
		}
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ktrigcondand-a", "ktrigcondand-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		y.reset()
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: y.check})
		CloseSide(k)
		games++
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			if !mutate {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nlast actions %v\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
			}
			return games, seed, stalls
		}
	}
	return games, 0, stalls
}

func TestTrigcondandLockstep(t *testing.T) {
	t.Parallel()
	reg, set := ktrigcondandSetup(t)
	var y ktrigcondandTally
	games, div, stalls := ktrigcondandRun(t, reg, set, 256, false, &y)
	t.Logf("%d games, diverged seed %d, stalls %d", games, div, stalls)
	enough := 0
	for i, name := range ktrigcondandCarriers {
		t.Logf("%s: %d end steps with exactly one clause held, %d with both", name, y.one[i], y.both[i])
		if y.one[i] >= 10 && y.both[i] >= 10 {
			enough++
		}
	}
	if div != 0 || stalls != 0 {
		return
	}
	if enough < 2 {
		t.Errorf("only %d carriers reached 10 one-clause and 10 both-clause end steps, want at least 2", enough)
	}
}

// TestTrigcondandMutation: dropping each carrier's second clause (the
// kernel then fires on the first alone) must make the lockstep diverge.
func TestTrigcondandMutation(t *testing.T) {
	reg, set := ktrigcondandSetup(t)
	var y ktrigcondandTally
	_, div, _ := ktrigcondandRun(t, reg, set, 64, true, &y)
	if div == 0 {
		t.Fatalf("first-clause-only: no divergence in 64 seeds")
	}
	t.Logf("diverged at seed %d", div)
}
