package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Nyxborn Unicorn's bestowed "has mentor" (CR 702.134b, 113.2c): the
// Continuous AddKeyword$ Mentor static grants its holder a latent Attacks
// trigger, listed once per granting static beside any printed K:Mentor line.
// Goblin Banneret prints Mentor, so a bestowed Unicorn on it is the
// printed+granted shape; a vanilla creature (Grizzly Bears, Memnite) wearing
// two Unicorns is the two-grant shape. The pool is dirac/cardset/pools/
// fixtures/mentor.

var kmentorGrantDecks = [2][]cardset.Entry{
	{
		{Name: "Nyxborn Unicorn", Count: 4}, {Name: "Goblin Banneret", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Memnite", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Pacifism", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 8},
	},
}

// mentorGrantPrinters are the pool's printed-Mentor creatures: a granted
// trigger on one of them is the printed+granted shape.
var mentorGrantPrinters = map[string]bool{
	"Goblin Banneret": true,
}

func TestMentorGrantLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mentor")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kmentorGrantDecks {
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
	tbl, _, _, err := set.Matchup(kmentorGrantDecks[0], kmentorGrantDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	games, stalls, granted, printedAndGranted, twoGrant, bestowed := 0, 0, 0, 0, 0, 0
	for seed := uint64(1); seed <= 300; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"mentor-a", "mentor-b"}, decks, 1)
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
		rc := RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20}
		rc.Finish = func(g *GorgeSide) {
			if g == nil || g.E == nil || g.E.G == nil {
				return
			}
			for p := range g.E.G.Players {
				for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
					if ob := g.E.G.Obj(id); ob != nil && ob.BestowedAttached() {
						bestowed++
						return
					}
				}
			}
		}
		res := LockstepLed(m, seed, g, k, kinnerlightPolicy(seed), rc)
		CloseSide(k)
		games++
		perHolder := map[state.ObjID]int{}
		for _, e := range g.E.L.Events {
			if e.Kind == events.KeywordTriggerPush && e.Counter == "__kwMentorGranted" {
				granted++
				perHolder[e.Obj]++
				if ob := g.E.G.Obj(e.Obj); ob != nil && ob.Face() != nil && mentorGrantPrinters[ob.Face().Name] {
					printedAndGranted++
				}
			}
		}
		for _, n := range perHolder {
			if n >= 2 {
				twoGrant++
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, %d stalls; %d bestowed; granted Mentor triggers %d (printed+granted %d, two-grant holders %d)", games, stalls, bestowed, granted, printedAndGranted, twoGrant)
	if stalls != 0 {
		t.Fatalf("%d stalls", stalls)
	}
	// the precondition: the grant was reached
	if granted < 1 {
		t.Errorf("granted Mentor triggers under-reached: %d (want at least 1)", granted)
	}
}
