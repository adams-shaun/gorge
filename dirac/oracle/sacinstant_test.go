package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated sacinstant pool, whose
// decks make Treasures (Prosperous Innkeeper), watch sacrifices (Giant Fly,
// Zhao) and hold instants cheap enough that a Treasure pays for one. gorge
// lowers a payment by activating each source at priority: a play that is
// offered at once (an instant) is chosen with the Treasure's sacrifice trigger
// still on the stack, below the spell; a sorcery-speed play waits for the stack
// to empty. The kernel must put the instant above the trigger the same way,
// on either player's turn and in response to a spell.

var ksacinstDeck = []cardset.Entry{
	{Name: "Prosperous Innkeeper", Count: 4}, {Name: "Giant Fly", Count: 4},
	{Name: "Zhao, Ruthless Admiral", Count: 2}, {Name: "Burst Lightning", Count: 4},
	{Name: "Giant Growth", Count: 3}, {Name: "Go for the Throat", Count: 3},
	{Name: "Lightning Bolt", Count: 3},
	{Name: "Forest", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 5},
}

// ksacinstTally counts the decision windows where an instant spell sits above a
// triggered ability on the stack, by whose turn it is and whether another spell
// sits below the trigger.
type ksacinstTally struct {
	above, ownTurn, oppTurn, overSpell int
	last                               [3]int
	turn                               int
}

func ksacinstInstant(o *state.Object) bool {
	if o.Card == nil || len(o.Card.Faces) == 0 {
		return false
	}
	for _, ty := range o.Card.Faces[0].Types {
		if ty == "Instant" {
			return true
		}
	}
	return false
}

func (y *ksacinstTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	n := len(g.Stack)
	if n < 2 {
		return nil
	}
	top := g.Obj(g.Stack[n-1])
	if top == nil || state.StackKindOf(g, top) != state.StackKindSpell || !ksacinstInstant(top) {
		return nil
	}
	key := [3]int{int(top.ID), n, int(g.Step)}
	if key == y.last {
		return nil
	}
	y.last = key
	trig, spell := false, false
	for _, id := range g.Stack[:n-1] {
		if o := g.Obj(id); o != nil {
			switch state.StackKindOf(g, o) {
			case state.StackKindTriggered:
				trig = true
			case state.StackKindSpell:
				spell = true
			}
		}
	}
	if !trig {
		return nil
	}
	y.above++
	if top.Controller == g.Active {
		y.ownTurn++
	} else {
		y.oppTurn++
	}
	if spell {
		y.overSpell++
	}
	return nil
}

func ksacinstRun(t *testing.T, seeds uint64, tot *ksacinstTally) (games, divSeed, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("sacinstant")
	if err != nil {
		t.Fatal(err)
	}
	md := [2][]cardset.Entry{ksacinstDeck, ksacinstDeck}
	decks := kcastDeck(t, reg, md)
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ksacinst-a", "ksacinst-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var y ksacinstTally
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: y.check})
		CloseSide(k)
		games++
		tot.above += y.above
		tot.ownTurn += y.ownTurn
		tot.oppTurn += y.oppTurn
		tot.overSpell += y.overSpell
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nlast actions %v\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
			return games, int(seed), stalls
		}
	}
	return games, 0, stalls
}

func TestSacInstantLockstep(t *testing.T) {
	t.Parallel()
	var tot ksacinstTally
	games, div, stalls := ksacinstRun(t, 200, &tot)
	t.Logf("%d games, diverged seed %d, stalls %d; windows with an instant above a trigger: %d (own turn %d, opponent's turn %d, over another spell %d)",
		games, div, stalls, tot.above, tot.ownTurn, tot.oppTurn, tot.overSpell)
	if div != 0 || stalls != 0 {
		return
	}
	if tot.oppTurn < 3 || tot.overSpell < 3 {
		t.Errorf("windows: opponent's turn %d, over a spell %d, want at least 3 of each", tot.oppTurn, tot.overSpell)
	}
}
