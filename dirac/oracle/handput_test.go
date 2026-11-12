package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The hand put-self lockstep (CR 602, 400.7): Talon Gates of Madara's "{4}:
// Put this card from your hand onto the battlefield" is an activated ability
// that works from the hand and leaves its source there, so it resolves from the
// stack and moves the card only while it is still in the hand. The kernel
// lowers it as OpNinjutsu with C 1 (dirac/kw_ninjutsu.go handEntryResolve),
// whose entry then fires the card's own enters trigger (up to one target
// creature phases out). Both seats play the same deck over the handput pool,
// the policy activating whatever it is offered, so the ability is activated by
// both engines and the options match decision by decision.

var handputDeck = []cardset.Entry{
	{Name: "Talon Gates of Madara", Count: 6}, {Name: "Grizzly Bears", Count: 6},
	{Name: "Llanowar Elves", Count: 4}, {Name: "Lightning Bolt", Count: 2},
	{Name: "Island", Count: 6}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
}

// handputScan counts, in gorge's event log, the activations of Talon's hand
// ability (an AbilityPush whose face ability has ActivationZone$ Hand) and the
// moves from the hand to the battlefield that no land play made.
func handputScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	pushed := map[state.ObjID]bool{}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.AbilityPush:
			ob := g.E.G.Obj(ev.Obj)
			if ob == nil || ob.Face() == nil || ob.Face().Name != "Talon Gates of Madara" {
				continue
			}
			if i := int(ev.Amount); i >= 0 && i < len(ob.Face().Abilities) && ob.Face().Abilities[i].Params["ActivationZone"] == "Hand" {
				n["activated"]++
				pushed[ev.Obj] = true
			}
		case events.MoveZone:
			if ev.From == state.ZHand && ev.To == state.ZBattlefield && pushed[ev.Obj] {
				n["entered"]++
				delete(pushed, ev.Obj)
			}
		}
	}
}

// runHandput plays the seeded games; mutate makes the ability's op the
// ninjutsu entry (tapped and attacking), which diverges from gorge's plain
// entry.
func runHandput(t *testing.T, seeds uint64, mutate bool) (games, divs int, branch map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("handput")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	var ents [2][]cardset.Entry
	for s := range decks {
		ents[s] = handputDeck
		for _, e := range handputDeck {
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
	tbl, _, _, err := set.Matchup(ents[0], ents[1])
	if err != nil {
		t.Fatal(err)
	}
	plain := 0
	for ci := range tbl.Cards {
		for ai := range tbl.Cards[ci].Abilities {
			for pi := range tbl.Cards[ci].Abilities[ai].Prog {
				in := &tbl.Cards[ci].Abilities[ai].Prog[pi]
				if in.Op == kop.OpNinjutsu && in.C == 1 {
					plain++
					if mutate {
						in.C = 0 // enters tapped, and attacks when it can
					}
				}
			}
		}
	}
	if plain == 0 {
		t.Fatal("the pool lowered no plain put-self-from-hand ability")
	}
	played, branch := map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"hp-a", "hp-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		handputScan(gs, branch)
		if d := res.Div; d != nil {
			divs++
			if mutate {
				return
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestHandputLockstep(t *testing.T) {
	t.Parallel()

	games, divs, branch := runHandput(t, 200, false)
	t.Logf("%d games, %d divergences; hand activations %d, entered %d", games, divs, branch["activated"], branch["entered"])
	for _, want := range []string{"activated", "entered"} {
		if branch[want] < 4 {
			t.Errorf("%s %d, want at least 4", want, branch[want])
		}
	}
}

// TestHandputMutation proves the pool enters the card untapped and not
// attacking: the ninjutsu entry diverges.
func TestHandputMutation(t *testing.T) {
	t.Parallel()

	games, divs, _ := runHandput(t, 200, true)
	if divs == 0 {
		t.Fatalf("mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}
