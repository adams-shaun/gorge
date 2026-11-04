package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The LandPlayed / SearchedLibrary / Forage / Discover trigger lockstep
// Shadow of the Goblin and The Endstone (land plays, the
// former's origin-scoped lines), Wan Shi Tong (an opponent's library
// search), Corpseberry Cultivator (a forage) and Curator of Sun's Creation
// (a discover) beside fetch lands, ramp spells, forage and discover
// enablers. Gorge's log is tallied per carrier: a TriggerPush on the carrier
// after the matching marker.

var ktrigm2Decks = [2][]cardset.Entry{
	{
		{Name: "Wan Shi Tong, Librarian", Count: 3}, {Name: "Curator of Sun's Creation", Count: 4},
		{Name: "Geological Appraiser", Count: 4}, {Name: "Daring Discovery", Count: 2},
		{Name: "Shadow of the Goblin", Count: 3}, {Name: "The Endstone", Count: 2},
		{Name: "Felidar Retreat", Count: 3}, {Name: "Evolving Wilds", Count: 2},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Island", Count: 4},
		{Name: "Mountain", Count: 5}, {Name: "Plains", Count: 4},
	},
	{
		{Name: "Corpseberry Cultivator", Count: 4}, {Name: "Treetop Sentries", Count: 4},
		{Name: "Feed the Cycle", Count: 3}, {Name: "Bake into a Pie", Count: 4},
		{Name: "Rampant Growth", Count: 3}, {Name: "Cultivate", Count: 3},
		{Name: "Fabled Passage", Count: 2}, {Name: "Evolving Wilds", Count: 2},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Forest", Count: 5}, {Name: "Swamp", Count: 3},
	},
}

// ktrigm2Tally counts carrier pushes after each marker.
type ktrigm2Tally struct{ land, search, forage, discover int }

// ktrigm2Policy plays randomly and scans the gorge log as it goes.
func ktrigm2Policy(seed uint64, tl *ktrigm2Tally) Policy {
	rnd := RandomPolicy(seed)
	at := 0
	var last events.Kind
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			evs := c.G.E.L.Events
			if len(evs) < at {
				at = 0
			}
			for j := at; j < len(evs); j++ {
				e := &evs[j]
				switch e.Kind {
				case events.LandPlayed, events.SearchedLibrary, events.ForageAction, events.Discover:
					last = e.Kind
				case events.StepChange, events.TurnChange, events.PutOnStack:
					last = events.GameStart
				case events.TriggerPush:
					o := g.Obj(e.Obj)
					if o == nil || o.Face() == nil {
						continue
					}
					switch name := o.Face().Name; {
					case last == events.LandPlayed && (name == "Shadow of the Goblin" || name == "The Endstone"):
						tl.land++
					case last == events.SearchedLibrary && name == "Wan Shi Tong, Librarian":
						tl.search++
					case last == events.ForageAction && name == "Corpseberry Cultivator":
						tl.forage++
					case last == events.Discover && name == "Curator of Sun's Creation":
						tl.discover++
					}
				}
			}
			at = len(evs)
		}
		return rnd.Pick(c)
	})
}

func runTrigmodes(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, tl ktrigm2Tally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("trigmodes")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugTrigModes2Off
	dirac.DebugTrigModes2Off = mutate
	defer func() { dirac.DebugTrigModes2Off = old }()
	var decks [2][]*cards.Card
	for s, d := range ktrigm2Decks {
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
	tbl, _, _, err := set.Matchup(ktrigm2Decks[0], ktrigm2Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ktrigm2-a", "ktrigm2-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, ktrigm2Policy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
	}
	return games, divs, stalls, tl
}

func TestLandSearchForageDiscoverTriggersLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runTrigmodes(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, carrier triggers after a land play %d, a search %d, a forage %d, a discover %d",
		games, divs, stalls, tl.land, tl.search, tl.forage, tl.discover)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		what string
		n    int
	}{{"land play", tl.land}, {"search", tl.search}, {"forage", tl.forage}, {"discover", tl.discover}} {
		if c.n < 5 {
			t.Errorf("only %d carrier triggers after a %s (need 5)", c.n, c.what)
		}
	}
}

// TestLandSearchForageDiscoverTriggersMutation fires no LandPlayed / SearchedLibrary / Forage /
// Discover line: gorge's still fire, so the games must diverge.
func TestLandSearchForageDiscoverTriggersMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runTrigmodes(t, 64, true)
	if divs == 0 {
		t.Fatalf("no-fire mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
