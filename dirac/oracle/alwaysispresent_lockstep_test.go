package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The board-condition state-trigger lockstep: Mode$ Always over IsPresent$
// with PresentCompare$ (CR 603.8; dirac/kgen/lower_trigger.go lowers it to the
// Present(filter, op, n) condition a static Condition$ uses). gorge checks the
// condition at every event; the kernel checks it at the counter-put event of
// the source, as the state-based actions finish a clean pass (sbaClean) and
// when an instance leaves the stack, so each carrier must sacrifice itself at
// the same decision in both engines:
//   - Barbarian Outcast and Bog Serpent: no Swamp you control (Stone Rain
//     takes the Swamps),
//   - Endrek Sahr: seven or more Thrulls (each Gray Ogre cast makes three),
//   - Afiya Grove: no +1/+1 counters on it (its upkeep trigger moves them).
var alwaysIsPresentScenarios = []struct {
	name  string
	decks [2][]cardset.Entry
	// carrier is the card whose sacrifice the scenario must reach.
	carrier string
}{
	{"swamps", [2][]cardset.Entry{
		{{Name: "Barbarian Outcast", Count: 6}, {Name: "Bog Serpent", Count: 4}, {Name: "Swamp", Count: 14}, {Name: "Mountain", Count: 10}, {Name: "Gray Ogre", Count: 6}},
		{{Name: "Stone Rain", Count: 10}, {Name: "Gray Ogre", Count: 8}, {Name: "Mountain", Count: 22}},
	}, "Barbarian Outcast"},
	{"thrulls", [2][]cardset.Entry{
		{{Name: "Endrek Sahr, Master Breeder", Count: 6}, {Name: "Gray Ogre", Count: 14}, {Name: "Swamp", Count: 12}, {Name: "Mountain", Count: 8}},
		{{Name: "Gray Ogre", Count: 16}, {Name: "Mountain", Count: 24}},
	}, "Endrek Sahr, Master Breeder"},
	{"counters", [2][]cardset.Entry{
		{{Name: "Afiya Grove", Count: 6}, {Name: "Gray Ogre", Count: 14}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10}},
		{{Name: "Gray Ogre", Count: 16}, {Name: "Mountain", Count: 24}},
	}, "Afiya Grove"},
}

// runAlwaysIsPresent plays seeds games of one scenario over the pool and
// returns the divergences and, per card name, the copies that ended a game in
// a graveyard (the carriers' trigger sacrifices them; the pool has no other
// way to bury a Grove, an Outcast or an Endrek but Stone Rain's land hits).
func runAlwaysIsPresent(t *testing.T, kdecks [2][]cardset.Entry, seeds int) (divs int, buried map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("alwayspresent")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kdecks {
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
	tbl, _, _, err := set.Matchup(kdecks[0], kdecks[1])
	if err != nil {
		t.Fatal(err)
	}
	buried = map[string]int{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kalways-a", "kalways-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		for p := state.PlayerID(0); p < 2; p++ {
			for _, id := range g.E.G.Zone(state.ZGraveyard, p) {
				if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
					buried[o.Face().Name]++
				}
			}
		}
		if d := res.Div; d != nil {
			divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return divs, buried
}

func TestAlwaysIsPresentLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, sc := range alwaysIsPresentScenarios {
		divs, buried := runAlwaysIsPresent(t, sc.decks, 128)
		t.Logf("%s: %d divergences, %s buried %d", sc.name, divs, sc.carrier, buried[sc.carrier])
		if buried[sc.carrier] < 5 {
			t.Errorf("%s: only %d %s ended in a graveyard (want at least 5): the state trigger is not reached", sc.name, buried[sc.carrier], sc.carrier)
		}
	}
}
