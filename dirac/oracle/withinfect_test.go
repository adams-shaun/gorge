package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The withInfect filter word (CR 702.90a: the printed keyword, read off the
// derived keywords) in the three corpus cards that carry it, played decision
// by decision against gorge over the withinfect pool: Corpse Cur's "return
// target creature card with infect from your graveyard" ETB, Hand of the
// Praetors' lord for other infect creatures you control and its "whenever you
// cast a creature spell with infect, target player gets a poison counter",
// and Burn the Impure's damage to that creature's controller if the target
// has infect. Both seats play the same deck, so every card is cast by both
// engines; infect and plain creatures share the battlefield so the word has
// both answers.

var wiDeck = []cardset.Entry{
	{Name: "Corpse Cur", Count: 4}, {Name: "Hand of the Praetors", Count: 4},
	{Name: "Burn the Impure", Count: 4}, {Name: "Glistener Elf", Count: 4},
	{Name: "Plague Stinger", Count: 4}, {Name: "Grizzly Bears", Count: 4},
	{Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5},
}

// wiScan counts each carrier's cast, the graveyard-to-hand returns, the
// poison counters placed on a player and the 3 life Burn the Impure's rider
// takes from a creature's controller (no other card here loses a player 3
// life: the infect creatures deal poison, the Bears 2).
func wiScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.PutOnStack:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ev.From == state.ZHand {
				switch nm := ob.Face().Name; nm {
				case "Corpse Cur", "Hand of the Praetors", "Burn the Impure":
					n[nm]++
				}
			}
		case events.MoveZone:
			if ev.From == state.ZGraveyard && ev.To == state.ZHand {
				n["returned"]++
			}
		case events.Damage:
			if ev.Amount == 3 && ev.Obj == 0 && ev.Counter == "" {
				n["burnrider"]++ // the pool's only 3-life loss is Burn the Impure's rider
			}
		case events.PlayerCounterChange:
			if ev.Amount > 0 {
				n["poison"]++
			}
		}
	}
}

func runWithInfect(t *testing.T, seeds uint64) (games int, branch map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("withinfect")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	var ents [2][]cardset.Entry
	for s := range decks {
		ents[s] = wiDeck
		for _, e := range wiDeck {
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
	played, branch := map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"wi-a", "wi-b"}, decks, 1)
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
		wiScan(gs, branch)
		if d := res.Div; d != nil {
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

func TestWithInfectLockstep(t *testing.T) {
	t.Parallel()

	games, branch := runWithInfect(t, 200)
	t.Logf("%d games; %v", games, branch)
	for _, want := range []string{"Corpse Cur", "Hand of the Praetors", "Burn the Impure", "returned", "poison", "burnrider"} {
		if branch[want] < 4 {
			t.Errorf("%s %d, want at least 4", want, branch[want])
		}
	}
}
