package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The planned-cast on-stack lockstep (gorge rules/castprobe.go
// paymentPlanHoldsOnStack; kernel cast_planholdsonstack.go): gorge's planned
// payment runs in the CR 601.2g window, after CR 601.2a has moved the card to
// the stack, so a plan whose source makes a different amount there is
// withheld and the cast is lowered instead: its sources tap first, with the
// card still in hand (CR 605.3a). Selvala, Eager Trailblazer adds one mana
// for each different power among its controller's creatures; Stingerback
// Terror (-1/-1 for each card in hand) and Syr Elenora (power equal to the
// cards in hand) change power when a card leaves the hand.

var kposDeck = []cardset.Entry{
	{Name: "Selvala, Eager Trailblazer", Count: 4}, {Name: "Stingerback Terror", Count: 4}, {Name: "Syr Elenora, the Discerning", Count: 3},
	{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 2},
	{Name: "Forest", Count: 9}, {Name: "Plains", Count: 6}, {Name: "Mountain", Count: 5},
}

// kposScan counts, in gorge's log, the hand casts made while Selvala's mana
// floated: Selvala (vigilant, so tapped only for mana) tapped before the
// spell's PutOnStack in the same step, which only a lowered cast does (a
// planned cast taps after its push).
func kposScan(g *GorgeSide, tally map[string]int) {
	if g == nil {
		return
	}
	selvala := false
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.Tap:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Selvala, Eager Trailblazer" {
				selvala = true
			}
		case events.PutOnStack:
			if ev.From == state.ZHand && selvala {
				tally["casts paid by a floated Selvala"]++
			}
			selvala = false
		case events.StepChange, events.TurnChange:
			selvala = false
		}
	}
}

// TestPlanOnStackLockstep plays the pool mirror, gorge and dirac decision by
// decision, on both surfaces.
func TestPlanOnStackLockstep(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	set, err := cardset.RawByName("planonstack")
	if err != nil {
		t.Fatal(err)
	}
	var deck []*cards.Card
	for _, e := range kposDeck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, c)
		}
	}
	if len(deck) != 40 {
		t.Fatalf("deck: %d cards", len(deck))
	}
	tally := map[string]int{}
	lowered := 0
	for seed := uint64(1); seed <= 60; seed++ {
		tbl, _, _, err := set.Matchup(kposDeck, kposDeck)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kpos-a", "kpos-b"}, [2][]*cards.Card{deck, deck}, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, map[string]int{})
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		kposScan(gs, tally)
		if gs != nil {
			lowered += gs.Stats.Lowered
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d lowered casts; %s", lowered, katTally(tally))
	if n := tally["casts paid by a floated Selvala"]; n < 3 {
		t.Errorf("casts paid by a floated Selvala %d, want at least 3", n)
	}
}
